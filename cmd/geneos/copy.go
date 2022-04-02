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

	oldname := args[0]
	newname := args[1]

	logDebug.Println("mv", ct, oldname, newname)
	oldconf, err := ct.GetInstance(oldname)
	if err != nil {
		return fmt.Errorf("%s %s not found", ct, oldname)
	}
	if err = migrateConfig(oldconf); err != nil {
		return fmt.Errorf("%s %s cannot be migrated to new configuration format", ct, oldname)
	}

	newconf, err := ct.GetInstance(newname)
	newname, _ = splitInstanceName(newname)

	if err == nil {
		return fmt.Errorf("%s already exists", newconf)
	}

	if _, err = findInstancePID(oldconf); err != ErrProcNotExist {
		if err = stopInstance(oldconf, nil); err == nil {
			// cannot use defer startInstance() here as we have
			// not yet created the new instance
			stopped = true
			defer func() {
				if !done {
					startInstance(oldconf, nil)
				}
			}()
		} else {
			return fmt.Errorf("cannot stop %s", oldname)
		}
	}

	// now a full clean
	if err = oldconf.Clean(true, []string{}); err != nil {
		return
	}

	// move directory
	if err = copyTree(oldconf.Remote(), oldconf.Home(), newconf.Remote(), newconf.Home()); err != nil {
		return
	}

	// delete one or the other, depending
	defer func() {
		if done {
			// once we are done, try to delete old instance
			oldold, _ := ct.GetInstance(oldname)
			logDebug.Println("removing old instance", oldold)
			oldold.Remote().removeAll(oldold.Home())
			log.Println(ct, oldold, "moved to", newconf)
		} else {
			// remove new instance
			logDebug.Println("removing new instance", newconf)
			newconf.Remote().removeAll(newconf.Home())
		}
	}()

	// update oldconf here and then write that out as if it were newconf
	// this gets around the defaults set in newconf being incomplete and wrong
	if err = changeDirPrefix(oldconf, oldconf.Remote().GeneosRoot(), newconf.Remote().GeneosRoot()); err != nil {
		return
	}

	// update *Home manually, as it's not just the prefix
	if err = setField(oldconf, oldconf.Prefix("Home"), filepath.Join(newconf.Type().ComponentDir(newconf.Remote()), newname)); err != nil {
		return
	}

	// after path updates, rename non paths
	ib := oldconf.Base()
	ib.InstanceLocation = newconf.Remote().RemoteName()
	ib.InstanceRemote = newconf.Remote()
	ib.InstanceName = newname

	// update any component name only if the same as the instance name
	if getString(oldconf, oldconf.Prefix("Name")) == oldname {
		if err = setField(oldconf, oldconf.Prefix("Name"), newname); err != nil {
			return
		}
	}

	// config changes don't matter until writing config succeeds
	if err = writeInstanceConfig(oldconf); err != nil {
		return
	}

	//	oldconf.Unload()
	if err = oldconf.Rebuild(false); err != nil && err != ErrNotSupported {
		return
	}

	done = true
	if stopped {
		return startInstance(oldconf, nil)
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
