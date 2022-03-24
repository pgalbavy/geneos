package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func init() {
	RegsiterCommand(Command{
		Name:          "edit",
		Function:      commandEdit,
		ParseFlags:    defaultFlag,
		ParseArgs:     defaultArgs,
		Wildcard:      true,
		ComponentOnly: false,
		CommandLine: `geneos edit [global|user]
	geneos edit [TYPE] [NAME...]`,
		Summary: `Open an editor for instance configuration file.`,
		Description: `Open an editor for JSON configuration file(s). If the literal 'global' or 'user' is
supplied then the respective configuration file is opened, otherwise one
or more configuration files are opened, depending on if TYPE and NAME(s) are supplied. The text
editor invoked will be the first set of the environment variables VISUAL or EDITOR or the linux
/usr/bin/editor alternative will be used. e.g.

	VISUAL=code geneos edit user

will open a VS Code editor window for the user configuration file.`,
	})

	RegsiterCommand(Command{
		Name:        "home",
		Function:    commandHome,
		ParseFlags:  defaultFlag,
		ParseArgs:   nil,
		CommandLine: "geneos home [TYPE] [NAME]",
		Summary:     `Output the home directory of the installation or the first matching instance`,
		Description: `Output the home directory of the first matching instance or local
installation or the remote on stdout. This is intended for scripting,
e.g.

	cd $(geneos home)
	cd $(geneos home gateway example1)
		
Because of the intended use no errors are logged and no other output.
An error in the examples above result in the user's home
directory being selected.`,
	})
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
			if c.Remote() != rLOCAL {
				logError.Fatalln(ErrNotSupported)
			}
			// this wil lfail if not migrated
			cs = append(cs, InstanceFile(c, ".json"))
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

func commandHome(ctunused Component, args []string, params []string) error {
	if len(args) == 0 {
		log.Println(Geneos())
		return nil
	}

	var ct Component
	// check if first arg is a type, if not set to None else pop first arg
	if ct = parseComponentName(args[0]); ct == Unknown {
		ct = None
	} else {
		args = args[1:]
	}

	var i []Instances
	if len(args) == 0 {
		i = ct.Instances(rLOCAL)
	} else {
		i = ct.instanceMatches(args[0])
	}

	if len(i) == 0 {
		log.Println(Geneos())
		return nil
	}

	if i[0].Type() == Remote {
		log.Println(getString(i[0], "Geneos"))
		return nil
	}

	log.Println(i[0].Home())
	return nil
}
