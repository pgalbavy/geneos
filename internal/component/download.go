package component

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/host"
)

func DownloadComponent(r *host.Host, ct ComponentType, version, basename string, overwrite bool) (err error) {
	switch ct {
	case None:
		for _, t := range RealComponents() {
			if err = DownloadComponent(r, t, version, basename, overwrite); err != nil {
				if errors.Is(err, fs.ErrExist) {
					continue
				}
				logError.Println(err)
				return
			}
		}
		return nil
	default:
		if r == host.ALL {
			return ErrInvalidArgs
		}
		filename, f, err := OpenArchive(r, ct, version)
		if err != nil {
			return err
		}
		defer f.Close()

		// call install here instead
		if err = Unarchive(r, ct, filename, basename, f, overwrite); err != nil {
			if errors.Is(err, fs.ErrExist) {
				return nil
			}
			return err
		}
		return nil
	}
}

var versRE = regexp.MustCompile(`(\d+(\.\d+){0,2})`)
var anchoredVersRE = regexp.MustCompile(`^(\d+(\.\d+){0,2})$`)

func MatchVersion(v string) bool {
	return anchoredVersRE.MatchString(v)
}

// given a directory find the "latest" version of the form
// [GA]M.N.P[-DATE] M, N, P are numbers, DATE is treated as a string
func latestMatch(r *host.Host, dir, filter string, fn func(os.DirEntry) bool) (latest string) {
	dirs, err := r.ReadDir(dir)
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

func CheckArchive(r *host.Host, ct ComponentType, version string) (filename string, resp *http.Response, err error) {
	baseurl := viper.GetString("DownloadURL")
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

	filename, err = FilenameFromHTTPResp(resp, resp.Request.URL)
	if err != nil {
		return
	}

	logDebug.Printf("download check for %s versions %q returned %s (%d bytes)", ct, version, filename, resp.ContentLength)
	return
}

func OpenArchive(r *host.Host, ct ComponentType, version string) (filename string, body io.ReadCloser, err error) {
	var finalURL string
	var resp *http.Response

	if installLocal {
		// archive directory is local only?
		archiveDir := host.LOCAL.GeneosPath("packages", "downloads")
		filename = latestMatch(host.LOCAL, archiveDir, "", func(v os.DirEntry) bool {
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
		if filename == "" {
			err = fmt.Errorf("local installation selected but no suitable file found for %s on %s (%w)", ct, r.String(), ErrInvalidArgs)
			return
		}
		var f io.ReadSeekCloser
		if f, err = host.LOCAL.Open(filepath.Join(archiveDir, filename)); err != nil {
			err = fmt.Errorf("local installation selected but no suitable file found for %s on %s (%w)", ct, r.String(), err)
			return
		}
		body = f
		return
	}

	if filename, resp, err = CheckArchive(r, ct, version); err != nil {
		return
	}
	finalURL = resp.Request.URL.String()
	logDebug.Println("final URL", finalURL)

	archiveDir := filepath.Join(Geneos(), "packages", "downloads")
	host.LOCAL.MkdirAll(archiveDir, 0775)
	archivePath := filepath.Join(archiveDir, filename)
	s, err := host.LOCAL.Stat(archivePath)
	if err == nil && s.St.Size() == resp.ContentLength {
		if f, err := host.LOCAL.Open(archivePath); err == nil {
			logDebug.Println("not downloading, file already exists:", archivePath)
			resp.Body.Close()
			return filename, f, nil
		}
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

func Unarchive(r *host.Host, ct ComponentType, filename, basename string, gz io.Reader, overwrite bool) (err error) {
	var version string

	if installOverride == "" {
		parts := archiveRE.FindStringSubmatch(filename)
		if len(parts) == 0 {
			return fmt.Errorf("%q: %w", filename, ErrInvalidArgs)
		}
		version = parts[2]
		// check the component in the filename
		// special handling for Sans
		ctFromFile := ParseComponentName(parts[1])
		switch ct {
		case ctFromFile:
			break
		case component.None, san.San:
			ct = ctFromFile
		default:
			// mismatch
			logError.Fatalf("component type and archive mismatch: %q is not a %q", filename, ct)
		}
	} else {
		s := strings.SplitN(installOverride, ":", 2)
		if len(s) != 2 {
			err = fmt.Errorf("type/version override must be in the form TYPE:VERSION (%w)", ErrInvalidArgs)
			return
		}
		ct = ParseComponentName(s[0])
		if ct == Unknown {
			return fmt.Errorf("invalid component type %q (%w)", s[0], ErrInvalidArgs)
		}
		version = s[1]
		if !MatchVersion(version) {
			return fmt.Errorf("invalid version %q (%w)", s[1], ErrInvalidArgs)
		}
	}

	basedir := r.GeneosPath("packages", ct.String(), version)
	logDebug.Println(basedir)
	if _, err = r.Stat(basedir); err == nil {
		// something is already using that dir
		// XXX - option to delete and overwrite?
		return
	}
	if err = r.MkdirAll(basedir, 0775); err != nil {
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
		if name, err = host.CleanRelativePath(name); err != nil {
			logError.Fatalln(err)
		}
		fullpath := filepath.Join(basedir, name)
		switch hdr.Typeflag {
		case tar.TypeReg:
			// check (and created) containing directories - account for munged tar files
			dir := filepath.Dir(fullpath)
			if err = r.MkdirAll(dir, 0775); err != nil {
				return
			}

			var out io.WriteCloser
			if out, err = r.Create(fullpath, hdr.FileInfo().Mode()); err != nil {
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
			if err = r.MkdirAll(fullpath, hdr.FileInfo().Mode()); err != nil {
				return
			}

		case tar.TypeSymlink, tar.TypeGNULongLink:
			if filepath.IsAbs(hdr.Linkname) {
				logError.Fatalln("archive contains absolute symlink target")
			}
			if _, err = r.Stat(fullpath); err != nil {
				if err = r.Symlink(hdr.Linkname, fullpath); err != nil {
					logError.Fatalln(err)
				}
			}

		default:
			log.Printf("unsupported file type %c\n", hdr.Typeflag)
		}
	}
	log.Printf("installed %q to %q\n", filename, basedir)
	return UpdateToVersion(r, ct, version, basename, overwrite)
}

func FilenameFromHTTPResp(resp *http.Response, u *url.URL) (filename string, err error) {
	cd, ok := resp.Header[http.CanonicalHeaderKey("content-disposition")]
	if !ok && resp.Request.Response != nil {
		cd, ok = resp.Request.Response.Header[http.CanonicalHeaderKey("content-disposition")]
	}
	if ok {
		_, params, err := mime.ParseMediaType(cd[0])
		if err == nil {
			if f, ok := params["filename"]; ok {
				filename = f
			}
		}
	}

	// if no content-disposition, then grab the path from the response URL
	if filename == "" {
		filename, err = host.CleanRelativePath(path.Base(u.Path))
		if err != nil {
			return
		}
	}
	return
}

func OpenLocalFileOrURL(source string) (from io.ReadCloser, filename string, err error) {
	u, err := url.Parse(source)
	if err != nil {
		return
	}

	switch {
	case u.Scheme == "https" || u.Scheme == "http":
		var resp *http.Response
		resp, err = http.Get(u.String())
		if err != nil {
			return nil, "", err
		}

		from = resp.Body
		if resp.StatusCode > 299 {
			return nil, "", fmt.Errorf("server returned %s for %q", resp.Status, source)
		}
		filename, err = FilenameFromHTTPResp(resp, resp.Request.URL)
	case source == "-":
		from = os.Stdin
		filename = "STDIN"
	default:
		filename = filepath.Base(source)
		from, err = os.Open(source)
		if err != nil {
			return
		}
	}
	return
}

func ReadLocalFileOrURL(source string) (b []byte, err error) {
	var from io.ReadCloser
	from, _, err = OpenLocalFileOrURL(source)
	if err != nil {
		return
	}
	defer from.Close()
	return io.ReadAll(from)
}

// check selected version exists first
func UpdateToVersion(r *host.Host, ct ComponentType, version, basename string, overwrite bool) (err error) {
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
			if err = UpdateToVersion(r, rct, version, basename, overwrite); err != nil && !errors.Is(err, os.ErrNotExist) {
				logError.Println(err)
			}
		}
		return nil
	}

	if r == host.ALL {
		for _, r := range host.AllHosts() {
			if err = UpdateToVersion(r, ct, version, basename, overwrite); err != nil && !errors.Is(err, os.ErrNotExist) {
				logError.Println(err)
			}
		}
		return
	}

	if ct == None {
		for _, t := range RealComponents() {
			if err = UpdateToVersion(r, t, version, basename, overwrite); err != nil && !errors.Is(err, os.ErrNotExist) {
				logError.Println(err)
			}
		}
		return nil
	}

	// from here remotes and component types are specific

	logDebug.Printf("checking and updating %s on %s %q to %q", ct, r, basename, version)

	basedir := r.GeneosPath("packages", ct.String())
	basepath := filepath.Join(basedir, basename)

	if version == "latest" {
		version = ""
	}
	version = latestMatch(r, basedir, "^"+version, func(d os.DirEntry) bool {
		return !d.IsDir()
	})
	if version == "" {
		return fmt.Errorf("%q verion of %s on %s: %w", originalVersion, ct, r, os.ErrNotExist)
	}

	// does the version directory exist?
	existing, err := r.ReadLink(basepath)
	if err != nil {
		logDebug.Println("cannot read link for existing version", basepath)
	}

	// before removing existing link, check there is something to link to
	if _, err = r.Stat(filepath.Join(basedir, version)); err != nil {
		return fmt.Errorf("%q version of %s on %s: %w", version, ct, r, os.ErrNotExist)
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
	if err = r.Remove(basepath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err = r.Symlink(version, basepath); err != nil {
		return err
	}
	log.Println(ct, "on", r, basename, "updated to", version)
	return nil
}
