package geneos

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"wonderland.org/geneos/internal/host"
)

// check selected version exists first
func Update(h *host.Host, ct *Component, version, basename string, overwrite bool) (err error) {
	if ct == nil {
		for _, t := range RealComponents() {
			if err = Update(h, t, version, basename, overwrite); err != nil && !errors.Is(err, os.ErrNotExist) {
				logError.Println(err)
			}
		}
		return nil
	}

	if version == "" {
		version = "latest"
	}

	originalVersion := version

	// before updating a specific type on a specific remote, loop
	// through related types, remotes and components. continue to
	// other items if a single update fails?
	//
	// XXX this is a common pattern, should abstract it a bit like loopCommand

	if ct.RelatedTypes != nil {
		for _, rct := range ct.RelatedTypes {
			if err = Update(h, rct, version, basename, overwrite); err != nil && !errors.Is(err, os.ErrNotExist) {
				logError.Println(err)
			}
		}
		return nil
	}

	if h == host.ALL {
		for _, r := range host.AllHosts() {
			if err = Update(r, ct, version, basename, overwrite); err != nil && !errors.Is(err, os.ErrNotExist) {
				logError.Println(err)
			}
		}
		return
	}

	// from here remotes and component types are specific

	logDebug.Printf("checking and updating %s on %s %q to %q", ct, h, basename, version)

	basedir := h.GeneosJoinPath("packages", ct.String())
	basepath := filepath.Join(basedir, basename)

	if version == "latest" {
		version = ""
	}
	version = latest(h, basedir, "^"+version, func(d os.DirEntry) bool {
		return !d.IsDir()
	})
	if version == "" {
		return fmt.Errorf("%q verion of %s on %s: %w", originalVersion, ct, h, os.ErrNotExist)
	}

	// does the version directory exist?
	existing, err := h.ReadLink(basepath)
	if err != nil {
		logDebug.Println("cannot read link for existing version", basepath)
	}

	// before removing existing link, check there is something to link to
	if _, err = h.Stat(filepath.Join(basedir, version)); err != nil {
		return fmt.Errorf("%q version of %s on %s: %w", version, ct, h, os.ErrNotExist)
	}

	if (existing != "" && !overwrite) || existing == version {
		return nil
	}

	if err = h.Remove(basepath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err = h.Symlink(version, basepath); err != nil {
		return err
	}
	log.Println(ct, "on", h, basename, "updated to", version)
	return nil
}
