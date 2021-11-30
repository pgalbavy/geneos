package main

import (
	"fmt"
)

// generic action commands

func Create(c Component) error {
	// create a directory and a default config file

	return fmt.Errorf("component creation net yet supported")

	/* switch Type(c) {
	case Gateway:

	default:
		// wildcard, create an environment (later)
		return fmt.Errorf("wildcard creation net yet supported")
	}

	err := os.MkdirAll(Home(c), 0775)
	if err != nil {
		return err
	}

	// update settings here, then write
	WriteJSONConfig(c)
	return nil */
}
