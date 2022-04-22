package instance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wonderland.org/geneos/internal/geneos"
	"wonderland.org/geneos/internal/host"
)

func CopyInstance(ct *geneos.Component, srcname, dstname string, remove bool) (err error) {
	var stopped, done bool
	if srcname == dstname {
		return fmt.Errorf("source and destination must have different names and/or locations")
	}

	logDebug.Println(ct, srcname, dstname)

	// move/copy all instances from remote
	// destination must also be a remote and different and exist
	if strings.HasPrefix(srcname, "@") {
		if !strings.HasPrefix(dstname, "@") {
			return fmt.Errorf("%w: destination must be a remote when source is a remote", geneos.ErrInvalidArgs)
		}
		srcremote := strings.TrimPrefix(srcname, "@")
		dstremote := strings.TrimPrefix(dstname, "@")
		if srcremote == dstremote {
			return fmt.Errorf("%w: src and destination remotes must be different", geneos.ErrInvalidArgs)
		}
		sr := host.Get(host.Name(srcremote))
		if !sr.Loaded() {
			return fmt.Errorf("%w: source remote %q not found", os.ErrNotExist, srcremote)
		}
		dr := host.Get(host.Name(dstremote))
		if !dr.Loaded() {
			return fmt.Errorf("%w: destination remote %q not found", os.ErrNotExist, dstremote)
		}
		// they both exist, now loop through all instances on src and try to move/copy
		for _, name := range AllNames(sr, ct) {
			CopyInstance(ct, name, dstname, remove)
		}
		return nil
	}

	if ct == nil {
		for _, t := range geneos.RealComponents() {
			if err = CopyInstance(t, srcname, dstname, remove); err != nil {
				logDebug.Println(err)
				continue
			}
		}
		return nil
	}

	src, err := Match(ct, srcname)
	if err != nil {
		return fmt.Errorf("%w: %q %q", err, ct, srcname)
	}
	if err = Migrate(src); err != nil {
		return fmt.Errorf("%s %s cannot be migrated to new configuration format", ct, srcname)
	}

	// if dstname is just a remote, tack the src prefix on to the start
	// let further calls check for syntax and validity
	if strings.HasPrefix(dstname, "@") {
		dstname = src.Name() + dstname
	}

	dst, err := Get(ct, dstname)
	if err != nil {
		logDebug.Println(err)
	}
	if dst.Loaded() {
		return fmt.Errorf("%s already exists", dst)
	}
	dst.Unload()

	if _, err = GetPID(src); err != os.ErrProcessDone {
		if err = Stop(src, false, nil); err == nil {
			stopped = true
			defer func(c geneos.Instance) {
				if !done {
					Start(c, nil)
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

	_, ds, dr := SplitName(dstname, host.LOCAL)

	// do a dance here to deep copy-ish the dst
	realdst := dst
	b, _ := json.Marshal(src)
	if err = json.Unmarshal(b, &realdst); err != nil {
		logError.Println(err)
	}

	// after path updates, rename non paths
	// ib := realdst.Base()
	// ib.InstanceLocation = dst.Remote().String()
	// ib.InstanceRemote = dst.Remote()
	// ib.InstanceName = ds

	// move directory
	if err = host.CopyAll(src.Host(), src.Home(), dr, dst.Home()); err != nil {
		return
	}

	// delete one or the other, depending
	defer func(srcname string, srcrem *host.Host, srchome string, dst geneos.Instance) {
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
			dst.Host().RemoveAll(dst.Home())
		}
	}(src.String(), src.Host(), src.Home(), dst)

	// XXX update src here and then write that out as if it were dst
	// this gets around the defaults set in dst being incomplete (and hence wrong)
	// if err = changeDirPrefix(realdst, src.Remote().GeneosRoot(), dr.GeneosRoot()); err != nil {
	// 	logDebug.Println(err)
	// 	return
	// }

	// update *Home manually, as it's not just the prefix
	realdst.V().Set(dst.Prefix("Home"), filepath.Join(dst.Type().ComponentDir(dr), ds))
	// dst.Unload()

	// fetch a new port if remotes are different and port is already used
	if src.Host() != dr {
		srcport := src.V().GetInt64(src.Prefix("Port"))
		dstports := GetPorts(dr)
		if _, ok := dstports[int(srcport)]; ok {
			dstport := NextPort(dr, dst.Type())
			realdst.V().Set(dst.Prefix("Port"), fmt.Sprint(dstport))
		}
	}

	// update any component name only if the same as the instance name
	if src.V().GetString(src.Prefix("Name")) == srcname {
		realdst.V().Set(dst.Prefix("Name"), dstname)
	}

	// config changes don't matter until writing config succeeds
	if err = WriteConfig(realdst); err != nil {
		logDebug.Println(err)
		return
	}

	// src.Unload()
	if err = realdst.Rebuild(false); err != nil && err != geneos.ErrNotSupported {
		logDebug.Println(err)
		return
	}

	done = true
	if stopped {
		return Start(realdst, nil)
	}
	return nil
}
