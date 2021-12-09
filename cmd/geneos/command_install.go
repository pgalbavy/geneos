package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func init() {
	commands["install"] = Command{commandInstall, parseArgs, "install"}
	commands["update"] = Command{commandUpdate, parseArgs, "update"}
}

//
// option to fetch latest versions from remote URL (or directory)
//

// 'geneos install gateway [files]'
func commandInstall(ct ComponentType, files []string) (err error) {
	if len(files) == 1 && files[0] == "latest" {
		f, gz, err := fetchLatest(ct)
		if err != nil {
			return err
		}
		defer gz.Close()

		log.Println("fetching latest", ct.String(), f)

		err = unarchive(f, gz)
		if err != nil {
			log.Println(err)
		}
		return nil
	}

	for _, archive := range files {
		f := filepath.Base(archive)
		gz, err := os.Open(archive)
		if err != nil {
			return err
		}
		defer gz.Close()

		err = unarchive(f, gz)
		if err != nil {
			log.Println(err)
		}
	}
	return
}

func unarchive(f string, gz io.Reader) (err error) {
	parts := strings.Split(f, "-")
	if parts[0] != "geneos" {
		log.Println("file must be named geneos-COMPONENT-VERSION*.tar.gz:", f)
		return
	}
	DebugLog.Printf("parts=%v\n", parts)
	comp := CompType(parts[1])
	if comp == None || comp == Unknown {
		log.Println("component type required")
		return
	}
	version := parts[2]
	basedir := filepath.Join(Config.ITRSHome, "packages", comp.String(), version)
	err = os.MkdirAll(basedir, 0775)
	if err != nil {
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
			err = os.MkdirAll(path, hdr.FileInfo().Mode())
			if err != nil {
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
	version := "latest"
	base := "active_prod"
	basedir := filepath.Join(Config.ITRSHome, "packages", ct.String())
	basepath := filepath.Join(basedir, base)

	switch ct {
	case Gateway, Netprobe:
		if version == "latest" {
			version = getLatest(basedir)
		}
		current, err := os.Readlink(basepath)
		if err != nil && errors.Is(err, &fs.PathError{}) {
			log.Println("cannot read link", basepath)
		}
		// empty current is fine
		if current == version {
			log.Println(base, "is already linked to", version)
			return nil
		}
		insts := matchComponents(ct, "Base", base)
		// stop matching instances
		for _, i := range insts {
			stop(i)
			defer start(i)
		}
		err = os.Remove(basepath)
		if err != nil {
			log.Println(err)
		}
		err = os.Symlink(version, basepath)
		if err != nil {
			log.Println(err)
			return err
		}
		log.Println(ct.String(), base, "updated to", version)
		return nil
	default:
		return ErrNotSupported
	}
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

func slicetoi(s []string) (n []int) {
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
func matchComponents(ct ComponentType, k, v string) (insts []Instance) {
	for _, i := range instances(ct) {
		if v == getString(i, Prefix(i)+k) {
			err := loadConfig(&i, false)
			if err != nil {
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
// Gateway -> Gateway+2
// Netprobe -> Netprobe
// Licd -> Licence+Daemon
// Webserver -> Web+Dashboard
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
	Gateway:  "Gateway+2",
	Netprobe: "Netprobe",
	Licd:     "License+Daemon",
}

func fetchLatest(ct ComponentType) (filename string, body io.ReadCloser, err error) {
	baseurl := Config.DownloadURL
	if baseurl == "" {
		baseurl = defaultURL
	}

	var authbody DownloadAuth
	authbody.Username = Config.DownloadUser
	authbody.Password = Config.DownloadPass

	authjson, err := json.Marshal(authbody)
	if err != nil {
		log.Fatalln(err)
	}

	resp, err := http.Post(baseurl+"/"+downloadComponent[ct]+"?os=linux", "application/json", bytes.NewBuffer(authjson))
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
