package main

import (
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

func init() {
	commands["logs"] = Command{commandLogs, "logs"}
}

func commandLogs(comp ComponentType, args []string) error {
	return loopCommand(logs, comp, args)
}

func logs(c Component) (err error) {
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

	err = watcher.Add(logfile)
	if err != nil {
		log.Fatal(err)
	}
	<-done

	return ErrNotSupported
}

func copyFromFile() {

}
