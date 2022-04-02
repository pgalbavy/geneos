package main

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
)

// move a directory, between any combination of local or remote locations
//
func moveDirectory(srcRemote *Remotes, srcDir string, dstRemote *Remotes, dstDir string) (err error) {
	if srcRemote == rALL || dstRemote == rALL {
		return ErrInvalidArgs
	}

	if srcRemote == rLOCAL {
		filesystem := os.DirFS(srcDir)
		fs.WalkDir(filesystem, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				logError.Println(err)
				return nil
			}
			fi, err := d.Info()
			if err != nil {
				logError.Println(err)
				return nil
			}
			log.Println(path)
			dstPath := filepath.Join(dstDir, path)
			srcPath := filepath.Join(srcDir, path)
			return moveEntry(fi, srcRemote, srcPath, dstRemote, dstPath)
		})
	}

	var s *sftp.Client
	if s, err = srcRemote.sftpOpenSession(); err != nil {
		return
	}

	w := s.Walk(srcDir)
	for w.Step() {
		log.Println(w.Path())
		if w.Err() != nil {
			logError.Println(w.Path(), err)
			continue
		}
		fi := w.Stat()
		srcPath := w.Path()
		dstPath := filepath.Join(dstDir, strings.TrimPrefix(w.Path(), srcDir))
		if err = moveEntry(fi, srcRemote, srcPath, dstRemote, dstPath); err != nil {
			logError.Println(err)
			continue
		}
	}
	return
}

func moveEntry(fi fs.FileInfo, srcRemote *Remotes, srcPath string, dstRemote *Remotes, dstPath string) (err error) {
	switch {
	case fi.IsDir():
		ds, err := srcRemote.statFile(srcPath)
		if err != nil {
			logError.Println(err)
			return err
		}
		logDebug.Println("mkdir", dstPath)
		if err = dstRemote.mkdirAll(dstPath, ds.st.Mode()); err != nil {
			log.Println(err)
			return err
		}
	case fi.Mode()&fs.ModeSymlink != 0:
		log.Println("move symlink", srcPath)
		link, err := srcRemote.readlink(srcPath)
		if err != nil {
			logError.Println(err)
			return err
		}
		if err = dstRemote.symlink(link, dstPath); err != nil {
			logError.Println(err)
			return err
		}
		logDebug.Println("linked", dstPath, "to", link)
	default:
		sf, ss, err := srcRemote.statAndOpenFile(srcPath)
		if err != nil {
			logError.Println(err)
			return err
		}
		df, err := dstRemote.createFile(dstPath, ss.st.Mode())
		if err != nil {
			logError.Println(err)
			return err
		}
		if _, err = io.Copy(df, sf); err != nil {
			logError.Println(err)
			return err
		}
		sf.Close()
		df.Close()
		logDebug.Println("copied", srcPath, "to", dstPath)
	}
	return nil
}
