package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func init() {
	RegsiterCommand(Command{
		Name:          "extract",
		Function:      commandExtract,
		ParseFlags:    extractFlag,
		ParseArgs:     defaultArgs,
		Wildcard:      true,
		ComponentOnly: true,
		CommandLine:   "geneos extract [-r REMOTE] [TYPE] | FILE [FILE...]",
		Summary:       `Extract files from downloaded Geneos packages. Intended for sites without Internet access.`,
		Description: `Extracts files from FILE(s) in to the packages/ directory. The filename(s) must of of the form:

	geneos-TYPE-VERSION*.tar.gz

The directory for the package is created using the VERSION from the archive
filename.

If a TYPE is given then the latest version from the packages/archives
directory for that TYPE is extracted, otherwise it is treated as a
normal file path. This is primarily for extracting to remote locations.

FLAGS:
	-r REMOTE - extract from local archive to remote. default is local. all means all remotes and local.`,
	})

	extractFlags = flag.NewFlagSet("extract", flag.ExitOnError)
	extractFlags.StringVar(&extractRemote, "r", string(ALL), "Perform on a remote. \"all\" means all remotes and locally")

	RegsiterCommand(Command{
		Name:          "download",
		Function:      commandDownload,
		ParseFlags:    downloadFlag,
		ParseArgs:     defaultArgs,
		Wildcard:      true,
		ComponentOnly: true,
		CommandLine:   "geneos download [-n] [-r REMOTE] [TYPE] [latest|FILTER|URL...]",
		Summary:       `Download and extract Geneos software archive.`,
		Description: `Download and extract the sources in the packages directory or latest version(s) from
the official download site. The filename must of of the format:

	geneos-TYPE-VERSION*.tar.gz
	
The TYPE, if supplied, limits the selection of downloaded archive(s). The directory
for the package is created using the VERSION from the archive filename.

The downloaded file is saved in the packages/archives/ direcvtory for
future re-use, especially for remote support.

FLAGS:
	-n - Do not save download archive
	-r REMOTE - download and extract from local archive to remote. default is local. all means all remotes and local.`,
	})

	downloadFlags = flag.NewFlagSet("download", flag.ExitOnError)
	downloadFlags.BoolVar(&downloadNosave, "n", false, "Do not save download")
	downloadFlags.BoolVar(&helpFlag, "h", false, helpUsage)
	downloadFlags.StringVar(&downloadRemote, "r", string(ALL), "Perform on a remote. \"all\" means all remotes and locally")

	RegsiterCommand(Command{
		Name:          "update",
		Function:      commandUpdate,
		ParseFlags:    updateFlag,
		ParseArgs:     defaultArgs,
		Wildcard:      true,
		ComponentOnly: true,
		CommandLine:   "geneos update [-b BASE] [-r REMOTE] [TYPE] VERSION",
		Summary:       `Update the active version of Geneos software.`,
		Description: `Update the symlink for the default base name of the package used to VERSION. The base directory,
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

Future version may support selecting a base other than 'active_prod'.

FLAGS:
	-b BASE - override the base link name, default active_prod
	-r REMOTE - update remote. default is local. 'all' means all remotes including local.`,
	})

	updateFlags = flag.NewFlagSet("update", flag.ExitOnError)
	updateFlags.StringVar(&updateBase, "b", "active_prod", "Override the base active_prod link name")
	updateFlags.StringVar(&updateRemote, "r", string(ALL), "Perform on a remote. \"all\" means all remotes and locally")
}

var extractFlags *flag.FlagSet
var extractRemote string

var downloadFlags *flag.FlagSet
var downloadNosave bool
var downloadRemote string

var updateFlags *flag.FlagSet
var updateBase string
var updateRemote string

func extractFlag(command string, args []string) []string {
	extractFlags.Parse(args)
	checkHelpFlag(command)
	return extractFlags.Args()
}

func downloadFlag(command string, args []string) []string {
	downloadFlags.Parse(args)
	checkHelpFlag(command)
	return downloadFlags.Args()
}

func updateFlag(command string, args []string) []string {
	updateFlags.Parse(args)
	checkHelpFlag(command)
	return updateFlags.Args()
}

