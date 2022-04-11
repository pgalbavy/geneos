package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//
// Provide instance move/copy/clone/etc.
//
// Existing is single type, single instance 'move' between any remotes, self processes args
//
// Also need:
//
// move arbitrary types by name to arbitrary remote, e.g. "geneos move localhost @remote"
// without needing to name type. Also move en masse from one remote to another
//
//
// resolve remote location from name, if source is only singular
//
// copy "live" and "offline" ? can we copy just the core configs but leave an instance running?
//
//
// clone to distribute across multiple remotes (or just copy?)
//

func init() {
	RegisterCommand(Command{
		Name:          "move",
		Function:      commandMove,
		ParseFlags:    defaultFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: true,
		CommandLine:   `geneos move [TYPE] SOURCE DESTINATION`,
		Summary:       `Move (or rename) instances`,
		Description: `Move (or rename) instances. As any existing legacy .rc
file is never changed, this will migrate the instance from .rc to
JSON. The instance is stopped and restarted after the instance is
moved. It is an error to try to move an instance to one that already
exists with the same name.

If the component support Rebuild then this is run after the move but
before the restart. This allows SANs to be updated as expected.

Moving across remotes is supported.`,
	})

	RegisterCommand(Command{
		Name:          "copy",
		Function:      commandCopy,
		ParseFlags:    defaultFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: true,
		CommandLine:   `geneos copy [TYPE] SOURCE DESTINATION`,
		Summary:       `Copy instances`,
		Description: `Copy instances. As any existing legacy .rc file is never changed,
this will migrate the instance from .rc to JSON. The instance is
stopped and restarted after the instance is moved. It is an error to
try to copy an instance to one that already exists with the same
name.

If the component support Rebuild then this is run after the move but
before the restart. This allows SANs to be updated as expected.

Moving across remotes is supported.`,
	})
}

// XXX add more wildcard support - src = @remote for all instances, auto
// component type loops etc.
func commandMove(ct Component, args []string, params []string) (err error) {
	if len(args) != 2 {
		return ErrInvalidArgs
	}

	return ct.copyInstance(args[0], args[1], true)
}

// XXX use case:
// gateway standby instance copy
// distribute common config netprobe across multiple remotes
// also create remotes as required?
func commandCopy(ct Component, args []string, params []string) (err error) {
	if len(args) != 2 {
		return ErrInvalidArgs
	}

	return ct.copyInstance(args[0], args[1], false)
}

