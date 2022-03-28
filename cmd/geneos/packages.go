package main

import (
	"archive/tar"
	"compress/gzip"
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
	"time"
)

// how to split an archive name into type and version
var archiveRE = regexp.MustCompile(`^geneos-(web-server|fixanalyser2-netprobe|file-agent|\w+)-([\w\.-]+?)[\.-]?linux`)

func init() {
	RegisterDirs([]string{
		"packages/downloads",
	})

	RegsiterCommand(Command{
		Name:          "install",
		Function:      commandInstall,
		ParseFlags:    installFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   "geneos install [-l] [-n] [-r REMOTE] [-T TYPE] [-V VERSION] [TYPE] | FILE|URL [FILE|URL...] | [VERSION | FILTER]",
		Summary:       `Install files from downloaded Geneos packages. Intended for sites without Internet access.`,
		Description: `Installs files from FILE(s) in to the packages/ directory. The filename(s) must of of the form:

	geneos-TYPE-VERSION*.tar.gz

The directory for the package is created using the VERSION from the archive
filename unless overridden by the -T and -V flags.

If a TYPE is given then the latest version from the packages/downloads
directory for that TYPE is installed, otherwise it is treated as a
normal file path. This is primarily for installing to remote locations.

TODO:

Install only changes creates a base link if one does not exist.
To update an existing base link use the -U option. This stops any
instance, updates the link and starts the instance up again.

Use the update command to explicitly change the base link after installation.

Use the -b flag to change the base link name from the default 'active_prod'. This also
applies when using -U.

"geneos install gateway"
"geneos install fa2 5.5 -U"
"geneos install netprobe -b active_dev -U"
"geneos update gateway -b active_prod"


FLAGS:
	-b BASE	The name of the package base version symlink, default active_prod
	-l	Local archives only, don't try to download from official site
	-n	Do not save a copy of downloads in packages/downloads
	-r REMOTE - install from local archive to remote. default is local. all means all remotes and local
	-U	Update base symlink, stopping and starting only running instances that use it
	-T TYPE	Override component type in archive name
	-V VERSION	Override version in archive name
	`,
	})

	installFlags = flag.NewFlagSet("install", flag.ExitOnError)
	installFlags.StringVar(&installBase, "b", "active_prod", "Override the base active_prod link name")

	installFlags.BoolVar(&installLocal, "l", false, "Install from local files only")
	installFlags.BoolVar(&installNoSave, "n", false, "Do not save a local copy of any downloads")
	installFlags.StringVar(&installRemote, "r", string(ALL), "Perform on a remote. \"all\" means all remotes and locally")

	installFlags.BoolVar(&installUpdate, "U", false, "Update the base directory symlink")
	installFlags.StringVar(&installTypeOverride, "T", "", "Override the component type in the archive file name")
	installFlags.StringVar(&installVersionOverride, "V", "", "Override the version number in the archive file name")

	installFlags.BoolVar(&helpFlag, "h", false, helpUsage)

	RegsiterCommand(Command{
		Name:          "update",
		Function:      commandUpdate,
		ParseFlags:    updateFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: true,
		CommandLine:   "geneos update [-b BASE] [-r REMOTE] [TYPE] VERSION",
		Summary:       `Update the active version of Geneos software.`,
		Description: `Update the symlink for the default base name of the package used to
	VERSION. The base directory, for historical reasons, is 'active_prod'
	and is usally linked to the latest version of a component type in the
	packages directory. VERSION can either be a directory name or the
	literal 'latest'. If TYPE is not supplied, all supported component
	types are updated to VERSION.

	Update will stop all matching instances of the each type before
	updating the link and starting them up again, but only if the
	instance uses the 'active_prod' basename.

	The 'latest' version is based on directory names of the form:

	[GA]X.Y.Z

	Where X, Y, Z are each ordered in ascending numerical order. If a
	directory starts 'GA' it will be selected over a directory with the
	same numerical versions. All other directories name formats will
	result in unexpected behaviour.

	Future version may support selecting a base other than 'active_prod'.

	FLAGS:
		-b BASE - override the base link name, default active_prod
		-r REMOTE - update remote. default is local. 'all' means all remotes including local.`,
	})

	updateFlags = flag.NewFlagSet("update", flag.ExitOnError)
	updateFlags.StringVar(&updateBase, "b", "active_prod", "Override the base active_prod link name")
	updateFlags.StringVar(&updateRemote, "r", string(ALL), "Perform on a remote. \"all\" means all remotes and locally")
}

var installFlags *flag.FlagSet
var installLocal, installNoSave, installUpdate bool
var installRemote, installBase string
var installTypeOverride, installVersionOverride string

