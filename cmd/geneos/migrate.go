package main

import (
	"errors"
	"io/fs"
)

func init() {
	RegsiterCommand(Command{
		Name:          "migrate",
		Function:      commandMigrate,
		ParseFlags:    defaultFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   "geneos migrate [TYPE] [NAME...]",
		Summary:       `Migrate legacy .rc configuration to .json`,
		Description: `Migrate any legacy .rc configuration files to JSON format and rename the .rc file to
.rc.orig.`,
	})

	RegsiterCommand(Command{
		Name:          "revert",
		Function:      commandRevert,
		ParseFlags:    defaultFlag,
		ParseArgs:     parseArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine:   `geneos revert [TYPE] [NAME...]`,
		Summary:       `Revert migration of .rc files from backups.`,
		Description: `Revert migration of legacy .rc files to JSON ir the .rc.orig backup file still exists.
Any changes to the instance configuration since initial migration will be lost as the .rc file
is never written to.`,
	})

}

func commandMigrate(ct Component, names []string, params []string) (err error) {
	return ct.loopCommand(migrateInstance, names, params)
}

func migrateInstance(c Instances, params []string) (err error) {
	if err = migrateConfig(c); err != nil {
		log.Println(c.Type(), c.Name(), "cannot migrate configuration", err)
	}
	return
}

// migrate config from .rc to .json, but check first
func migrateConfig(c Instances) (err error) {
	// if no .rc, return
	if _, err = c.Remote().statFile(InstanceFileWithExt(c, "rc")); errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	// if .json exists, return
	if _, err = c.Remote().statFile(InstanceFileWithExt(c, "json")); err == nil {
		return nil
	}

	// write new .json
	if err = writeInstanceConfig(c); err != nil {
		logError.Println("failed to wrtite config file:", err)
		return
	}

	// back-up .rc
	if err = c.Remote().renameFile(InstanceFileWithExt(c, "rc"), InstanceFileWithExt(c, "rc.orig")); err != nil {
		logError.Println("failed to rename old config:", err)
	}

	logDebug.Printf("migrated %s to JSON config", c)
	return
}

func commandRevert(ct Component, names []string, params []string) (err error) {
	return ct.loopCommand(revertInstance, names, params)
}

func revertInstance(c Instances, params []string) (err error) {
	// if *.rc file exists, remove rc.orig+JSON, continue
	if _, err := c.Remote().statFile(InstanceFileWithExt(c, "rc")); err == nil {
		// ignore errors
		if c.Remote().removeFile(InstanceFileWithExt(c, "rc.orig")) == nil || c.Remote().removeFile(InstanceFileWithExt(c, "json")) == nil {
			logDebug.Println(c.Type(), c.Name(), "removed extra config file(s)")
		}
		return err
	}

	if err = c.Remote().renameFile(InstanceFileWithExt(c, "rc.orig"), InstanceFileWithExt(c, "rc")); err != nil {
		return
	}

	if err = c.Remote().removeFile(InstanceFileWithExt(c, "json")); err != nil {
		return
	}

	logDebug.Println(c.Type(), c.Name(), "reverted to RC config")
	return nil
}
