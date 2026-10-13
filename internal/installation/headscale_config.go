package installation

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/UruhaLushia/headdeck/internal/config"
	hc "github.com/UruhaLushia/headdeck/internal/headscale/config"
	"github.com/UruhaLushia/headdeck/internal/paths"
	"github.com/UruhaLushia/headdeck/internal/securefile"
	"go.yaml.in/yaml/v3"
)

func AuthorizeHeadscaleConfig(ctx context.Context, cfg *config.Config) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("configuration authorization requires root")
	}
	if !config.ValidServiceUnit(cfg.Features.ServiceUnit) {
		return fmt.Errorf("invalid Headscale unit")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	conn, err := systemConnection(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	props, err := conn.GetUnitTypePropertiesContext(ctx, cfg.Features.ServiceUnit, "Service")
	if err != nil {
		return err
	}
	pid, _ := props["MainPID"].(uint32)
	if err := CheckHeadscaleConfigSource(pid); err != nil {
		return err
	}
	name, _ := props["Group"].(string)
	if name == "" {
		name, _ = props["User"].(string)
		if name == "" {
			name = "root"
		}
		account, err := user.Lookup(name)
		if err != nil {
			return err
		}
		name = account.Gid
	}
	group, err := user.LookupGroup(name)
	if err != nil {
		group, err = user.LookupGroupId(name)
	}
	if err != nil {
		return err
	}
	readerGID, err := strconv.Atoi(group.Gid)
	if err != nil {
		return err
	}
	uid, gid, err := serviceAccount()
	if err != nil {
		return err
	}
	if readerGID == 0 {
		readerGID = gid
	}
	info, err := os.Lstat(hc.DefaultPath)
	if err != nil {
		return err
	}
	if err := securefile.Directory(filepath.Dir(hc.DefaultPath)); err != nil {
		return err
	}
	alreadyManaged := info.Mode()&os.ModeSymlink != 0
	if alreadyManaged {
		target, err := filepath.EvalSymlinks(hc.DefaultPath)
		if err != nil {
			return err
		}
		if target != filepath.Join(hc.ManagedDir, "config.yaml") {
			return fmt.Errorf("configuration symlink is not managed by Headdeck")
		}
	} else if !securefile.Owned(info) || info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("initial Headscale configuration must be owned by root and not writable by other users")
	}
	data, err := os.ReadFile(hc.DefaultPath)
	if err != nil {
		return err
	}
	var document struct {
		Policy struct {
			Mode string `yaml:"mode"`
			Path string `yaml:"path"`
		} `yaml:"policy"`
		DNS struct {
			ExtraRecordsPath string `yaml:"extra_records_path"`
		} `yaml:"dns"`
	}
	if err := yaml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("invalid Headscale configuration")
	}
	if err := securefile.PrepareDirectory(hc.ManagedDir, 0o751); err != nil {
		return err
	}
	managedInfo, err := os.Lstat(hc.ManagedDir)
	if err != nil {
		return err
	}
	if !managedInfo.IsDir() || managedInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("managed configuration directory must not be a symlink")
	}
	if err := os.Chown(hc.ManagedDir, uid, gid); err != nil {
		return err
	}
	if err := os.Chmod(hc.ManagedDir, 0o751); err != nil {
		return err
	}
	if external := document.DNS.ExtraRecordsPath; external != "" && !alreadyManaged {
		if !filepath.IsAbs(external) || filepath.Clean(external) == hc.DefaultPath {
			return fmt.Errorf("invalid external DNS record path")
		}
		if err := authorizeConfigFile(external, "records"+filepath.Ext(external), uid, readerGID); err != nil {
			return err
		}
	}
	if err := authorizeConfigFile(hc.DefaultPath, "config.yaml", uid, readerGID); err != nil {
		return err
	}
	if document.Policy.Mode != "database" && document.Policy.Path != "" {
		if !filepath.IsAbs(document.Policy.Path) || filepath.Clean(document.Policy.Path) == hc.DefaultPath {
			return fmt.Errorf("invalid policy path")
		}
		if err := authorizeConfigFile(document.Policy.Path, "policy"+filepath.Ext(document.Policy.Path), uid, readerGID); err != nil {
			return err
		}
	}
	dir := filepath.Join(paths.SystemdUnitDir, paths.ServiceName+".d")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// This root-owned drop-in grants only the configured service's read group.
	f, err := os.OpenFile(filepath.Join(dir, "headscale-config.conf"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "[Service]\nSupplementaryGroups=%d\nReadWritePaths=%s -/var/lib/headscale\nCapabilityBoundingSet=CAP_DAC_OVERRIDE CAP_CHOWN\n", readerGID, hc.ManagedDir); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := conn.ReloadContext(ctx); err != nil {
		return err
	}
	result := make(chan string, 1)
	if _, err := conn.RestartUnitContext(ctx, paths.ServiceName, "fail", result); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case state := <-result:
		if state != "done" {
			return fmt.Errorf("headdeck restart finished with %s", state)
		}
		return nil
	}
}

func CheckHeadscaleConfigSource(pid uint32) error {
	if err := CheckHeadscaleArguments(pid); err != nil {
		return err
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return err
	}
	for _, entry := range strings.Split(string(data), "\x00") {
		if strings.HasPrefix(entry, "HEADSCALE_") {
			return fmt.Errorf("headscale environment configuration overrides are not yet supported")
		}
	}
	return nil
}

func CheckHeadscaleArguments(pid uint32) error {
	if pid == 0 {
		return fmt.Errorf("start Headscale before authorizing configuration management")
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return err
	}
	args := strings.Split(string(data), "\x00")
	for i, arg := range args {
		path := ""
		if arg == "--config" || arg == "-c" {
			if i+1 >= len(args) {
				return fmt.Errorf("invalid Headscale configuration argument")
			}
			path = args[i+1]
		}
		if strings.HasPrefix(arg, "--config=") {
			path = strings.TrimPrefix(arg, "--config=")
		}
		if path != "" && filepath.Clean(path) != hc.DefaultPath {
			return fmt.Errorf("custom Headscale configuration paths are not yet supported")
		}
	}
	return nil
}

func authorizeConfigFile(original, name string, uid, gid int) error {
	managed := filepath.Join(hc.ManagedDir, name)
	info, err := os.Lstat(original)
	if err != nil {
		return err
	}
	if err := securefile.Directory(filepath.Dir(original)); err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(original)
		if err != nil {
			return err
		}
		if target != managed {
			return fmt.Errorf("refusing to replace an existing configuration symlink")
		}
		return nil
	}
	if !info.Mode().IsRegular() || info.Size() > 4<<20 || !securefile.Owned(info) || info.Mode().Perm()&0o133 != 0 {
		return fmt.Errorf("invalid configuration file")
	}
	data, err := os.ReadFile(original)
	if err != nil {
		return err
	}
	backup, err := os.OpenFile(original+".headdeck-backup", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create configuration backup: %w", err)
	}
	if _, err := backup.Write(data); err != nil {
		backup.Close()
		return err
	}
	if err := backup.Close(); err != nil {
		return err
	}
	f, err := os.OpenFile(managed, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Chown(uid, gid); err != nil {
		return err
	}
	if err := f.Chmod(0o640); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	link := original + ".headdeck-link"
	if err := os.Symlink(managed, link); err != nil {
		return err
	}
	defer os.Remove(link)
	return os.Rename(link, original)
}
