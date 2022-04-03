package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
)

//
// Provide instance mv/cp/clone/etc.
//
// Existing is single type, single instance 'mv' between any remotes, self processes args
//
// Also need:
//
// mv arbitrary types by name to arbitrary remote, e.g. "geneos mv localhost @remote"
// without needing to name type. Also mv en masse from one remote to another
//
//
// resolve remote location from name, if source is only singular
//
// cp "live" and "offline" ? can we cp just the core configs but leave an instance running?
//
//
// clone to distribute across multiple remotes (or just cp?)
//

func init() {
	RegsiterCommand(Command{
		Name:          "mv",
		Function:      commandMv,
		ParseFlags:    defaultFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: true,
		CommandLine:   `geneos mv TYPE FROM TO`,
		Summary:       `Move an instance`,
		Description: `Move an instance. TYPE is requied to resolve any ambiguities if two
instances share the same name. No configuration changes are made outside the
instance JSON config file. As any existing .rc legacy file is never
changed, this will migrate the instance from .rc to JSON. The
instance is stopped and restarted after the instance is moved. It is
an error to try to move an instance to one that already exists with
the same name.

If the component support Rebuild then this is run after the move but before the restart.
This allows SANs to be updated as expected.

Moving across remotes is supported.`,
	})

}

func commandMv(ct Component, args []string, params []string) (err error) {
	var stopped, done bool
	if ct == None || len(args) != 2 {
		return ErrInvalidArgs
	}

	srcname := args[0]
	dstname := args[1]

	logDebug.Println("mv", ct, srcname, dstname)
	src, err := ct.FindInstance(srcname)
	if err != nil {
		return fmt.Errorf("%s %q not matched to exactly one instance", ct, srcname)
	}
	if err = migrateConfig(src); err != nil {
		return fmt.Errorf("%s %s cannot be migrated to new configuration format", ct, srcname)
	}

	// if dstname is just a remote, tack the src prefix on to the start
	// let fursth calls check for syntax and validity
	if strings.HasPrefix(dstname, "@") {
		dstname = src.Name() + dstname
	}

	dst, err := ct.GetInstance(dstname)
	dstname, _ = splitInstanceName(dstname)

	if err == nil {
		return fmt.Errorf("%s already exists", dst)
	}

	if _, err = findInstancePID(src); err != ErrProcNotExist {
		if err = stopInstance(src, nil); err == nil {
			// cannot use defer startInstance() here as we have
			// not yet created the new instance
			stopped = true
			defer func() {
				if !done {
					startInstance(src, nil)
				}
			}()
		} else {
			return fmt.Errorf("cannot stop %s", srcname)
		}
	}

	// now a full clean
	if err = src.Clean(true, []string{}); err != nil {
		return
	}

	// move directory
	if err = copyTree(src.Remote(), src.Home(), dst.Remote(), dst.Home()); err != nil {
		return
	}

	// delete one or the other, depending
	defer func() {
		if done {
			// once we are done, try to delete old instance
			orig, _ := ct.GetInstance(srcname)
			logDebug.Println("removing old instance", orig)
			orig.Remote().removeAll(orig.Home())
			log.Println(ct, orig, "moved to", dst)
		} else {
			// remove new instance
			logDebug.Println("removing new instance", dst)
			dst.Remote().removeAll(dst.Home())
		}
	}()

	// update oldconf here and then write that out as if it were newconf
	// this gets around the defaults set in newconf being incomplete and wrong
	if err = changeDirPrefix(src, src.Remote().GeneosRoot(), dst.Remote().GeneosRoot()); err != nil {
		return
	}

	// update *Home manually, as it's not just the prefix
	if err = setField(src, src.Prefix("Home"), filepath.Join(dst.Type().ComponentDir(dst.Remote()), dstname)); err != nil {
		return
	}

	// after path updates, rename non paths
	ib := src.Base()
	ib.InstanceLocation = dst.Remote().RemoteName()
	ib.InstanceRemote = dst.Remote()
	ib.InstanceName = dstname

	// update any component name only if the same as the instance name
	if getString(src, src.Prefix("Name")) == srcname {
		if err = setField(src, src.Prefix("Name"), dstname); err != nil {
			return
		}
	}

	// config changes don't matter until writing config succeeds
	if err = writeInstanceConfig(src); err != nil {
		return
	}

	//	oldconf.Unload()
	if err = src.Rebuild(false); err != nil && err != ErrNotSupported {
		return
	}

	done = true
	if stopped {
		return startInstance(src, nil)
	}
	return nil
}

// move a directory, between any combination of local or remote locations
//
func copyTree(srcRemote *Remotes, srcDir string, dstRemote *Remotes, dstDir string) (err error) {
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
			dstPath := filepath.Join(dstDir, path)
			srcPath := filepath.Join(srcDir, path)
			return copyDirEntry(fi, srcRemote, srcPath, dstRemote, dstPath)
		})
		return
	}

	var s *sftp.Client
	if s, err = srcRemote.sftpOpenSession(); err != nil {
		return
	}

	w := s.Walk(srcDir)
	for w.Step() {
		if w.Err() != nil {
			logError.Println(w.Path(), err)
			continue
		}
		fi := w.Stat()
		srcPath := w.Path()
		dstPath := filepath.Join(dstDir, strings.TrimPrefix(w.Path(), srcDir))
		if err = copyDirEntry(fi, srcRemote, srcPath, dstRemote, dstPath); err != nil {
			logError.Println(err)
			continue
		}
	}
	return
}

func copyDirEntry(fi fs.FileInfo, srcRemote *Remotes, srcPath string, dstRemote *Remotes, dstPath string) (err error) {
	switch {
	case fi.IsDir():
		ds, err := srcRemote.statFile(srcPath)
		if err != nil {
			logError.Println(err)
			return err
		}
		if err = dstRemote.mkdirAll(dstPath, ds.st.Mode()); err != nil {
			return err
		}
	case fi.Mode()&fs.ModeSymlink != 0:
		link, err := srcRemote.readlink(srcPath)
		if err != nil {
			return err
		}
		if err = dstRemote.symlink(link, dstPath); err != nil {
			return err
		}
	default:
		sf, ss, err := srcRemote.statAndOpenFile(srcPath)
		if err != nil {
			return err
		}
		defer sf.Close()
		df, err := dstRemote.createFile(dstPath, ss.st.Mode())
		if err != nil {
			return err
		}
		defer df.Close()
		if _, err = io.Copy(df, sf); err != nil {
			return err
		}
	}
	return nil
}
