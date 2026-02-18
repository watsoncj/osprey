package remote

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/browser-forensics/browser-forensics/internal/embedded"
	"github.com/browser-forensics/browser-forensics/internal/model"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Options configures a remote scan.
type Options struct {
	Host     string
	Port     int
	User     string
	KeyFile  string // path to private key (optional if agent is available)
	Password string // password auth fallback
	Hours    float64
}

const remoteAgentPath = `C:\Windows\Temp\bf-agent.exe`

// RunScan deploys the embedded agent to a remote Windows host over SSH,
// executes it, collects the JSON report, and cleans up.
func RunScan(ctx context.Context, opts Options) (model.RunReport, error) {
	client, err := dial(ctx, opts)
	if err != nil {
		return model.RunReport{}, fmt.Errorf("ssh connect: %w", err)
	}
	defer client.Close()

	log.Printf("Connected to %s", opts.Host)

	if err := deployAgent(client); err != nil {
		return model.RunReport{}, fmt.Errorf("deploy agent: %w", err)
	}
	defer removeAgent(client)

	log.Printf("Agent deployed to %s:%s", opts.Host, remoteAgentPath)

	report, err := executeAgent(ctx, client, opts.Hours)
	if err != nil {
		return model.RunReport{}, fmt.Errorf("execute agent: %w", err)
	}

	return report, nil
}

func dial(ctx context.Context, opts Options) (*ssh.Client, error) {
	var authMethods []ssh.AuthMethod

	// Try SSH agent first
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			authMethods = append(authMethods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
		}
	}

	// Try key file
	if opts.KeyFile != "" {
		key, err := os.ReadFile(opts.KeyFile)
		if err == nil {
			signer, err := ssh.ParsePrivateKey(key)
			if err == nil {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}
	}

	// Try default key locations
	if opts.KeyFile == "" {
		home, _ := os.UserHomeDir()
		for _, name := range []string{"id_rsa", "id_ed25519", "id_ecdsa"} {
			keyPath := filepath.Join(home, ".ssh", name)
			key, err := os.ReadFile(keyPath)
			if err != nil {
				continue
			}
			signer, err := ssh.ParsePrivateKey(key)
			if err != nil {
				continue
			}
			authMethods = append(authMethods, ssh.PublicKeys(signer))
		}
	}

	// Password fallback
	if opts.Password != "" {
		authMethods = append(authMethods, ssh.Password(opts.Password))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication methods available (tried SSH agent, key files, password)")
	}

	var hostKeyCallback ssh.HostKeyCallback
	knownHostsPath := filepath.Join(os.Getenv("HOME"), ".ssh", "known_hosts")
	if cb, err := knownhosts.New(knownHostsPath); err == nil {
		hostKeyCallback = cb
	} else {
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
		log.Printf("Warning: known_hosts not available, accepting any host key")
	}

	config := &ssh.ClientConfig{
		User:            opts.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         30 * time.Second,
	}

	port := opts.Port
	if port == 0 {
		port = 22
	}
	addr := fmt.Sprintf("%s:%d", opts.Host, port)

	dialer := net.Dialer{Timeout: config.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return ssh.NewClient(c, chans, reqs), nil
}

// deployAgent writes the embedded agent binary to the remote host using
// PowerShell base64 decoding over SSH. This avoids needing SFTP.
func deployAgent(client *ssh.Client) error {
	agentBytes := embedded.AgentEXE

	const chunkSize = 48000 // safe size for PowerShell command line

	for i := 0; i < len(agentBytes); i += chunkSize {
		end := i + chunkSize
		if end > len(agentBytes) {
			end = len(agentBytes)
		}
		chunk := agentBytes[i:end]
		b64 := base64.StdEncoding.EncodeToString(chunk)

		var cmd string
		if i == 0 {
			cmd = fmt.Sprintf(
				`powershell -Command "[IO.File]::WriteAllBytes('%s', [Convert]::FromBase64String('%s'))"`,
				remoteAgentPath, b64,
			)
		} else {
			cmd = fmt.Sprintf(
				`powershell -Command "$f = [IO.File]::OpenWrite('%s'); $f.Seek(0, [IO.SeekOrigin]::End) | Out-Null; $b = [Convert]::FromBase64String('%s'); $f.Write($b, 0, $b.Length); $f.Close()"`,
				remoteAgentPath, b64,
			)
		}

		session, err := client.NewSession()
		if err != nil {
			return fmt.Errorf("new session: %w", err)
		}

		var stderr bytes.Buffer
		session.Stderr = &stderr
		if err := session.Run(cmd); err != nil {
			session.Close()
			return fmt.Errorf("write chunk at offset %d: %w: %s", i, err, stderr.String())
		}
		session.Close()

		log.Printf("Deployed %d/%d bytes", end, len(agentBytes))
	}

	return nil
}

func removeAgent(client *ssh.Client) {
	session, err := client.NewSession()
	if err != nil {
		log.Printf("Warning: could not create session to clean up agent: %v", err)
		return
	}
	defer session.Close()

	cmd := fmt.Sprintf(`del /f "%s"`, remoteAgentPath)
	if err := session.Run(cmd); err != nil {
		log.Printf("Warning: could not remove remote agent: %v", err)
	} else {
		log.Printf("Cleaned up remote agent")
	}
}

func executeAgent(ctx context.Context, client *ssh.Client, hours float64) (model.RunReport, error) {
	session, err := client.NewSession()
	if err != nil {
		return model.RunReport{}, err
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	cmd := fmt.Sprintf(`"%s" -format json -hours %g`, remoteAgentPath, hours)
	log.Printf("Executing: %s", cmd)

	done := make(chan error, 1)
	go func() {
		done <- session.Run(cmd)
	}()

	select {
	case err := <-done:
		if err != nil {
			return model.RunReport{}, fmt.Errorf("agent execution failed: %w\nstderr: %s", err, stderr.String())
		}
	case <-ctx.Done():
		session.Signal(ssh.SIGTERM)
		return model.RunReport{}, ctx.Err()
	}

	stderrStr := strings.TrimSpace(stderr.String())
	if stderrStr != "" {
		for _, line := range strings.Split(stderrStr, "\n") {
			log.Printf("[remote] %s", line)
		}
	}

	var rr model.RunReport
	if err := json.NewDecoder(&stdout).Decode(&rr); err != nil {
		return model.RunReport{}, fmt.Errorf("decode report: %w\nraw output: %s", err, stdout.String())
	}

	return rr, nil
}
