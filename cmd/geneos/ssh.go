package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

const userSSHdir = ".ssh"

// cache SSH connections
var remoteSSHClients = make(map[string]*ssh.Client)
var remoteSFTPClients = make(map[string]*sftp.Client)

// load all the known private keys with no passphrase
func readSSHkeys(homedir string) (signers []ssh.Signer) {
	for _, keyfile := range strings.Split(GlobalConfig["PrivateKeys"], ",") {
		path := filepath.Join(homedir, userSSHdir, keyfile)
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

func sshConnect(dest, user string) (client *ssh.Client, err error) {
	var khCallback ssh.HostKeyCallback
	var authmethods []ssh.AuthMethod
	var signers []ssh.Signer
	var agentClient agent.ExtendedAgent
	var homedir string

	homedir, err = os.UserHomeDir()
	if err != nil {
		logError.Fatalln(err)
	}

	if khCallback == nil {
		k := filepath.Join(homedir, userSSHdir, "known_hosts")
		khCallback, err = knownhosts.New(k)
		if err != nil {
			logDebug.Println("cannot load ssh known_hosts file, ssh will not be available.")
			return
		}
	}

	if agentClient == nil {
		socket := os.Getenv("SSH_AUTH_SOCK")
		if socket != "" {
			sshAgent, err := net.Dial("unix", socket)
			if err != nil {
				log.Printf("Failed to open SSH_AUTH_SOCK: %v", err)
			} else {
				agentClient = agent.NewClient(sshAgent)
			}
		}
	}

	if signers == nil {
		signers = readSSHkeys(homedir)
	}

	if agentClient != nil {
		authmethods = append(authmethods, ssh.PublicKeysCallback(agentClient.Signers))
	}
	if signers == nil {
		authmethods = append(authmethods, ssh.PublicKeys(signers...))
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authmethods,
		HostKeyCallback: khCallback,
		Timeout:         5 * time.Second,
	}
	client, err = ssh.Dial("tcp", dest, config)
	if err != nil {
		logError.Fatalln("unable to connect:", err)
	}
	return
}

func sshOpenRemote(remote string) (client *ssh.Client, err error) {
	client, ok := remoteSSHClients[remote]
	if !ok {
		i := NewRemote(remote)
		if err = loadConfig(i, false); err != nil {
			logError.Fatalln(err)
		}
		dest := getString(i, "Hostname") + ":" + getIntAsString(i, "Port")
		user := getString(i, "Username")
		client, err = sshConnect(dest, user)
		if err != nil {
			logError.Fatalln(err)
		}
		logDebug.Println("remote opened", remote, dest, user)
		remoteSSHClients[remote] = client
	}
	return
}

func sshCloseRemote(remote string) {
	sftpCloseSession(remote)
	c, ok := remoteSSHClients[remote]
	if ok {
		c.Close()
		delete(remoteSSHClients, remote)
	}
}

// succeed or fatal
func sftpOpenSession(remote string) (s *sftp.Client) {
	s, ok := remoteSFTPClients[remote]
	if !ok {
		c, err := sshOpenRemote(remote)
		if err != nil {
			logError.Fatalln(err)
		}
		s, err = sftp.NewClient(c)
		if err != nil {
			logError.Fatalln(err)
		}
		logDebug.Println("remote opened", remote)
		remoteSFTPClients[remote] = s
	}
	return
}

func sftpCloseSession(remote string) {
	s, ok := remoteSFTPClients[remote]
	if ok {
		s.Close()
		delete(remoteSFTPClients, remote)
	}
}
