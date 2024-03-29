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
func Update(h *host.Host, ct *Component, options ...GeneosOptions) (err error) {
	opts := EvalOptions(options...)
	if ct == nil {
		for _, t := range RealComponents() {
			if err = Update(h, t, options...); err != nil && !errors.Is(err, os.ErrNotExist) {
				logError.Println(err)
			}
		}
		return nil
	}

	if opts.version == "" {
		opts.version = "latest"
	}

	originalVersion := opts.version

	// before updating a specific type on a specific host, loop
	// through related types, hosts and components. continue to
	// other items if a single update fails?
	//
	// XXX this is a common pattern, should abstract it a bit like loopCommand

	if ct.RelatedTypes != nil {
		for _, rct := range ct.RelatedTypes {
			if err = Update(h, rct, options...); err != nil && !errors.Is(err, os.ErrNotExist) {
				logError.Println(err)
			}
		}
		return nil
	}

	if h == host.ALL {
		for _, h := range host.AllHosts() {
			if err = Update(h, ct, options...); err != nil && !errors.Is(err, os.ErrNotExist) {
				logError.Println(err)
			}
		}
		return
	}

	// from here hosts and component types are specific

	logDebug.Printf("checking and updating %s on %s %q to %q", ct, h, opts.basename, opts.version)

	basedir := h.GeneosJoinPath("packages", ct.String())
	basepath := filepath.Join(basedir, opts.basename)

	if opts.version == "latest" {
		opts.version = ""
	}
	opts.version = latest(h, basedir, "^"+opts.version, func(d os.DirEntry) bool {
		return !d.IsDir()
	})
	if opts.version == "" {
		return fmt.Errorf("%q version of %s on %s: %w", originalVersion, ct, h, os.ErrNotExist)
	}

	// does the version directory exist?
	existing, err := h.Readlink(basepath)
	if err != nil {
		logDebug.Println("cannot read link for existing version", basepath)
	}

	// before removing existing link, check there is something to link to
	if _, err = h.Stat(filepath.Join(basedir, opts.version)); err != nil {
		return fmt.Errorf("%q version of %s on %s: %w", opts.version, ct, h, os.ErrNotExist)
	}

	if (existing != "" && !opts.overwrite) || existing == opts.version {
		return nil
	}

	if opts.restart {
		// this cannot call 'instance' methods as that would be a dependency loop...

	}

	if err = h.Remove(basepath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err = h.Symlink(opts.version, basepath); err != nil {
		return err
	}
	log.Println(ct, h.Path(basepath), "updated to", opts.version)
	return nil
}
