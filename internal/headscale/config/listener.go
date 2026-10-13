package config

import (
	"fmt"
	"net"
	"net/netip"
	"path/filepath"
	"strconv"

	"go.yaml.in/yaml/v3"
)

type listenerConfig struct {
	ListenAddr             string `yaml:"listen_addr"`
	TLSCertPath            string `yaml:"tls_cert_path"`
	TLSKeyPath             string `yaml:"tls_key_path"`
	TLSLetsEncryptHostname string `yaml:"tls_letsencrypt_hostname"`
}

func (c listenerConfig) localAddress() (string, error) {
	if c.TLSCertPath != "" || c.TLSKeyPath != "" || c.TLSLetsEncryptHostname != "" {
		return "", fmt.Errorf("terminate HTTPS at Headdeck or its reverse proxy; Headscale must use an internal HTTP listener")
	}
	address := c.ListenAddr
	if address == "" {
		address = "127.0.0.1:8080"
	}
	host, port, err := net.SplitHostPort(address)
	n, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("headscale listen_addr must be host:port")
	}
	switch host {
	case "", "0.0.0.0", "localhost":
		host = "127.0.0.1"
	case "::":
		host = "::1"
	default:
		ip, err := netip.ParseAddr(host)
		if err != nil || !ip.IsLoopback() {
			return "", fmt.Errorf("headscale listen_addr must use a loopback or wildcard address")
		}
	}
	return net.JoinHostPort(host, port), nil
}

// Upstream uses the local Headscale listener rather than a second public URL.
func Upstream(path, webListen string) (string, error) {
	if path == "" {
		path = DefaultPath
	}
	data, err := readFile(path)
	if err != nil {
		return "", err
	}
	var c listenerConfig
	if err := yaml.Unmarshal(data, &c); err != nil {
		return "", err
	}
	address, err := c.localAddress()
	if err != nil {
		return "", err
	}
	if webListen == "" {
		return "http://" + address, nil
	}
	_, port, _ := net.SplitHostPort(address)
	_, webPort, err := net.SplitHostPort(webListen)
	if err != nil || port == webPort {
		return "", fmt.Errorf("headdeck and Headscale require separate internal listening ports")
	}
	return "http://" + address, nil
}

func UnixSocket(path string) (string, error) {
	if path == "" {
		path = DefaultPath
	}
	data, err := readFile(path)
	if err != nil {
		return "", err
	}
	var c struct {
		Socket string `yaml:"unix_socket"`
	}
	if err := yaml.Unmarshal(data, &c); err != nil {
		return "", err
	}
	if c.Socket == "" {
		c.Socket = "/var/run/headscale/headscale.sock"
	}
	if !filepath.IsAbs(c.Socket) || filepath.Clean(c.Socket) != c.Socket {
		return "", fmt.Errorf("headscale unix_socket must be a clean absolute path")
	}
	return c.Socket, nil
}

// DiagnosticsUpstream resolves only local listeners, never a caller-supplied URL.
func DiagnosticsUpstream(path string) (string, error) {
	if path == "" {
		path = DefaultPath
	}
	data, err := readFile(path)
	if err != nil {
		return "", err
	}
	c := struct {
		Address string `yaml:"metrics_listen_addr"`
	}{Address: "127.0.0.1:9090"}
	if err := yaml.Unmarshal(data, &c); err != nil {
		return "", err
	}
	if c.Address == "" {
		return "", fmt.Errorf("headscale metrics listener is disabled")
	}
	address, err := (listenerConfig{ListenAddr: c.Address}).localAddress()
	if err != nil {
		return "", fmt.Errorf("invalid metrics_listen_addr: %w", err)
	}
	return "http://" + address, nil
}
