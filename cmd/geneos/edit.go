package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func init() {
	commands["edit"] = Command{commandEdit, parseArgs,
		`geneos edit [global|user]
	geneos edit [TYPE] [NAME...]`,
		`Open a text editor for JSON configuration file(s). If the literal word 'global' or 'user' is
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
func commandEdit(ct ComponentType, args []string, params []string) (err error) {
	// default to combined global + user config
	// allow overrides to show specific or components
	if len(args) == 0 {
		// special case "config show" for resolved settings
		log.Println("not enough args")
		return
	}

	if superuser {
		log.Fatalln("no editing as root, for now")
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

	// loop instances - parse the args again and load/print the config,
	// but allow for RC files again
	var cs []string
	for _, name := range args {
		for _, c := range NewComponent(ct, name) {
			// try to migrate the config, which will not work if empty
			if err = loadConfig(c, true); err != nil {
				log.Println(Type(c), Name(c), "cannot load configuration, check syntax")
				// in case of error manually build path
				cs = append(cs, filepath.Join(RunningConfig.ITRSHome, ct.String(), ct.String()+"s", name, ct.String()+".json"))
			}
			if c != nil {
				cs = append(cs, filepath.Join(Home(c), Type(c).String()+".json"))
			}
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
	return
}