func installFlag(command string, args []string) []string {
	installFlags.Parse(args)
	checkHelpFlag(command)
	return installFlags.Args()
}

var updateFlags *flag.FlagSet
var updateBase string
var updateRemote string

func updateFlag(command string, args []string) []string {
	updateFlags.Parse(args)
	checkHelpFlag(command)
	return updateFlags.Args()
}

func commandInstall(ct Component, args []string, params []string) (err error) {
	// first, see if user wants a particular version
	version := "latest"

	for n := 0; n < len(params); n++ {
		if anchoredVersRE.MatchString(params[n]) {
			version = params[n]
			params[n] = params[len(params)-1]
			params = params[:len(params)-1]
		}
	}

	if ct == Unknown {
		ct = None
	}

	r := GetRemote(RemoteName(installRemote))

	// if we have a component on the command line then use an archive in packages/downloads
	// or download from official web site unless -l is given. version numbers checked.
	// default to 'latest'
	//
	// overrides do not work in this case as the version and type have to be part of the
	// archive file name
	if ct != None {
		logDebug.Printf("downloading %q version of %s to %s remote(s)", version, ct, installRemote)
		filename, f, err := ct.openArchive(rLOCAL, version)
		if err != nil {
			return err
		}
		defer f.Close()

		if r == rALL {
			for _, r := range AllRemotes() {
				if err = ct.unarchive(r, filename, installBase, f, installUpdate); err != nil {
					logError.Println(err)
					continue
				}
			}
		} else {
			if err = ct.unarchive(r, filename, installBase, f, installUpdate); err != nil {
				return err
			}
			logDebug.Println("installed", ct.String())
		}

		return nil
	}

	// no component type means we might want file or url or auto url
	if len(params) == 0 {
		// normal download here
		if installLocal {
			log.Println("install -l (local) flag with no component or file/url")
			return nil
		}
		var rs []*Remotes
		if installRemote == string(ALL) {
			rs = AllRemotes()
		} else {
			rs = []*Remotes{GetRemote(RemoteName(installRemote))}
		}

		for _, r := range rs {
			if err = ct.downloadComponent(r, version, installBase, installUpdate); err != nil {
				logError.Println(err)
				continue
			}
		}
		// downloadComponent() in the loop above calls updateToVersion()
		return nil
	}

	// work through command line params and try to install them using the naming format
	// of standard downloads - fix versioning
	for _, file := range params {
		f, filename, err := openLocalFileOrURL(file)
		if err != nil {
			log.Println(err)
			continue
		}
		defer f.Close()

		if installRemote == string(ALL) {
			for _, r := range AllRemotes() {
				// what is finalVersion ?
				if err = ct.unarchive(r, filename, installBase, f, installUpdate); err != nil {
					logError.Println(err)
					continue
				}
			}
		} else {
			r := GetRemote(RemoteName(installRemote))
			if err = ct.unarchive(r, filename, installBase, f, installUpdate); err != nil {
				return err
			}
			logDebug.Println("installed", ct.String())
		}
	}

	return nil
}

func (ct Component) downloadComponent(r *Remotes, version, basename string, overwrite bool) (err error) {
	switch ct {
	case Remote:
		// do nothing
		return nil
	case None:
		for _, t := range RealComponents() {
			if err = t.downloadComponent(r, version, basename, overwrite); err != nil {
				if errors.Is(err, fs.ErrExist) {
					continue
				}
				logError.Println(err)
				return
			}
		}
		return nil
	default:
		if r == rALL {
			return ErrInvalidArgs
		}
		filename, f, err := ct.openArchive(r, version)
		if err != nil {
			return err
		}
		defer f.Close()

		// call install here instead
		if err = ct.unarchive(r, filename, basename, f, overwrite); err != nil {
			if errors.Is(err, fs.ErrExist) {
				return nil
			}
			return err
		}
		return nil
	}
}

func commandUpdate(ct Component, args []string, params []string) (err error) {
	version := "latest"
	if len(args) > 0 {
		version = args[0]
	}
	r := GetRemote(RemoteName(updateRemote))
	if err = ct.updateToVersion(r, version, updateBase, true); err != nil && errors.Is(err, ErrNotFound) {
		return nil
	}
	return
}

