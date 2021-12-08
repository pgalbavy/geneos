package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func init() {
	commands["install"] = Command{commandInstall, parseArgs, "install"}
	commands["update"] = Command{commandUpdate, parseArgs, "update"}
}

// 'geneos install gateway file://path/*tgz'
func commandInstall(ct ComponentType, files []string) (err error) {

NAMES:
	for _, archive := range files {
		f := filepath.Base(archive)
		parts := strings.Split(f, "-")
		if parts[0] != "geneos" {
			log.Println("archive must be named geneos-COMPONENT-VERSION*.tar.gz:", archive)
			continue
		}
		DebugLog.Printf("parts=%v\n", parts)
		comp := CompType(parts[1])
		if comp == None || comp == Unknown {
			log.Println("component type required")
			continue
		}
		version := parts[2]
		basedir := filepath.Join(Config.ITRSHome, "packages", comp.String(), version)
		err = os.MkdirAll(basedir, 0775)
		if err != nil {
			log.Println(err)
			continue
		}
		gz, err := os.Open(archive)
		if err != nil {
			log.Println(err)
			continue
		}
		t, err := gzip.NewReader(gz)
		if err != nil {
			log.Println(err)
			gz.Close()
			continue
		}
		tr := tar.NewReader(t)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Println(err)
				t.Close()
				gz.Close()
				continue NAMES
			}
			// log.Println("file:", hdr.Name, "size", hdr.Size)
			// strip leading component name
			name := strings.TrimPrefix(hdr.Name, comp.String())
			path := filepath.Join(basedir, name)
			switch hdr.Typeflag {
			case tar.TypeReg:
				out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, hdr.FileInfo().Mode())
				if err != nil {
					log.Println(err)
					continue
				}
				n, err := io.Copy(out, tr)
				if err != nil {
					//
				}
				if n != hdr.Size {
					log.Println("lengths different:", hdr.Size, n)
				}
				out.Close()
				DebugLog.Println("file:", path)
			case tar.TypeDir:
				err = os.MkdirAll(path, hdr.FileInfo().Mode())
				if err != nil {
					log.Println(err)
					continue
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
		t.Close()
		gz.Close()
		log.Println("installed", f, "to", basedir)

	}
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