//
// if there is no 'active_prod' link then attach it to the latest version
// installed
//
func commandExtract(ct Component, files []string, params []string) (err error) {
	if ct != None {
		logDebug.Println(ct.String())
		// archive directory is local only?
		archiveDir := rLOCAL.GeneosPath("packages", "archives")
		archiveFile := rLOCAL.LatestMatch(archiveDir, func(v os.DirEntry) bool {
			logDebug.Println(v.Name(), ct.String())
			switch ct {
			case Webserver:
				return !strings.Contains(v.Name(), "web-server")
			case FA2:
				return !strings.Contains(v.Name(), "fixanalyser2-netprobe")
			case FileAgent:
				return !strings.Contains(v.Name(), "file-agent")
			default:
				return !strings.Contains(v.Name(), ct.String())
			}
		})
		gz, _, err := rLOCAL.statAndOpenFile(filepath.Join(archiveDir, archiveFile))
		if err != nil {
			return err
		}
		defer gz.Close()
		r := GetRemote(RemoteName(extractRemote))
		if _, err = ct.Unarchive(r, archiveFile, gz); err != nil {
			log.Println("location:", extractRemote, err)
			return err
		}
		return nil
	}

	for _, file := range files {
		filename := filepath.Base(file)
		gz, _, err := rLOCAL.statAndOpenFile(file)
		if err != nil {
			return err
		}
		defer gz.Close()

		if extractRemote == string(ALL) {
			for _, r := range AllRemotes() {
				if _, err = ct.Unarchive(r, filename, gz); err != nil {
					log.Println("location:", r.InstanceName, err)
					continue
				}
			}
		} else {
			r := GetRemote(RemoteName(extractRemote))
			if _, err = ct.Unarchive(r, filename, gz); err != nil {
				log.Println("location:", r.InstanceName, err)
				return err
			}
		}
	}

	// create a symlink only if one doesn't exist
	r := GetRemote(RemoteName(extractRemote))
	return ct.UpdateToVersion(r, "latest", false)
}

func commandDownload(ct Component, files []string, params []string) (err error) {
	version := "latest"
	if len(files) > 0 {
		version = files[0]
	}

	if downloadRemote == string(ALL) {
		for _, r := range AllRemotes() {
			if err = ct.DownloadComponent(r, version); err != nil {
				logError.Println("location:", r.InstanceName, err)
				continue
			}
		}
		return
	}

	r := GetRemote(RemoteName(downloadRemote))
	if err = ct.DownloadComponent(r, version); err != nil {
		logError.Println("location:", downloadRemote, err)
		return err
	}
	return
}

func (ct Component) DownloadComponent(r *Remotes, version string) (err error) {
	switch ct {
	case Remote:
		// do nothing
		return nil
	case None:
		for _, t := range RealComponents() {
			if err = t.DownloadComponent(r, version); err != nil {
				if errors.Is(err, fs.ErrExist) {
					continue
				}
				logError.Println(err)
				return
			}
		}
		return nil
	default:
		filename, gz, err := ct.DownloadArchive(r, version)
		if err != nil {
			return err
		}
		defer gz.Close()

		logDebug.Println("downloaded", ct.String(), filename)

		var finalVersion string
		if finalVersion, err = ct.Unarchive(r, filename, gz); err != nil {
			if errors.Is(err, fs.ErrExist) {
				return nil
			}
			return err
		}
		if version == "latest" {
			return ct.UpdateToVersion(r, finalVersion, true)
		}
		return ct.UpdateToVersion(r, finalVersion, false)
	}
}

// how to split an archive name into type and version
var archiveRE = regexp.MustCompile(`^geneos-(web-server|fixanalyser2-netprobe|file-agent|\w+)-([\w\.-]+?)[\.-]?linux`)