// check selected version exists first
func (ct Component) updateToVersion(r *Remotes, version, basename string, overwrite bool) (err error) {
	if version == "" {
		version = "latest"
	}

	originalVersion := version

	// before updating a specific type on a specific remote, loop
	// through related types, remotes and components. continue to
	// other items if a single update fails?
	//
	// XXX this is a common pattern, should abstract it a bit like loopCommand

	if components[ct].RelatedTypes != nil {
		for _, rct := range components[ct].RelatedTypes {
			if err = rct.updateToVersion(r, version, basename, overwrite); err != nil && !errors.Is(err, ErrNotFound) {
				logError.Println(err)
			}
		}
		return nil
	}

	if r == rALL {
		for _, r := range AllRemotes() {
			if err = ct.updateToVersion(r, version, basename, overwrite); err != nil && !errors.Is(err, ErrNotFound) {
				logError.Println(err)
			}
		}
		return
	}

	if ct == None {
		for _, t := range RealComponents() {
			if err = t.updateToVersion(r, version, basename, overwrite); err != nil && !errors.Is(err, ErrNotFound) {
				logError.Println(err)
			}
		}
		return nil
	}

	// from here remotes and component types are specific

	logDebug.Printf("checking and updating %s on %s %q to %q", ct, r.InstanceName, basename, version)

	basedir := r.GeneosPath("packages", ct.String())
	basepath := filepath.Join(basedir, basename)

	if version == "latest" {
		version = ""
	}
	version = r.latestMatch(basedir, "^"+version, func(d os.DirEntry) bool {
		return !d.IsDir()
	})
	if version == "" {
		return fmt.Errorf("%q verion of %s on %s: %w", originalVersion, ct, r.InstanceName, ErrNotFound)
	}

	// does the version directory exist?
	existing, err := r.readlink(basepath)
	if err != nil {
		logDebug.Println("cannot read link for existing version", basepath)
	}

	// before removing existing link, check there is something to link to
	if _, err = r.statFile(filepath.Join(basedir, version)); err != nil {
		return fmt.Errorf("%q version of %s on %s: %w", version, ct, r.InstanceName, ErrNotFound)
	}

	if (existing != "" && !overwrite) || existing == version {
		return nil
	}

	// check remote only
	insts := ct.findInstances(r, "Base", basename)

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
	log.Println(ct, "on", r.InstanceName, basename, "updated to", version)
	return nil
}

var versRE = regexp.MustCompile(`(\d+(\.\d+){0,2})`)
var anchoredVersRE = regexp.MustCompile(`^(\d+(\.\d+){0,2})$`)

