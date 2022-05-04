package geneos

import (
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

	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/host"
)

const defaultURL = "https://resources.itrsgroup.com/download/latest/"

func init() {
	viper.SetDefault("downloadurl", defaultURL)
}

// how to split an archive name into type and version
var archiveRE = regexp.MustCompile(`^geneos-(web-server|fixanalyser2-netprobe|file-agent|\w+)-([\w\.-]+?)[\.-]?linux`)

type dload struct {
	override  string
	local     bool
	nosave    bool
	overwrite bool
	version   string
	basename  string
}

type DownloadOptions func(*dload)

func NoSave(n bool) DownloadOptions {
	return func(d *dload) { d.nosave = n }
}

func LocalOnly(l bool) DownloadOptions {
	return func(d *dload) { d.local = l }
}

func Overwrite(o bool) DownloadOptions {
	return func(d *dload) { d.overwrite = o }
}

func Override(s string) DownloadOptions {
	return func(d *dload) { d.override = s }
}

func Version(v string) DownloadOptions {
	return func(d *dload) { d.version = v }
}

func Basename(b string) DownloadOptions {
	return func(d *dload) { d.basename = b }
}

func Download(r *host.Host, ct *Component, options ...DownloadOptions) (err error) {
	d := &dload{}
	for _, opt := range options {
		opt(d)
	}
	switch ct {
	case nil:
		for _, t := range RealComponents() {
			if err = Download(r, t, options...); err != nil {
				if errors.Is(err, fs.ErrExist) {
					continue
				}
				return
			}
		}
		return nil
	default:
		if r == host.ALL {
			return ErrInvalidArgs
		}
		filename, f, err := OpenArchive(r, ct, options...)
		if err != nil {
			return err
		}
		defer f.Close()

		// call install here instead
		if err = Unarchive(r, ct, filename, f, options...); err != nil {
			if errors.Is(err, fs.ErrExist) {
				return nil
			}
			return err
		}
		return nil
	}
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