func (ct Component) copyInstance(srcname, dstname string, remove bool) (err error) {
	var stopped, done bool
	if srcname == dstname {
		return fmt.Errorf("source and destination must have different names and/or locations")
	}

	logDebug.Println(ct, srcname, dstname)

	// move/copy all instances from remote
	// destination must also be a remote and different and exist
	if strings.HasPrefix(srcname, "@") {
		if !strings.HasPrefix(dstname, "@") {
			return fmt.Errorf("%w: destination must be a remote when source is a remote", ErrInvalidArgs)
		}
		srcremote := strings.TrimPrefix(srcname, "@")
		dstremote := strings.TrimPrefix(dstname, "@")
		if srcremote == dstremote {
			return fmt.Errorf("%w: src and destination remotes must be different", ErrInvalidArgs)
		}
		sr := GetRemote(RemoteName(srcremote))
		if !sr.Loaded() {
			return fmt.Errorf("%w: source remote %q not found", ErrNotFound, srcremote)
		}
		dr := GetRemote(RemoteName(dstremote))
		if !dr.Loaded() {
			return fmt.Errorf("%w: destination remote %q not found", ErrNotFound, dstremote)
		}
		// they both exist, now loop through all instances on src and try to move/copy
		for _, name := range ct.FindNames(sr) {
			ct.copyInstance(name, dstname, remove)
		}
		return nil
	}

	if ct == None {
		for _, t := range RealComponents() {
			if err = t.copyInstance(srcname, dstname, remove); err != nil {
				logDebug.Println(err)
				continue
			}
		}
		return nil
	}

	src, err := ct.FindInstance(srcname)
	if err != nil {
		return fmt.Errorf("%w: %q %q", err, ct, srcname)
	}
	if err = migrateConfig(src); err != nil {
		return fmt.Errorf("%s %s cannot be migrated to new configuration format", ct, srcname)
	}

	// if dstname is just a remote, tack the src prefix on to the start
	// let further calls check for syntax and validity
	if strings.HasPrefix(dstname, "@") {
		dstname = src.Name() + dstname
	}

	dst, err := ct.GetInstance(dstname)
	if err != nil {
		logDebug.Println(err)
	}
	if dst.Loaded() {
		return fmt.Errorf("%s already exists", dst)
	}
	dst.Unload()

	if _, err = findInstancePID(src); err != ErrProcNotFound {
		if err = stopInstance(src, nil); err == nil {
			stopped = true
			defer func(c Instances) {
				if !done {
					startInstance(c, nil)
				}
			}(src)
		} else {
			return fmt.Errorf("cannot stop %s", srcname)
		}
	}

	// now a full clean
	if err = Clean(src, true, []string{}); err != nil {
		return
	}

	_, ds, dr := SplitInstanceName(dstname, rLOCAL)

	// do a dance here to deep copy-ish the dst
	realdst := dst
	b, _ := json.Marshal(src)
	if err = json.Unmarshal(b, &realdst); err != nil {
		logError.Println(err)
	}

	// after path updates, rename non paths
	ib := realdst.Base()
	ib.InstanceLocation = dst.Remote().RemoteName()
	ib.InstanceRemote = dst.Remote()
	ib.InstanceName = ds

	// move directory
	if err = copyTree(src.Remote(), src.Home(), dr, dst.Home()); err != nil {
		return
	}

	// delete one or the other, depending
	defer func(srcname string, srcrem *Remotes, srchome string, dst Instances) {
		if done {
			if remove {
				// once we are done, try to delete old instance
				logDebug.Println("removing old instance", srcname)
				srcrem.RemoveAll(srchome)
				log.Println(srcname, "moved to", dst)
			} else {
				log.Println(srcname, "copied to", dstname)
			}
		} else {
			// remove new instance
			logDebug.Println("removing new instance", dst)
			dst.Remote().RemoveAll(dst.Home())
		}
	}(src.String(), src.Remote(), src.Home(), dst)

	// XXX update src here and then write that out as if it were dst
	// this gets around the defaults set in dst being incomplete (and hence wrong)
	if err = changeDirPrefix(realdst, src.Remote().GeneosRoot(), dr.GeneosRoot()); err != nil {
		logDebug.Println(err)
		return
	}

	// update *Home manually, as it's not just the prefix
	if err = setField(realdst, dst.Prefix("Home"), filepath.Join(dst.Type().ComponentDir(dr), ds)); err != nil {
		logDebug.Println(err)
		return
	}
	// dst.Unload()

	// fetch a new port if remotes are different and port is already used
	if src.Remote() != dr {
		srcport := getInt(src, src.Prefix("Port"))
		dstports := dr.getPorts()
		if _, ok := dstports[int(srcport)]; ok {
			dstport := dr.nextPort(dst.Type())
			if err = setField(realdst, dst.Prefix("Port"), fmt.Sprint(dstport)); err != nil {
				logDebug.Println(err)
				return
			}
		}
	}

	// update any component name only if the same as the instance name
	if getString(src, src.Prefix("Name")) == srcname {
		if err = setField(realdst, dst.Prefix("Name"), dstname); err != nil {
			logDebug.Println(err)
			return
		}
	}

	// config changes don't matter until writing config succeeds
	if err = writeInstanceConfig(realdst); err != nil {
		logDebug.Println(err)
		return
	}

	// src.Unload()
	if err = realdst.Rebuild(false); err != nil && err != ErrNotSupported && err != ErrNoAction {
		logDebug.Println(err)
		return
	}

	done = true
	if stopped {
		return startInstance(realdst, nil)
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

	s, err := srcRemote.sftpOpenSession()
	if err != nil {
		return err
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
		ds, err := srcRemote.Stat(srcPath)
		if err != nil {
			logError.Println(err)
			return err
		}
		if err = dstRemote.MkdirAll(dstPath, ds.st.Mode()); err != nil {
			return err
		}
	case fi.Mode()&fs.ModeSymlink != 0:
		link, err := srcRemote.ReadLink(srcPath)
		if err != nil {
			return err
		}
		if err = dstRemote.Symlink(link, dstPath); err != nil {
			return err
		}
	default:
		ss, err := srcRemote.Stat(srcPath)
		if err != nil {
			return err
		}
		sf, err := srcRemote.Open(srcPath)
		if err != nil {
			return err
		}
		defer sf.Close()
		df, err := dstRemote.Create(dstPath, ss.st.Mode())
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
