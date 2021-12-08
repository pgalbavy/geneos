package main

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	commands["install"] = Command{commandInstall, parseArgs, "install"}
}

// 'geneos install gateway file://path/*tgz'
func commandInstall(comp ComponentType, files []string) (err error) {

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
