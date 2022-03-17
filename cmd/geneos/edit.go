package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func init() {
	commands["edit"] = Command{
		Function:      commandEdit,
		ParseFlags:    defaultFlag,
		ParseArgs:     defaultArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine: `geneos edit [global|user]
	geneos edit [TYPE] [NAME...]`,
		Summary: `Open an editor for instance configuration file.`,
		Description: `Open an editor for JSON configuration file(s). If the literal word 'global' or 'user' is
supplied then the respective non-instance specific configuration file is opened, otherwise one
or more configuration files are opened, depending on if TYPE and NAME(s) are supplied. The text
editor invoked will be the first set of the environment variables VISUAL or EDITOR or the linux
/usr/bin/editor alternative will be used. e.g.

	VISUAL=code geneos edit user

will open a VS Code editor window for the user configuration file.

See also 'geneos set' and 'geneos show'.`}
}

//
// run the configured editor against the instance chosen
//
func commandEdit(ct Component, args []string, params []string) (err error) {
	// default for no args is to edit user config
	if len(args) == 0 {
		args = []string{"user"}
	}

	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			// let the Linux alternatives system sort it out
			editor = "editor"
		}
	}

	// read the cofig into a struct then print it out again,
	// to sanitise the contents - or generate an error
	switch args[0] {
	case "global":
		editConfigFiles(editor, globalConfig)
		return
	case "user":
		userConfDir, _ := os.UserConfigDir()
		editConfigFiles(editor, filepath.Join(userConfDir, "geneos.json"))
		return
	}

	// instance config files ?
	if superuser {
		logError.Fatalln("no editing instance configs as root, for now")
	}

	// loop instances - parse the args again and load/print the config,
	// XXX allow for RC files again
	var cs []string
	for _, name := range args {
		for _, c := range ct.instanceMatches(name) {
			if c.Location() != LOCAL {
				logError.Fatalln(ErrNotSupported)
			}
			// this wil lfail if not migrated
			cs = append(cs, filepath.Join(c.Home(), c.Type().String()+".json"))
		}
	}
	if len(cs) > 0 {
		err = editConfigFiles(editor, cs...)
	}

	return
}

func editConfigFiles(editor string, files ...string) (err error) {
	cmd := exec.Command(editor, files...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	err = cmd.Run()

	// change file ownerships back here - but to who?
	return
}
