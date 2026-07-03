package zfs

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHCollector implements Collector over SSH to a remote ZFS host.
type SSHCollector struct {
	cfg    CollectorConfig
	client *ssh.Client
}

// NewSSHCollector creates and connects an SSH-based ZFS collector.
func NewSSHCollector(cfg CollectorConfig) (*SSHCollector, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("ssh: host is required")
	}
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30
	}
	sc := &SSHCollector{cfg: cfg}
	if err := sc.connect(); err != nil {
		return nil, err
	}
	return sc, nil
}

func (sc *SSHCollector) connect() error {
	var auth []ssh.AuthMethod
	if sc.cfg.SSHKey != "" {
		var keyBytes []byte
		if _, err := os.Stat(sc.cfg.SSHKey); err == nil {
			keyBytes, err = os.ReadFile(sc.cfg.SSHKey)
			if err != nil {
				return fmt.Errorf("read ssh key: %w", err)
			}
		} else {
			keyBytes = []byte(sc.cfg.SSHKey)
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return fmt.Errorf("parse ssh key: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if sc.cfg.Password != "" {
		auth = append(auth, ssh.Password(sc.cfg.Password))
	}
	if len(auth) == 0 {
		return fmt.Errorf("ssh: no auth method (set ssh_key or password)")
	}
	sshCfg := &ssh.ClientConfig{
		User:            sc.cfg.Username,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         time.Duration(sc.cfg.Timeout) * time.Second,
	}
	addr := net.JoinHostPort(sc.cfg.Host, fmt.Sprintf("%d", sc.cfg.Port))
	client, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	sc.client = client
	return nil
}

func (sc *SSHCollector) exec(ctx context.Context, cmd string) ([]byte, error) {
	sess, err := sc.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}
	defer sess.Close()
	go func() {
		<-ctx.Done()
		sess.Close()
	}()
	out, err := sess.CombinedOutput(cmd)
	if err != nil {
		if len(out) == 0 {
			return nil, fmt.Errorf("exec %q: %w", cmd, err)
		}
		// non-zero exit but has output (e.g. degraded pool) — return output
	}
	return out, nil
}

func (sc *SSHCollector) CollectPools(ctx context.Context) ([]*Pool, error) {
	out, err := sc.exec(ctx, "zpool list -H -p -o name,health,size,allocated,free,capacity")
	if err != nil {
		return nil, err
	}
	return parsePoolList(out)
}

func (sc *SSHCollector) CollectDatasets(ctx context.Context, poolName string) ([]*Dataset, error) {
	out, err := sc.exec(ctx, fmt.Sprintf("zfs list -H -p -o name,used,available,referenced,mountpoint,type -r %s", poolName))
	if err != nil {
		return nil, err
	}
	return parseDatasetList(out)
}

func (sc *SSHCollector) CollectSnapshots(ctx context.Context, datasetName string) ([]*Snapshot, error) {
	out, err := sc.exec(ctx, fmt.Sprintf("zfs list -H -p -t snapshot -o name,used,referenced,creation -r %s", datasetName))
	if err != nil {
		if strings.Contains(err.Error(), "no datasets") {
			return nil, nil
		}
		return nil, err
	}
	return parseSnapshotList(out)
}

func (sc *SSHCollector) CollectScrubStatus(ctx context.Context, poolName string) (*Scrub, error) {
	out, err := sc.exec(ctx, fmt.Sprintf("zpool status -v %s", poolName))
	if err != nil {
		return nil, err
	}
	return parseScrubStatus(out), nil
}

func (sc *SSHCollector) CollectVdevTree(ctx context.Context, poolName string) (*Vdev, error) {
	out, err := sc.exec(ctx, fmt.Sprintf("zpool status -v %s", poolName))
	if err != nil {
		return nil, err
	}
	return parseVdevTree(out, poolName), nil
}

func (sc *SSHCollector) CollectSMARTData(ctx context.Context) (map[string]*SMARTData, error) {
	// Check smartctl availability
	if _, err := sc.exec(ctx, "which smartctl"); err != nil {
		return map[string]*SMARTData{}, nil
	}
	out, err := sc.exec(ctx, "lsblk -d -o NAME -n 2>/dev/null")
	if err != nil {
		return map[string]*SMARTData{}, nil
	}
	result := map[string]*SMARTData{}
	for _, line := range strings.Split(string(out), "\n") {
		dev := "/dev/" + strings.TrimSpace(line)
		if strings.TrimSpace(line) == "" {
			continue
		}
		smartOut, _ := sc.exec(ctx, fmt.Sprintf("smartctl -j -a %s 2>/dev/null", dev))
		if sd := parseSMARTJSON(smartOut, dev); sd != nil {
			result[dev] = sd
		}
	}
	return result, nil
}

func (sc *SSHCollector) CreateSnapshot(ctx context.Context, datasetName, snapshotName string) error {
	_, err := sc.exec(ctx, fmt.Sprintf("zfs snapshot %s@%s", datasetName, snapshotName))
	return err
}

func (sc *SSHCollector) DestroySnapshot(ctx context.Context, snapshotName string) error {
	_, err := sc.exec(ctx, "zfs destroy "+snapshotName)
	return err
}

func (sc *SSHCollector) StartScrub(ctx context.Context, poolName string) error {
	_, err := sc.exec(ctx, "zpool scrub "+poolName)
	return err
}

func (sc *SSHCollector) Close() error {
	if sc.client != nil {
		return sc.client.Close()
	}
	return nil
}