func (ct Component) Unarchive(r *Remotes, filename string, gz io.Reader) (finalVersion string, err error) {
	parts := archiveRE.FindStringSubmatch(filename)
	if len(parts) == 0 {
		logError.Fatalf("invalid archive name format: %q", filename)
	}
	version := parts[2]
	filect := parseComponentName(parts[1])
	switch ct {
	case None, San:
		ct = filect
	case filect:
		break
	default:
		// mismatch
		logError.Fatalf("component type and archive mismatch: %q is not a %q", filename, ct)
	}

	basedir := r.GeneosPath("packages", ct.String(), version)
	logDebug.Println(basedir)
	if _, err = r.statFile(basedir); err == nil {
		return
	}
	if err = r.mkdirAll(basedir, 0775); err != nil {
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
		// strip leading component name (XXX - except webserver)
		// do not trust tar archives to contain safe paths
		var name string
		switch ct {
		case Webserver:
			name = hdr.Name
		case FA2:
			name = strings.TrimPrefix(hdr.Name, "fix-analyser2/")
		case FileAgent:
			name = strings.TrimPrefix(hdr.Name, "agent/")
		default:
			name = strings.TrimPrefix(hdr.Name, ct.String()+"/")
		}
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
			// check (and created) containing directories - account for munged tar files
			dir := filepath.Dir(fullpath)
			if err = r.mkdirAll(dir, 0775); err != nil {
				return
			}

			out, err := r.createFile(fullpath, hdr.FileInfo().Mode())
			if err != nil {
				return "", err
			}
			n, err := io.Copy(out, tr)
			if err != nil {
				out.Close()
				return "", err
			}
			if n != hdr.Size {
				log.Println("lengths different:", hdr.Size, n)
			}
			out.Close()

		case tar.TypeDir:
			if err = r.mkdirAll(fullpath, hdr.FileInfo().Mode()); err != nil {
				return
			}
		case tar.TypeSymlink, tar.TypeGNULongLink:
			if filepath.IsAbs(hdr.Linkname) {
				logError.Fatalln("archive contains absolute symlink target")
			}
			if _, err = r.statFile(fullpath); err != nil {
				if err = r.symlink(hdr.Linkname, fullpath); err != nil {
					logError.Fatalln(err)
				}
			}
		default:
			log.Printf("unsupported file type %c\n", hdr.Typeflag)
		}
	}
	log.Printf("extracted %q to %q\n", filename, basedir)
	finalVersion = filepath.Base(basedir)
	return
}

func commandUpdate(ct Component, args []string, params []string) (err error) {
	version := "latest"
	if len(args) > 0 {
		version = args[0]
	}
	r := GetRemote(RemoteName(updateRemote))
	return ct.UpdateToVersion(r, version, true)
}

