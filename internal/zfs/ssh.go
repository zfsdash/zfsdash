package zfs

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHConfig holds SSH connection parameters.
type SSHConfig struct {
	Host          string
	Port          int
	User          string
	Password      string
	PrivateKeyPEM string
	Timeout       time.Duration
}

// SSHCollector collects ZFS data from a remote host over SSH.
type SSHCollector struct {
	client *ssh.Client
	host   string
}

// NewSSHCollector dials the remote host and returns an SSHCollector.
func NewSSHCollector(cfg SSHConfig) (*SSHCollector, error) {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	var auth []ssh.AuthMethod
	if cfg.PrivateKeyPEM != "" {
		signer, err := ssh.ParsePrivateKey([]byte(cfg.PrivateKeyPEM))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if cfg.Password != "" {
		auth = append(auth, ssh.Password(cfg.Password))
	}
	if len(auth) == 0 {
		return nil, fmt.Errorf("no SSH auth method provided (need password or private_key_pem)")
	}
	sshCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         cfg.Timeout,
	}
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	client, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	return &SSHCollector{client: client, host: cfg.Host}, nil
}

func (s *SSHCollector) run(ctx context.Context, cmd string) (string, error) {
	sess, err := s.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()
	var stdout, stderr bytes.Buffer
	sess.Stdout = &stdout
	sess.Stderr = &stderr
	if err := sess.Run(cmd); err != nil {
		return "", fmt.Errorf("run %q: %w: %s", cmd, err, stderr.String())
	}
	return stdout.String(), nil
}

func (s *SSHCollector) GetPools(ctx context.Context) ([]*Pool, error) {
	out, err := s.run(ctx, "zpool list -H -p -o name,size,allocated,free,fragmentation,capacity,health")
	if err != nil {
		return nil, err
	}
	pools, err := ParsePoolList(out)
	if err != nil {
		return nil, err
	}
	for _, pool := range pools {
		if sOut, serr := s.run(ctx, "zpool status "+pool.Name); serr == nil {
			ParseZpoolStatus(sOut, pool)
		}
	}
	return pools, nil
}

func (s *SSHCollector) GetDatasets(ctx context.Context, pool string) ([]*Dataset, error) {
	cmd := "zfs list -H -p -t filesystem,volume -o name,type,used,avail,refer,logicalused,mounted,mountpoint,compression,ratio,dedup,quota,reservation,volsize,encryption"
	if pool != "" {
		cmd += " " + pool
	}
	out, err := s.run(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return ParseDatasetList(out)
}

func (s *SSHCollector) GetSnapshots(ctx context.Context, dataset string) ([]*Snapshot, error) {
	cmd := "zfs list -H -p -t snapshot -o name,used,refer"
	if dataset != "" {
		cmd += " " + dataset
	}
	out, err := s.run(ctx, cmd)
	if err != nil {
		if strings.Contains(err.Error(), "no datasets available") {
			return nil, nil
		}
		return nil, err
	}
	return ParseSnapshotList(out)
}

func (s *SSHCollector) GetSMARTData(ctx context.Context) ([]*SMARTData, error) {
	out, err := s.run(ctx, "smartctl --scan 2>/dev/null")
	if err != nil {
		return nil, nil
	}
	var results []*SMARTData
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		smartOut, serr := s.run(ctx, "smartctl -A -H -i "+fields[0]+" 2>/dev/null")
		if serr == nil {
			if sd := parseSMARTOutput(fields[0], smartOut); sd != nil {
				results = append(results, sd)
			}
		}
	}
	return results, nil
}

func (s *SSHCollector) CreateSnapshot(ctx context.Context, dataset, snapName string) error {
	_, err := s.run(ctx, "zfs snapshot "+dataset+"@"+snapName)
	return err
}

func (s *SSHCollector) DeleteSnapshot(ctx context.Context, fullName string) error {
	_, err := s.run(ctx, "zfs destroy "+fullName)
	return err
}

func (s *SSHCollector) StartScrub(ctx context.Context, pool string) error {
	_, err := s.run(ctx, "zpool scrub "+pool)
	return err
}

func (s *SSHCollector) StopScrub(ctx context.Context, pool string) error {
	_, err := s.run(ctx, "zpool scrub -s "+pool)
	return err
}

func (s *SSHCollector) Close() error {
	return s.client.Close()
}
