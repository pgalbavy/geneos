package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

func setuid(cmd *exec.Cmd, username string) error {
	var gids []uint32

	if os.Geteuid() != 0 && os.Getuid() != 0 {
		return fmt.Errorf("not running as root")
	}

	u, err := user.Lookup(username)
	if err != nil {
		fmt.Println("lookup:", err)
		return err
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	groups, _ := u.GroupIds()
	for _, g := range groups {
		gid, _ := strconv.Atoi(g)
		gids = append(gids, uint32(gid))
	}

	cred := &syscall.Credential{
		Uid:         uint32(uid),
		Gid:         uint32(gid),
		Groups:      gids,
		NoSetGroups: false,
	}
	sys := &syscall.SysProcAttr{Credential: cred}

	cmd.SysProcAttr = sys

	return nil
}

// check if the current user can do "something" with the selected component
//
// just check if running as root or if a username is specified in the config
// that the current user matches.
//
// this does not however change the user to match anything, so starting a
// process still requires a seteuid type change
func cando(c Component) bool {
	if os.Geteuid() == 0 {
		return true
	}
	username := getString(c, Prefix(c)+"User")
	if len(username) == 0 {
		return true
	}
	u, _ := user.Current()

	return username == u.Username
}
