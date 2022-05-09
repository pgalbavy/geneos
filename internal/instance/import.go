package instance

import (
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
	"wonderland.org/geneos/internal/utils"
)

func ImportFile(h *host.Host, home string, user string, source string, options ...geneos.GeneosOptions) (err error) {
	var backuppath string
	var from io.ReadCloser

	if h == host.ALL {
		return geneos.ErrInvalidArgs
	}

	uid, gid, _, err := utils.GetIDs(user)
	if err != nil {
		return err
	}

	// destdir becomes the absolute path for the imported file
	destdir := home
	// destfile is the basename of the import path, empty if the source
	// filename should be kept
	destfile := ""

	// if the source is a http(s) url then skip '=' split (protect queries in URL)
	if !strings.HasPrefix(source, "https://") && !strings.HasPrefix(source, "http://") {
		splitsource := strings.SplitN(source, "=", 2)
		if len(splitsource) > 1 {
			// do some basic validation on user-supplied destination
			if splitsource[0] == "" {
				logError.Fatalln("dest path empty")
			}
			destfile, err = host.CleanRelativePath(splitsource[0])
			if err != nil {
				logError.Fatalln("dest path must be relative to (and in) instance directory")
			}
			// if the destination exists is it a directory?
			if s, err := h.Stat(filepath.Join(home, destfile)); err == nil {
				if s.St.IsDir() {
					destdir = filepath.Join(home, destfile)
					destfile = ""
				}
			}
			source = splitsource[1]
			if source == "" {
				logError.Fatalln("no source defined")
			}
		}
	}

	from, filename, err := geneos.OpenLocalFileOrURL(source)
	if err != nil {
		logError.Fatalln(err)
	}
	defer from.Close()

	if destfile == "" {
		destfile = filename
	}
	destfile = filepath.Join(destdir, destfile)

	// check to containing directory, as destfile above may be a
	// relative path under destdir and not just a filename
	if _, err := h.Stat(filepath.Dir(destfile)); err != nil {
		err = h.MkdirAll(filepath.Dir(destfile), 0775)
		if err != nil && !errors.Is(err, fs.ErrExist) {
			logError.Fatalln(err)
		}
		// if created by root, chown the last directory element
		if err == nil && utils.IsSuperuser() {
			if err = h.Chown(filepath.Dir(destfile), uid, gid); err != nil {
				return err
			}
		}
	}

	// xxx - wrong way around. create tmp first, move over later
	if s, err := h.Stat(destfile); err == nil {
		if !s.St.Mode().IsRegular() {
			logError.Fatalln("dest exists and is not a plain file")
		}
		datetime := time.Now().UTC().Format("20060102150405")
		backuppath = destfile + "." + datetime + ".old"
		if err = h.Rename(destfile, backuppath); err != nil {
			return err
		}
	}

	cf, err := h.Create(destfile, 0664)
	if err != nil {
		return err
	}
	defer cf.Close()

	if utils.IsSuperuser() {
		if err = h.Chown(destfile, uid, gid); err != nil {
			h.Remove(destfile)
			if backuppath != "" {
				if err = h.Rename(backuppath, destfile); err != nil {
					return err
				}
				return err
			}
		}
	}

	if _, err = io.Copy(cf, from); err != nil {
		return err
	}
	log.Printf("imported %q to %s:%s", source, h.String(), destfile)
	return nil
}
