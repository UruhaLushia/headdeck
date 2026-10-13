package config

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"go.yaml.in/yaml/v3"
	"golang.org/x/sys/unix"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
	"github.com/UruhaLushia/headdeck/internal/securefile"
)

const DefaultPath = "/etc/headscale/config.yaml"
const ManagedDir = "/var/lib/headdeck-headscale"

type document struct {
	root                      yaml.Node
	config                    protocol.DNSConfig
	serverURL, external, hash string
	files                     map[string][]byte
}

func child(n *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func readFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4<<20 {
		return nil, fmt.Errorf("invalid or oversized Headscale configuration")
	}
	return io.ReadAll(io.LimitReader(f, 4<<20))
}

func readDocument(path string) (*document, error) {
	data, err := readFile(path)
	if err != nil {
		return nil, err
	}
	d := &document{files: map[string][]byte{path: data}}
	if err := yaml.Unmarshal(data, &d.root); err != nil {
		return nil, fmt.Errorf("cannot parse Headscale configuration")
	}
	if len(d.root.Content) != 1 || d.root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("headscale configuration must be a mapping")
	}
	root := d.root.Content[0]
	n := child(root, "dns")
	if n == nil || n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("headscale configuration has no modern dns section")
	}
	if err := n.Decode(&d.config); err != nil {
		return nil, fmt.Errorf("cannot decode DNS configuration")
	}
	if n := child(root, "server_url"); n != nil {
		d.serverURL = n.Value
	}
	if n := child(n, "extra_records_path"); n != nil {
		d.external = n.Value
	}
	if d.external != "" {
		if !filepath.IsAbs(d.external) {
			return nil, fmt.Errorf("external DNS records require an absolute path")
		}
		data, err := readFile(d.external)
		if err != nil {
			return nil, fmt.Errorf("cannot read external DNS records: %w", err)
		}
		if err := yaml.Unmarshal(data, &d.config.ExtraRecords); err != nil {
			return nil, fmt.Errorf("cannot parse external DNS records")
		}
		d.files[d.external] = data
	}
	d.hash = fingerprint(d.files[path], d.files[d.external])
	normalizeDNS(&d.config)
	return d, nil
}

func normalizeDNS(c *protocol.DNSConfig) {
	if c.Nameservers.Global == nil {
		c.Nameservers.Global = []string{}
	}
	if c.Nameservers.Split == nil {
		c.Nameservers.Split = map[string][]string{}
	}
	if c.SearchDomains == nil {
		c.SearchDomains = []string{}
	}
	if c.ExtraRecords == nil {
		c.ExtraRecords = []protocol.DNSRecord{}
	}
}

func fingerprint(parts ...[]byte) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write(p)
		h.Write([]byte{0})
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func writable(path, managedDir string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	return err == nil && filepath.Dir(resolved) == managedDir && unix.Access(resolved, unix.W_OK) == nil && unix.Access(managedDir, unix.W_OK) == nil
}

func (s *Store) writeFile(path string, data []byte) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if filepath.Dir(resolved) != s.managedDir {
		return fmt.Errorf("refusing to write outside the managed configuration directory")
	}
	if err := securefile.Directory(s.managedDir); err != nil {
		return err
	}
	root, err := os.OpenRoot(s.managedDir)
	if err != nil {
		return err
	}
	defer root.Close()
	name := filepath.Base(resolved)
	info, err := root.Lstat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 != 0 || !securefile.Owned(info) {
		return fmt.Errorf("managed configuration must be a non-executable regular file with trusted ownership")
	}
	temporary := ".headdeck-" + rand.Text()
	f, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	defer f.Close()
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot preserve file ownership")
	}
	if err := f.Chown(int(stat.Uid), int(stat.Gid)); err != nil {
		return err
	}
	if err := f.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := root.Rename(temporary, name); err != nil {
		return err
	}
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (d *document) render(c protocol.DNSConfig) ([]byte, error) {
	n := child(d.root.Content[0], "dns")
	var updated yaml.Node
	if err := updated.Encode(c); err != nil {
		return nil, err
	}
	for i := 0; i < len(updated.Content); i += 2 {
		key, value := updated.Content[i], updated.Content[i+1]
		if d.external != "" && key.Value == "extra_records" {
			continue
		}
		if existing := child(n, key.Value); existing != nil {
			value.HeadComment, value.LineComment, value.FootComment = existing.HeadComment, existing.LineComment, existing.FootComment
			*existing = *value
		} else {
			n.Content = append(n.Content, key, value)
		}
	}
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&d.root); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