// given a directory find the "latest" version of the form
// [GA]M.N.P[-DATE] M, N, P are numbers, DATE is treated as a string
func (r *Remotes) latestMatch(dir, filter string, fn func(os.DirEntry) bool) (latest string) {
	dirs, err := r.readDir(dir)
	if err != nil {
		return
	}

	filterRE, err := regexp.Compile(filter)
	if err != nil {
		logDebug.Printf("invalid filter regexp %q", filter)
	}
	for n := 0; n < len(dirs); n++ {
		if !filterRE.MatchString(dirs[n].Name()) {
			dirs[n] = dirs[len(dirs)-1]
			dirs = dirs[:len(dirs)-1]
		}
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

		// make sure we have three levels, fill with 0
		for len(s) < len(max) {
			s = append(s, "0")
		}
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
// not right for FA2 Sans...
//
func (ct Component) findInstances(r *Remotes, k, v string) (insts []Instances) {
	if ct == None {
		for _, rct := range RealComponents() {
			insts = append(insts, rct.findInstances(r, k, v)...)
		}
		return
	}

	// also check for any other component types that have related types
	for _, rct := range components[ct].RelatedTypes {
		logDebug.Println(ct, "also matching", rct)
		insts = append(insts, rct.findInstances(r, k, v)...)
	}

	for _, i := range ct.GetInstancesForComponent(r) {
		if !getBool(i, "ConfigLoaded") {
			log.Println("cannot load configuration for", i)
			continue
		}
		if v == getString(i, i.Prefix(k)) {
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

//
// locate and open the remote archive using the download conventions
//

func (ct Component) checkArchive(r *Remotes, version string) (filename string, resp *http.Response, err error) {
	baseurl := GlobalConfig["DownloadURL"]
	if baseurl == "" {
		baseurl = defaultURL
	}

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

	if resp, err = http.Head(source); err != nil {
		logError.Fatalln(err)
	}

	if resp.StatusCode > 299 {
		err = fmt.Errorf("cannot download %s package version %s: %s", ct, version, resp.Status)
		resp.Body.Close()
		return
	}

	filename, err = filenameFromHTTPResp(resp, resp.Request.URL)
	if err != nil {
		return
	}

	logDebug.Printf("download check for %s versions %q returned %s (%d bytes)", ct, version, filename, resp.ContentLength)
	return
}

//
func (ct Component) openArchive(r *Remotes, version string) (filename string, body io.ReadCloser, err error) {
	var finalURL string
	var resp *http.Response

	if installLocal {
		// archive directory is local only?
		archiveDir := rLOCAL.GeneosPath("packages", "downloads")
		filename = rLOCAL.latestMatch(archiveDir, "", func(v os.DirEntry) bool {
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
		var f io.ReadSeekCloser
		if f, _, err = rLOCAL.statAndOpenFile(filepath.Join(archiveDir, filename)); err == nil {
			body = f
		}
		return
	}

	filename, resp, err = ct.checkArchive(r, version)
	if err != nil {
		return
	}
	finalURL = resp.Request.URL.String()
	logDebug.Println("final URL", finalURL)

	archiveDir := filepath.Join(Geneos(), "packages", "downloads")
	rLOCAL.mkdirAll(archiveDir, 0775)
	archivePath := filepath.Join(archiveDir, filename)
	if f, s, err := rLOCAL.statAndOpenFile(archivePath); err == nil && s.st.Size() == resp.ContentLength {
		logDebug.Println("not downloading, file already exists:", archivePath)
		resp.Body.Close()
		return filename, f, nil
	}

	resp, err = http.Get(finalURL)
	if err != nil {
		logError.Fatalln(err)
	}
	if resp.StatusCode > 299 {
		err = fmt.Errorf("cannot download %s package version %q: %s", ct, version, resp.Status)
		resp.Body.Close()
		return
	}

	// transient download
	if installNoSave {
		body = resp.Body
		return
	}

	// save the file archive and rewind, return
	var w *os.File
	w, err = os.Create(archivePath)
	if err != nil {
		return
	}
	log.Printf("downloading %s package version %q to %s", ct, version, archivePath)
	t1 := time.Now()
	if _, err = io.Copy(w, resp.Body); err != nil {
		return
	}
	t2 := time.Now()
	resp.Body.Close()
	b, d := resp.ContentLength, t2.Sub(t1).Seconds()
	bps := 0.0
	if d > 0 {
		bps = float64(b) / d
	}
	log.Printf("downloaded %d bytes in %.3f seconds (%.0f bytes/sec)", b, d, bps)
	if _, err = w.Seek(0, 0); err != nil {
		return
	}
	body = w
	return
}

func (ct Component) unarchive(r *Remotes, filename, basename string, gz io.Reader, overwrite bool) (err error) {
	var version string

	if !(installTypeOverride != "" && installVersionOverride != "") {
		parts := archiveRE.FindStringSubmatch(filename)
		if len(parts) == 0 {
			logError.Fatalf("invalid archive name format: %q", filename)
		}
		version = parts[2]
		// check the component in the filename
		// special handling for Sans
		ctFromFile := parseComponentName(parts[1])
		switch ct {
		case ctFromFile:
			break
		case None, San:
			ct = ctFromFile
		default:
			// mismatch
			logError.Fatalf("component type and archive mismatch: %q is not a %q", filename, ct)
		}
	}

	if installTypeOverride != "" {
		ct = parseComponentName(installTypeOverride)
		if ct == Unknown {
			return fmt.Errorf("no component type %q: %w", installTypeOverride, ErrInvalidArgs)
		}
	}
	if installVersionOverride != "" {
		version = installVersionOverride
		if !anchoredVersRE.MatchString(version) {
			return fmt.Errorf("invalid version %q: %w", installVersionOverride, ErrInvalidArgs)
		}
	}

	basedir := r.GeneosPath("packages", ct.String(), version)
	logDebug.Println(basedir)
	if _, err = r.statFile(basedir); err == nil {
		// something is already using that dir
		// XXX - option to delete and overwrite?
		return
	}
	if err = r.mkdirAll(basedir, 0775); err != nil {
		return
	}

	t, err := gzip.NewReader(gz)
	if err != nil {
		// cannot gunzip file
		return
	}
	defer t.Close()

	var name string
	var fnname func(string) string

	switch ct {
	case Webserver:
		fnname = func(name string) string { return name }
	case FA2:
		fnname = func(name string) string {
			return strings.TrimPrefix(name, "fix-analyser2/")
		}
	case FileAgent:
		fnname = func(name string) string {
			return strings.TrimPrefix(name, "agent/")
		}
	default:
		fnname = func(name string) string {
			return strings.TrimPrefix(name, ct.String()+"/")
		}
	}

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
		// strip leading component name (XXX - except webserver)
		// do not trust tar archives to contain safe paths

		if name = fnname(hdr.Name); name == "" {
			continue
		}
		if name, err = cleanRelativePath(name); err != nil {
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

			var out io.WriteCloser
			if out, err = r.createFile(fullpath, hdr.FileInfo().Mode()); err != nil {
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
	log.Printf("installed %q to %q\n", filename, basedir)
	return ct.updateToVersion(r, version, basename, overwrite)
}
