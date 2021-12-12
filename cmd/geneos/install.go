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
	"os"
	"path/filepath"
	"strings"
)

func init() {
	commands["install"] = Command{commandInstall, parseArgs, "geneos install [TYPE] [latest|FILE...]",
		`Install the supplied FILE(s) in the packages/ directory, or fetch the latest version from the official
download site. The filename must of of the format:

	geneos-TYPE-VERSION*.tar.gz

The TYPE, if supplied, only influences the downloaded archive and is ignored for local files. The directory
for the package is created using the VERSION from the archive filename.

Future support will include URLs and/or specific versions for downloads as well as options to override the
segments of the archive name used.
`}

	commands["update"] = Command{commandUpdate, parseArgs, "geneos update [TYPE] VERSION",
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
}

//
// if there is no 'active_prod' link then attach it to the latest version
// installed
//
// 'geneos install gateway [files]'
// 'geneos install netprobe latest'
func commandInstall(ct ComponentType, files []string) (err error) {
	if len(files) == 1 && files[0] == "latest" {
		return installLatest(ct)
	}

	for _, archive := range files {
		f := filepath.Base(archive)
		gz, err := os.Open(archive)
		if err != nil {
			return err
		}
		defer gz.Close()

		if err = unarchive(f, gz); err != nil {
			log.Println(err)
			return err
		}
	}
	return updateLatest(ct, true)
}

func installLatest(ct ComponentType) (err error) {
	switch ct {
	case None:
		for _, t := range ComponentTypes() {
			if err = installLatest(t); err != nil {
				if !errors.Is(err, fs.ErrExist) {
					return
				}
				log.Println(err)
			}
		}
		return nil
	default:
		f, gz, err := fetchLatest(ct)
		if err != nil {
			return err
		}
		defer gz.Close()

		log.Println("fetching latest", ct.String(), f)

		if err = unarchive(f, gz); err != nil {
			return err
		}
		return updateLatest(ct, true)
	}
}

func unarchive(f string, gz io.Reader) (err error) {
	parts := strings.Split(f, "-")
	if parts[0] != "geneos" {
		log.Println("file must be named geneos-TYPE-VERSION*.tar.gz:", f)
		return
	}
	DebugLog.Printf("parts=%v\n", parts)
	comp := CompType(parts[1])
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
		name := strings.TrimPrefix(hdr.Name, comp.String())
		path := filepath.Join(basedir, name)
		switch hdr.Typeflag {
		case tar.TypeReg:
			out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, hdr.FileInfo().Mode())
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
			DebugLog.Println("file:", path)
		case tar.TypeDir:
			if err = os.MkdirAll(path, hdr.FileInfo().Mode()); err != nil {
				return
			}
			DebugLog.Println("dir:", path)
		case tar.TypeSymlink, tar.TypeGNULongLink:
			link := strings.TrimPrefix(hdr.Linkname, "/")
			os.Symlink(link, path)
			DebugLog.Println("link:", path, "->", link)
		default:
			log.Printf("unsupported file type %c\n", hdr.Typeflag)
		}
	}
	log.Println("installed", f, "to", basedir)

	return
}

//
// update a version of component packages
//
// 'geneos update gateway 5.8.0'
// 'geneos update netprobe latest active_dev'
//
// defaults all components, 'latest' and 'active_prod'
//
// check if already the same, then
// stop, update, start any instances using that link
//
// latest is: [GA]N.M.P-DATE - GA is optional, ignore all other non-numeric
// prefixes. Sort N.M.P using almost semantic versioning
func commandUpdate(ct ComponentType, args []string) (err error) {
	return updateLatest(ct, false)
}

func updateLatest(ct ComponentType, readonly bool) error {
	version := "latest"
	base := "active_prod"
	basedir := filepath.Join(RunningConfig.ITRSHome, "packages", ct.String())
	basepath := filepath.Join(basedir, base)

	switch ct {
	case None:
		for _, t := range ComponentTypes() {
			updateLatest(t, readonly)
		}
	case Gateways, Netprobes, Licds:
		if version == "" || version == "latest" {
			version = getLatest(basedir)
		}
		current, err := os.Readlink(basepath)
		if err != nil && errors.Is(err, &fs.PathError{}) {
			log.Println("cannot read link", basepath)
		}
		if current != "" && readonly {
			return nil
		}
		// empty current is fine
		if current == version {
			log.Println(base, "is already linked to", version)
			return nil
		}
		insts := matchComponents(ct, "Base", base)
		// stop matching instances
		for _, i := range insts {
			stopInstance(i)
			defer startInstance(i)
		}
		if err = os.Remove(basepath); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err = os.Symlink(version, basepath); err != nil {
			return err
		}
		log.Println(ct.String(), base, "updated to", version)
	default:
		return ErrNotSupported
	}
	return nil
}

// given a directory find the "latest" version of the form
// [GA]M.N.P[-DATE] M, N, P are numbers, DATE is treated as a string
func getLatest(dir string) (latest string) {
	dirs, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalln(err)
	}
	m := make([]int, 3)
	for _, v := range dirs {
		if !v.IsDir() {
			continue
		}
		// strip 'GA' prefix and get name
		d := strings.TrimPrefix(v.Name(), "GA")
		s := strings.SplitN(d, ".", 3)
		n := slicetoi(s)

		for y := range m {
			if n[y] < m[y] {
				break
			}
			if n[y] > m[y] {
				latest = v.Name()
				m[y] = n[y]
				continue
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

const defaultURL = "https://resources.itrsgroup.com/download/latest"

type DownloadAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var downloadComponent = map[ComponentType]string{
	Gateways:  "Gateway+2",
	Netprobes: "Netprobe",
	Licds:     "Licence+Daemon",
}

//
func fetchLatest(ct ComponentType) (filename string, body io.ReadCloser, err error) {
	baseurl := RunningConfig.DownloadURL
	if baseurl == "" {
		baseurl = defaultURL
	}

	var resp *http.Response

	if RunningConfig.DefaultUser != "" {
		var authbody DownloadAuth
		authbody.Username = RunningConfig.DownloadUser
		authbody.Password = RunningConfig.DownloadPass

		var authjson []byte
		authjson, err = json.Marshal(authbody)
		if err != nil {
			log.Fatalln(err)
		}

		resp, err = http.Post(baseurl+"/"+downloadComponent[ct]+"?os=linux", "application/json", bytes.NewBuffer(authjson))
	} else {
		resp, err = http.Get(baseurl + "/" + downloadComponent[ct] + "?os=linux")
	}
	if err != nil {
		log.Fatalln(err)
	}
	if resp.StatusCode != 200 {
		log.Fatalln(resp.Status)
	}

	u := resp.Request.URL
	filename = filepath.Base(u.Path)
	body = resp.Body
	return
}
