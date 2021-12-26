package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func init() {
	commands["unpack"] = Command{commandUnpack, nil, checkComponentArg, "geneos unpack FILE...",
		`Unpacks the supplied archive FILE(s) in to the packages/ directory. The filename(s) must of of the form:

	geneos-TYPE-VERSION*.tar.gz

The directory for the package is created using the VERSION from the archive filename.`}

	commands["download"] = Command{commandDownload, nil, checkComponentArg, "geneos download [TYPE] [latest|FILTER|URL...]",
		`Download and unpack the sources in the packages directory or latest version(s) from
the official download site. The filename must of of the format:

	geneos-TYPE-VERSION*.tar.gz

The TYPE, if supplied, limits the selection of downloaded archive(s). The directory
for the package is created using the VERSION from the archive filename.`}

	commands["update"] = Command{commandUpdate, nil, checkComponentArg, "geneos update [TYPE] VERSION",
		`Update the symlink for the default base name of the package used to VERSION. The base directory,
for historical reasons, is 'active_prod' and is usally linked to the latest version of a component type
in the packages directory. VERSION can either be a directory name or the literal 'latest'. If TYPE is not
supplied, all supported component types are updated to VERSION.

Update will stop all matching instances of the each type before updating the link and starting them up
again, but only if the instance uses the 'active_prod' basename.

The 'latest' version is based on directory names of the form:

	[GA]X.Y.Z

Where X, Y, Z are each ordered in ascending numerical order. If a directory starts 'GA' it will be selected
over a directory with the same numerical versions. All other directories name formats will result in unexpected
behaviour.

Future version may support selecting a base other than 'active_prod'.`}

	// need overwrite flags
}

//
// if there is no 'active_prod' link then attach it to the latest version
// installed
//
func commandUnpack(ct ComponentType, files []string, params []string) (err error) {
	if ct != None {
		logError.Fatalln("Must not specify a compoenent type, only archive files")
	}

	for _, archive := range files {
		filename := filepath.Base(archive)
		gz, err := os.Open(archive)
		if err != nil {
			return err
		}
		defer gz.Close()

		if err = unarchive(filename, gz); err != nil {
			log.Println(err)
			return err
		}
	}
	// create a symlink only if one doesn't exist
	return updateToVersion(ct, "latest", false)
}

func commandDownload(ct ComponentType, files []string, params []string) (err error) {
	version := "latest"
	if len(files) > 0 {
		version = files[0]
	}
	return downloadComponent(ct, version)
}

func downloadComponent(ct ComponentType, version string) (err error) {
	switch ct {
	case None:
		for _, t := range componentTypes() {
			if err = downloadComponent(t, version); err != nil {
				if errors.Is(err, fs.ErrExist) {
					log.Println(err)
					continue
				}
				return
			}
		}
		return nil
	default:
		filename, gz, err := downloadArchive(ct, version)
		if err != nil {
			return err
		}
		defer gz.Close()

		log.Println("fetched", ct.String(), filename)

		if err = unarchive(filename, gz); err != nil {
			return err
		}
		return updateToVersion(ct, version, false)
	}
}

var archiveRE = regexp.MustCompile(`^geneos-(\w+)-([\w\.-]+?)[\.-]?linux`)

func unarchive(filename string, gz io.Reader) (err error) {
	parts := archiveRE.FindStringSubmatch(filename)
	if len(parts) == 0 {
		logError.Fatalf("invalid archive name format: %q", filename)
	}
	comp := parseComponentName(parts[1])
	if comp == None || comp == Unknown {
		log.Println("component type required")
		return
	}
	version := parts[2]
	basedir := filepath.Join(RunningConfig.ITRSHome, "packages", comp.String(), version)
	if _, err = os.Stat(basedir); err == nil {
		return fmt.Errorf("%s: %w", basedir, fs.ErrExist)
	}
	if err = os.MkdirAll(basedir, 0775); err != nil {
		return
	}

	t, err := gzip.NewReader(gz)
	if err != nil {
		return
	}
	defer t.Close()

	tr := tar.NewReader(t)
	for {
		var hdr *tar.Header
		hdr, err = tr.Next()
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return
		}
		// log.Println("file:", hdr.Name, "size", hdr.Size)
		// strip leading component name
		// do not trust tar archives to contain safe paths
		var name string
		name = strings.TrimPrefix(hdr.Name, comp.String()+"/")
		if name == "" {
			continue
		}
		name, err = cleanRelativePath(name)
		if err != nil {
			logError.Fatalln(err)
		}
		fullpath := filepath.Join(basedir, name)
		switch hdr.Typeflag {
		case tar.TypeReg:
			out, err := os.OpenFile(fullpath, os.O_CREATE|os.O_WRONLY, hdr.FileInfo().Mode())
			if err != nil {
				return err
			}
			n, err := io.Copy(out, tr)
			if err != nil {
				out.Close()
				return err
			}
			if n != hdr.Size {
				log.Println("lengths different:", hdr.Size, n)
			}
			out.Close()
		case tar.TypeDir:
			if err = os.MkdirAll(fullpath, hdr.FileInfo().Mode()); err != nil {
				return
			}
		case tar.TypeSymlink, tar.TypeGNULongLink:
			if filepath.IsAbs(hdr.Linkname) {
				logError.Fatalln("archive contains absolute symlink target")
			}
			if _, err = os.Stat(fullpath); err != nil {
				if err = os.Symlink(hdr.Linkname, fullpath); err != nil {
					logError.Fatalln(err)
				}
			}
		default:
			log.Printf("unsupported file type %c\n", hdr.Typeflag)
		}
	}
	log.Printf("unpacked %q to %q\n", filename, basedir)
	return
}

