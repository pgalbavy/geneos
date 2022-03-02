package main

import (
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

var signers []ssh.Signer
var khCallback ssh.HostKeyCallback
var agentClient agent.ExtendedAgent

// cache SSH connections
var remoteSSHClients = make(map[string]*ssh.Client)
var remoteSFTPClients = make(map[string]*sftp.Client)

// load private keys, known hosts
func init() {
	homedir, err := os.UserHomeDir()
	if err != nil {
		logError.Fatalln(err)
	}

	k := filepath.Join(homedir, ".ssh", "known_hosts")
	khCallback, err = knownhosts.New(k)
	if err != nil {
		log.Println("cannot load ssh known_hosts file, ssh will not be available.", err)
		return
	}

	signers = readSSHkeys(homedir)

	socket := os.Getenv("SSH_AUTH_SOCK")
	sshAgent, err := net.Dial("unix", socket)
	if err != nil {
		log.Printf("Failed to open SSH_AUTH_SOCK: %v", err)
	} else {
		agentClient = agent.NewClient(sshAgent)
	}

}

// load all the known private keys with no passphrase
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

func sshConnect(dest, user string) (client *ssh.Client, err error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(agentClient.Signers),
			ssh.PublicKeys(signers...),
		},
		HostKeyCallback: khCallback,
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

func sftpOpenSession(remote string) (s *sftp.Client, err error) {
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

// func sshTest(username string, host string) {
// 	config := &ssh.ClientConfig{
// 		User: username,
// 		Auth: []ssh.AuthMethod{
// 			ssh.PublicKeysCallback(agentClient.Signers),
// 			ssh.PublicKeys(signers...),
// 		},
// 		HostKeyCallback: khCallback,
// 	}
// 	conn, err := ssh.Dial("tcp", host, config)
// 	if err != nil {
// 		log.Fatal("unable to connect: ", err)
// 	}
// 	defer conn.Close()

// 	session, err := conn.NewSession()
// 	if err != nil {
// 		logError.Fatalln(err)
// 	}
// 	defer session.Close()

// 	var b bytes.Buffer
// 	session.Stdout = &b
// 	if err := session.Run("ls -l"); err != nil {
// 		log.Fatal("Failed to run: " + err.Error())
// 	}
// 	fmt.Println(b.String())

// 	client, err := sftp.NewClient(conn)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer client.Close()

// 	// walk a directory
// 	w := client.Walk("/home/pi")
// 	for w.Step() {
// 		if w.Err() != nil {
// 			continue
// 		}
// 		log.Println(w.Path())
// 	}
// }
