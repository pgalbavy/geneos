package main

import (
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

const userSSHdir = ".ssh"

var privateKeyFiles = []string{
	"id_rsa",
	"id_ecdsa",
	"id_ecdsa_sk",
	"id_ed25519",
	"id_ed25519_sk",
	"id_dsa",
}

var signers []ssh.Signer
var khCallback ssh.HostKeyCallback
var agentClient agent.ExtendedAgent

// cache SSH connections
var remoteSSHClients = make(map[string]*ssh.Client)
var remoteSFTPClients = make(map[string]*sftp.Client)

// load all the known private keys with no passphrase
func readSSHkeys(homedir string) (signers []ssh.Signer) {
	for _, keyfile := range privateKeyFiles {
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

// this is not an init() func as we do late initialisation in case we
// don't need ssh
func sshInit() (err error) {
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
	if signers == nil {
		signers = readSSHkeys(homedir)
	}
	if agentClient == nil {
		socket := os.Getenv("SSH_AUTH_SOCK")
		sshAgent, err := net.Dial("unix", socket)
		if err != nil {
			log.Printf("Failed to open SSH_AUTH_SOCK: %v", err)
		} else {
			agentClient = agent.NewClient(sshAgent)
		}
	}
	return
}

func sshConnect(dest, user string) (client *ssh.Client, err error) {
	sshInit()
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(agentClient.Signers),
			ssh.PublicKeys(signers...),
		},
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
		i := remoteInstance(remote).(Instance)
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
