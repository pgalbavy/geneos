package main

import (
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

func init() {
	commands["logs"] = Command{commandLogs, parseArgs, "geneos logs [TYPE] [NAME...]",
		`Show logs for matching instances. Not fully implemented.`}
}

func commandLogs(ct ComponentType, args []string) error {
	return loopCommand(logsInstance, ct, args)
}

func logsInstance(c Instance) (err error) {
	logfile := filepath.Join(getString(c, Prefix(c)+"LogD"), getString(c, Prefix(c)+"LogF"))

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("modified file:", event.Name)
					copyFromFile()
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	if err = watcher.Add(logfile); err != nil {
		log.Fatal(err)
	}
	<-done

	return ErrNotSupported
}

func copyFromFile() {

}
