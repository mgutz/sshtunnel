// Package sshtunnel provides simple SSH tunneling
package sshtunnel

import (
	"fmt"
	"io"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Config is the configuration object for sshtunnel.
type Config struct {
	SSHAddress    string // SSH Server
	RemoteAddress string // Remote address on the SSH Server to tunnel
	LocalAddress  string // Local address to access tunnel
	SSHConfig     *ssh.ClientConfig
}

// SSHTunnel represents an SSH Tunnel connection.
type SSHTunnel struct {
	quit    chan bool
	pending chan error
	config  *Config
}

// New creates an instance of SSHTunnel
func New(config *Config) *SSHTunnel {
	return &SSHTunnel{
		quit:    make(chan bool),
		pending: make(chan error),
		config:  config,
	}
}

// Open initiates tunnel returning a pending channel.
//
// Example
//
//	if err := <-tunnel.Open(); err != nil {
//		// do work
//	}
func (st *SSHTunnel) Open() chan error {
	go st.createTunnel()
	return st.pending
}

func (st *SSHTunnel) createTunnel() {
	config := st.config

	// login to remote ssh server
	conn, err := ssh.Dial("tcp", config.SSHAddress, config.SSHConfig)
	if err != nil {
		st.pending <- err
		return
	}

	// local endpoint that will forward
	local, err := net.Listen("tcp", config.LocalAddress)
	if err != nil {
		st.pending <- err
		return
	}

	// ready
	st.pending <- nil

	for {
		l, err := local.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}

		go func() {
			remote, err := conn.Dial("tcp", config.RemoteAddress)
			if err != nil {
				fmt.Println(err)
				return
			}

			go iocopy(l, remote)
			go iocopy(remote, l)

			if <-st.quit {
				if remote != nil {
					remote.Close()
				}
				if local != nil {
					local.Close()
				}
				if conn != nil {
					conn.Close()
				}
				return
			}
		}()

	}
}

// SSHAgent uses ssh agent for authentication. To check `ssh-add -L`
func SSHAgent() ssh.AuthMethod {
	if sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		return ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	}
	return nil
}

// Close ends tunnel connection, by signaling gooroutine. This should always
// be called to cleanup.
func (st *SSHTunnel) Close() {
	st.quit <- true
}

func iocopy(writer, reader net.Conn) {
	_, err := io.Copy(writer, reader)
	if err != nil {
		fmt.Printf("io.Copy error: %s", err)
	}
}