func commandUpdate(ct ComponentType, args []string, params []string) (err error) {
	version := "latest"
	if len(args) > 0 {
		version = args[0]
	}
	return updateToVersion(ct, version, true)
}

// check selected version exists first
func updateToVersion(ct ComponentType, version string, overwrite bool) (err error) {
	base := "active_prod"
	basedir := filepath.Join(RunningConfig.ITRSHome, "packages", ct.String())
	basepath := filepath.Join(basedir, base)

	switch ct {
	case None:
		for _, t := range componentTypes() {
			if err = updateToVersion(t, version, overwrite); err != nil {
				log.Println(err)
			}
		}
	case Gateways, Netprobes, Licds:
		if version == "" || version == "latest" {
			version = latestDir(basedir)
		}
		// does the version directory exist?
		if _, err = os.Stat(filepath.Join(basedir, version)); err != nil {
			err = fmt.Errorf("update %s to version %s failed: %w", ct, version, err)
			return err
		}
		current, err := os.Readlink(basepath)
		if err != nil && errors.Is(err, &fs.PathError{}) {
			log.Println("cannot read link", basepath)
		}
		if current != "" && !overwrite {
			return nil
		}
		// empty current is fine
		if current == version {
			log.Println(ct, base, "is already linked to", version)
			return nil
		}
		insts := matchComponents(ct, "Base", base)
		// stop matching instances
		for _, i := range insts {
			stopInstance(i, nil)
			defer startInstance(i, nil)
		}
		if err = os.Remove(basepath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err = os.Symlink(version, basepath); err != nil {
			return err
		}
		log.Println(ct, base, "updated to", version)
	default:
		return ErrNotSupported
	}
	return nil
}

var versRE = regexp.MustCompile(`^\d+(\.\d+){0,2}`)

// given a directory find the "latest" version of the form
// [GA]M.N.P[-DATE] M, N, P are numbers, DATE is treated as a string
func latestDir(dir string) (latest string) {
	dirs, err := os.ReadDir(dir)
	if err != nil {
		logError.Fatalln(err)
	}
	max := make([]int, 3)
	for _, v := range dirs {
		if !v.IsDir() {
			continue
		}
		// strip 'GA' prefix and get name
		d := strings.TrimPrefix(v.Name(), "GA")
		if !versRE.MatchString(d) {
			logDebug.Println(d, "does not match a valid directory pattern")
			continue
		}
		s := strings.SplitN(d, ".", 3)
		next := slicetoi(s)

	OUTER:
		for i := range max {
			switch {
			case next[i] < max[i]:
				break OUTER
			case next[i] > max[i]:
				// do a final lexical scan for suffixes?
				latest = v.Name()
				max[i] = next[i]
			default:
				// if equal and we are on last number, lexical comparison
				// to pick up suffixes
				if len(max) == i+1 && v.Name() > latest {
					latest = v.Name()
				}
			}
		}
	}
	return
}

// given a component type and a key/value pair, return matching
// instances
func matchComponents(ct ComponentType, k, v string) (insts []Instance) {
	for _, i := range instances(ct) {
		if v == getString(i, Prefix(i)+k) {
			if err := loadConfig(&i, false); err != nil {
				log.Println(Type(i), Name(i), "cannot load config")
			}
			insts = append(insts, i)
		}
	}
	return
}

// fetch a (the latest) component from a URL, but the URLs
// are special and the resultant redirection contains the filename
// etc.
//
// URL is
// https://resources.itrsgroup.com/download/latest/[COMPONENT]?os=linux
// is RHEL8 is required, add ?title=el8
//
// there is a mapping of our compoent types to the URLs too.
//
// Gateways -> Gateway+2
// Netprobes -> Netprobe
// Licds -> Licence+Daemon
// Webservers -> Web+Dashboard
//
// auth requires a POST with a JSON body of
// { "username": "EMAIL", "password": "PASSWORD" }
// until anon access is allowed
//

const defaultURL = "https://resources.itrsgroup.com/download/latest/"

type DownloadAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var downloadMap = map[ComponentType]string{
	Gateways:  "Gateway+2",
	Netprobes: "Netprobe",
	Licds:     "Licence+Daemon",
}

// XXX use HEAD to check match and compare to on disk versions
func downloadArchive(ct ComponentType, version string) (filename string, body io.ReadCloser, err error) {
	baseurl := RunningConfig.DownloadURL
	if baseurl == "" {
		baseurl = defaultURL
	}

	var resp *http.Response

	downloadURL, _ := url.Parse(baseurl)
	realpath, _ := url.Parse(downloadMap[ct])
	v := url.Values{}
	v.Set("os", "linux")
	if version != "latest" {
		v.Set("title", version)
	}
	realpath.RawQuery = v.Encode()
	source := downloadURL.ResolveReference(realpath).String()
	logDebug.Println("source url:", source)

	// if a download user is set then issue a POST with username and password
	// in a JSON body, else just try the GET
	if RunningConfig.DownloadUser != "" {
		var authbody DownloadAuth
		authbody.Username = RunningConfig.DownloadUser
		authbody.Password = RunningConfig.DownloadPass

		var authjson []byte
		authjson, err = json.Marshal(authbody)
		if err != nil {
			logError.Fatalln(err)
		}

		resp, err = http.Post(source, "application/json", bytes.NewBuffer(authjson))
	} else {
		resp, err = http.Get(source)
	}
	if err != nil {
		logError.Fatalln(err)
	}
	if resp.StatusCode > 299 {
		err = fmt.Errorf("cannot download %s package version %s: %s", ct, version, resp.Status)
		resp.Body.Close()
		return
		// logError.Fatalln(resp.Status)
	}

	filename, err = filenameFromHTTPResp(resp, downloadURL)
	if err != nil {
		logError.Fatalln(err)
	}
	body = resp.Body
	return
}
