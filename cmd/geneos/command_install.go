package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	commands["install"] = commandInstall
}

func commandInstall(comp ComponentType, args []string) error {
	return fmt.Errorf("pacxkage installation net yet supported")
}

func install(c Component) (err error) {
	var names []string
	for _, archive := range names {
		f := filepath.Base(archive)
		parts := strings.Split(f, "-")
		if parts[0] == "geneos" {
			fmt.Printf("parts=%v\n", parts)
			comp := CompType(parts[1])
			if comp != None {
				version := parts[2]
				os.MkdirAll(filepath.Join(Config.Root, "packages", comp.String(), version), 0775)
				gz, _ := os.Open(archive)
				t, _ := gzip.NewReader(gz)
				tr := tar.NewReader(t)
				for {
					hdr, err := tr.Next()
					if err == io.EOF {
						break
					}
					if err != nil {
						//
					}
					log.Println("file:", hdr.Name, "size", hdr.Size)

				}
				t.Close()
				gz.Close()
			}
		}
	}
	return
}
