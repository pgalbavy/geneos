package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

var userSSHdir = ".ssh"

var privateKeyFiles = []string{
	"id_rsa",
	"id_ecdsa",
	"id_ecdsa_sk",
	"id_ed25519",
	"id_ed25519_sk",
	"id_dsa",
}

// ssh utilities for remote connections

func readSSHkeys(homedir string) (signers []ssh.Signer) {
	for _, keyfile := range privateKeyFiles {
		path := filepath.Join(homedir, ".ssh", keyfile)
		key, err := os.ReadFile(path)
		if err != nil {
			logDebug.Println(err)
			continue
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			logDebug.Println(err)
			continue
		}
		logDebug.Println("loaded private key from", path)
		signers = append(signers, signer)
	}
	return
}

func sshTest(username string, host string) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	sshAgent, err := net.Dial("unix", socket)
	if err != nil {
		log.Fatalf("Failed to open SSH_AUTH_SOCK: %v", err)
	}

	agentClient := agent.NewClient(sshAgent)

	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalln(err)
	}
	knownHostsFile := filepath.Join(homedir, ".ssh", "known_hosts")
	logDebug.Println(knownHostsFile)
	khcallback, err := knownhosts.New(knownHostsFile)
	if err != nil {
		log.Fatalln(err)
	}
	signers := readSSHkeys(homedir)
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(agentClient.Signers),
			ssh.PublicKeys(signers...),
		},
		HostKeyCallback: khcallback,
	}
	conn, err := ssh.Dial("tcp", host, config)
	if err != nil {
		log.Fatal("unable to connect: ", err)
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		log.Fatalln(err)
	}
	defer session.Close()

	var b bytes.Buffer
	session.Stdout = &b
	if err := session.Run("ls -l"); err != nil {
		log.Fatal("Failed to run: " + err.Error())
	}
	fmt.Println(b.String())

	client, err := sftp.NewClient(conn)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// walk a directory
	w := client.Walk("/home/pi")
	for w.Step() {
		if w.Err() != nil {
			continue
		}
		log.Println(w.Path())
	}
}
