package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func init() {
	commands["edit"] = Command{commandEdit, parseArgs, "edit"}
}

//
// run the configured editor against the instance chosen
//
func commandEdit(ct ComponentType, args []string) (err error) {
	// default to combined global + user config
	// allow overrides to show specific or components
	if len(args) == 0 {
		// special case "config show" for resolved settings
		log.Println("not enough args")
		return
	}

	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			log.Println("VISUAL or EDITOR must be defined")
			return
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
		for _, c := range New(ct, name) {
			if err = loadConfig(c, true); err != nil {
				log.Println(Type(c), Name(c), "cannot load configuration")
				continue
			}
			if c != nil {
				cs = append(cs, filepath.Join(Home(c), Type(c).String()+".json"))
			}
		}
	}
	if len(cs) > 0 {
		editConfigFiles(editor, cs...)
	}

	return
}

func editConfigFiles(editor string, files ...string) (err error) {
	cmd := exec.Command(editor, files...)
	err = cmd.Run()
	return
}
