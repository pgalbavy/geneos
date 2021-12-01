package main

import (
	"fmt"
)

func init() {
	commands["create"] = commandCreate
}

func commandCreate(comp ComponentType, args []string) error {
	return fmt.Errorf("component creation net yet supported")
}
