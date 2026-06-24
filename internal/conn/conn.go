package conn

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Config holds parameters for establishing an SSH connection
type Config struct {
	Host           string
	Port           int
	User           string
	IdentityFiles  []string
	Password       string
	KnownHostsFile string
	Timeout        time.Duration

	// the path to the sftp-server binary on the remote
	SFTPServerPath string // empty string means use the ssh subsystem "sftp"
}

// Conn is a live SSH session with its stdin/stdout wired to an sftp-server
type Conn struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
}

// Dial opens an SSH connection and starts an sftp-server session
func Dial(ctx context.Context, cfg Config) (*Conn, error) {
	authMethods, err := buildAuthMethods(cfg)
	if err != nil {
		return nil, fmt.Errorf("build auth: %w", err)
	}

	hostKeyCallback, err := buildHostKeyCallback(cfg.KnownHostsFile)
	if err != nil {
		return nil, fmt.Errorf("known hosts: %w", err)
	}

	sshCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         cfg.Timeout,
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	dialer := &net.Dialer{}
	netConn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("tcp dial %s: %w", addr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(netConn, addr, sshCfg)
	if err != nil {
		netConn.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("new session: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if cfg.SFTPServerPath != "" {
		// runs a command on the remote and logs the errors to stderr instead of syslog
		if err := session.Start(cfg.SFTPServerPath + " -e"); err != nil {
			session.Close()
			client.Close()
			return nil, fmt.Errorf("start sftp-server: %w", err)
		}
	} else {
		// it sends an SSH "subsystem" request named sftp in sshd_config
		if err := session.RequestSubsystem("sftp"); err != nil {
			session.Close()
			client.Close()
			return nil, fmt.Errorf("sftp subsystem: %w", err)
		}
	}

	return &Conn{
		client:  client,
		session: session,
		stdin:   stdin,
		stdout:  stdout,
	}, nil
}

// ReadWriter returns the IO pair connected to the remote sftp-server
func (c *Conn) ReadWriter() (io.Reader, io.WriteCloser) {
	return c.stdout, c.stdin
}

// Close tears down the SSH
func (c *Conn) Close() error {
	_ = c.stdin.Close()
	_ = c.session.Close()
	return c.client.Close()
}

func buildAuthMethods(cfg Config) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	// prefer ssh-agent if available: keys stay in the agent and signing is
	// delegated to it, so passphrase-protected keys work without prompting
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
		}
	}

	// look for the keys
	keyFiles := cfg.IdentityFiles
	if len(keyFiles) == 0 {
		home, err := os.UserHomeDir()
		if err == nil {
			for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
				keyFiles = append(keyFiles, filepath.Join(home, ".ssh", name))
			}
		}
	}
	for _, path := range keyFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			continue
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	// password method
	if cfg.Password != "" {
		methods = append(methods, ssh.Password(cfg.Password))
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("no authentication methods available")
	}
	return methods, nil
}

func buildHostKeyCallback(knownHostsFile string) (ssh.HostKeyCallback, error) {
	if knownHostsFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ssh.InsecureIgnoreHostKey(), nil //nolint:gosec
		}
		knownHostsFile = filepath.Join(home, ".ssh", "known_hosts")
	}
	if _, err := os.Stat(knownHostsFile); os.IsNotExist(err) {
		return ssh.InsecureIgnoreHostKey(), nil //nolint:gosec
	}
	return knownhosts.New(knownHostsFile)
}
