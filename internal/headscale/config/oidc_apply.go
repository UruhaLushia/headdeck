package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/UruhaLushia/headdeck/internal/config"
	"go.yaml.in/yaml/v3"
)

type OIDCChange struct {
	Before []byte `json:"before"`
	After  []byte `json:"after"`
}

func (s *Store) PlanOIDC(w config.WebConfig) (*OIDCChange, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !writable(s.path, s.managedDir) {
		return nil, fmt.Errorf("authorize Headscale configuration management first")
	}
	d, err := readDocument(s.path)
	if err != nil {
		return nil, err
	}
	var current struct {
		listenerConfig `yaml:",inline"`
		Database       struct {
			Type   string `yaml:"type"`
			SQLite struct {
				Path string `yaml:"path"`
			} `yaml:"sqlite"`
		} `yaml:"database"`
	}
	if err := d.root.Decode(&current); err != nil {
		return nil, err
	}
	if current.Database.Type != "sqlite" || current.Database.SQLite.Path != "/var/lib/headscale/db.sqlite" {
		return nil, fmt.Errorf("identity takeover requires the local /var/lib/headscale/db.sqlite database")
	}
	address, err := current.localAddress()
	if err != nil {
		return nil, err
	}
	wanted := map[string]any{
		"issuer": w.OIDCIssuer(), "client_id": w.OIDCServer.ClientID, "client_secret": w.OIDCServer.ClientSecret,
		"scope": []string{"openid", "profile", "email"}, "pkce": map[string]any{"enabled": true, "method": "S256"},
		"only_start_if_oidc_is_available": true, "use_expiry_from_token": false,
	}
	root := d.root.Content[0]
	for key, value := range map[string]string{"server_url": strings.TrimRight(w.PublicURL, "/"), "listen_addr": address} {
		if n := child(root, key); n != nil {
			n.Kind, n.Tag, n.Value = yaml.ScalarNode, "!!str", value
		} else {
			root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
		}
	}
	old := child(root, "oidc")
	if old != nil {
		if expiry := child(old, "expiry"); expiry != nil {
			wanted["expiry"] = expiry.Value
		}
	}
	var replacement yaml.Node
	if err := replacement.Encode(wanted); err != nil {
		return nil, err
	}
	if old != nil {
		*old = replacement
	} else {
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "oidc"}, &replacement)
	}
	after, err := renderDocument(d)
	if err != nil {
		return nil, err
	}
	return &OIDCChange{Before: d.files[s.path], After: after}, nil
}

func (s *Store) WriteOIDC(ctx context.Context, change OIDCChange, restore bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	from, to := change.Before, change.After
	if restore {
		from, to = to, from
	}
	d, err := readDocument(s.path)
	if err != nil {
		return err
	}
	if bytes.Equal(d.files[s.path], to) {
		return nil
	}
	if !bytes.Equal(d.files[s.path], from) {
		return fmt.Errorf("headscale configuration changed during login takeover; refusing to overwrite it")
	}
	if err := s.writeFile(s.path, to); err != nil {
		return err
	}
	updated, err := readDocument(s.path)
	if err != nil {
		return err
	}
	if err := s.advanceDrafts(ctx, d.hash, updated.hash); err != nil {
		return errors.Join(err, s.writeFile(s.path, from))
	}
	return nil
}