// check selected version exists first
func (ct Component) UpdateToVersion(r *Remotes, version string, overwrite bool) (err error) {
	if components[ct].ParentType != None {
		ct = components[ct].ParentType
	}

	if r == rALL {
		for _, r := range AllRemotes() {
			if err = ct.UpdateToVersion(r, version, overwrite); err != nil {
				log.Println("could not update", r.InstanceName, err)
			}
		}
		return
	}

	basedir := r.GeneosPath("packages", ct.String())
	basepath := filepath.Join(basedir, updateBase)

	if ct == None {
		for _, t := range RealComponents() {
			if err = t.UpdateToVersion(r, version, overwrite); err != nil {
				log.Println(err)
			}
		}
		return nil
	}

	logDebug.Printf("checking and updating %s %s %q to %q", r.InstanceName, ct.String(), updateBase, version)

	if version == "" || version == "latest" {
		version = r.LatestMatch(basedir, func(d os.DirEntry) bool {
			return !d.IsDir()
		})
	}
	// does the version directory exist?
	current, err := r.readlink(basepath)
	if err != nil {
		logDebug.Println("cannot read link for existing version", basepath)
	}
	if _, err = r.statFile(filepath.Join(basedir, version)); err != nil {
		err = fmt.Errorf("update %s@%s to version %s failed", ct, r.InstanceName, version)
		return err
	}
	if current != "" && !overwrite {
		return nil
	}

	// empty current is fine
	if current == version {
		logDebug.Println(ct, updateBase, "is already linked to", current)
		return nil
	}
	// check remote only
	insts := ct.MatchComponents(r, "Base", updateBase)
	// stop matching instances
	for _, i := range insts {
		stopInstance(i, nil)
		defer startInstance(i, nil)
	}
	if err = r.removeFile(basepath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err = r.symlink(version, basepath); err != nil {
		return err
	}
	log.Println(ct, "on", r.InstanceName, updateBase, "updated to", version)
	return nil
}

var versRE = regexp.MustCompile(`(\d+(\.\d+){0,2})`)

// given a directory find the "latest" version of the form
// [GA]M.N.P[-DATE] M, N, P are numbers, DATE is treated as a string
func (r *Remotes) LatestMatch(dir string, fn func(os.DirEntry) bool) (latest string) {
	dirs, err := r.readDir(dir)
	if err != nil {
		logError.Fatalln(err)
	}
	max := make([]int, 3)
	for _, v := range dirs {
		if fn(v) {
			continue
		}
		// strip 'GA' prefix and get name
		d := strings.TrimPrefix(v.Name(), "GA")
		x := versRE.FindString(d)
		if x == "" {
			logDebug.Println(d, "does not match a valid directory pattern")
			continue
		}
		s := strings.SplitN(x, ".", 3)
		next := sliceAtoi(s)

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

func sliceAtoi(s []string) (n []int) {
	for _, x := range s {
		i, err := strconv.Atoi(x)
		if err != nil {
			i = 0
		}
		n = append(n, i)
	}
	return
}

// given a component type and a key/value pair, return matching
// instances
//
// also check if "Parent" is of the required type, then that also matches
//
func (ct Component) MatchComponents(r *Remotes, k, v string) (insts []Instances) {
	// also check for any other component types that have this as a parent, recurse
	for _, c := range components {
		if ct == c.ParentType {
			logDebug.Println("also matching", c.ComponentType.String())
			insts = append(insts, c.ComponentType.MatchComponents(r, k, v)...)
		}
	}

	for _, i := range ct.Instances(r) {
		if v == getString(i, i.Prefix(k)) {
			if err := i.Load(); err != nil {
				log.Println(i.Type(), i.Name(), "cannot load config")
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

func (ct Component) DownloadArchive(r *Remotes, version string) (filename string, body io.ReadCloser, err error) {
	baseurl := GlobalConfig["DownloadURL"]
	if baseurl == "" {
		baseurl = defaultURL
	}

	var resp *http.Response

	downloadURL, _ := url.Parse(baseurl)
	realpath, _ := url.Parse(components[ct].DownloadBase)
	v := url.Values{}
	// XXX OS filter for EL8 here - to test
	// cannot fetch partial versions for el8
	platform := ""
	p, ok := r.OSInfo["PLATFORM_ID"]
	if ok {
		s := strings.Split(p, ":")
		if len(s) > 1 {
			platform = "-" + s[1]
		}
	}
	v.Set("os", "linux")
	if version != "latest" {
		v.Set("title", version+platform)
	} else if platform != "" {
		v.Set("title", platform)
	}
	realpath.RawQuery = v.Encode()
	source := downloadURL.ResolveReference(realpath).String()
	logDebug.Println("source url:", source)

	// if a download user is set then issue a POST with username and password
	// in a JSON body, else just try the GET
	if GlobalConfig["DownloadUser"] != "" {
		var authbody DownloadAuth
		authbody.Username = GlobalConfig["DownloadUser"]
		authbody.Password = GlobalConfig["DownloadPass"]

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
	}

	filename, err = filenameFromHTTPResp(resp, downloadURL)
	if err != nil {
		logError.Fatalln(err)
	}

	// check size against downloaded archive and serve local instead, regardless
	// of -n flag
	archiveDir := filepath.Join(Geneos(), "packages", "archives")
	rLOCAL.mkdirAll(archiveDir, 0775)
	archivePath := filepath.Join(archiveDir, filename)
	s, err := rLOCAL.statFile(archivePath)
	if err == nil && s.st.Size() == resp.ContentLength {
		logDebug.Println("file with same size already exists, skipping save")
		f, _, err := rLOCAL.statAndOpenFile(archivePath)
		if err != nil {
			return filename, body, nil
		}
		resp.Body.Close()
		return filename, f, nil
	}

	// transient download
	if downloadNosave {
		body = resp.Body
		return
	}

	// save the file archive and rewind, return
	f, err := os.Create(archivePath)
	if err != nil {
		log.Fatalln(err)
	}
	if _, err = io.Copy(f, resp.Body); err != nil {
		log.Fatalln(err)
	}
	resp.Body.Close()
	if _, err = f.Seek(0, 0); err != nil {
		log.Fatalln(err)
	}
	body = f
	return
}
