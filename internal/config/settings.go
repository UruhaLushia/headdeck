package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Setting struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	Secret     bool   `json:"secret,omitempty"`
	Configured bool   `json:"configured,omitempty"`
}

func SettingsChanged(next, current *Config) bool {
	return !reflect.DeepEqual(values(next), values(current))
}

func fields(c *Config) map[string]reflect.Value {
	out := make(map[string]reflect.Value)
	collectFields(reflect.ValueOf(c).Elem(), "", out)
	return out
}

func collectFields(v reflect.Value, prefix string, out map[string]reflect.Value) {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		name := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]
		if prefix == "" && name == "ipc" {
			continue
		}
		key := prefix + name
		value := v.Field(i)
		if value.Kind() == reflect.Struct {
			collectFields(value, key+".", out)
		} else {
			out[key] = value
		}
	}
}

func values(c *Config) map[string]string {
	out := make(map[string]string)
	for key, v := range fields(c) {
		switch {
		case v.Type() == reflect.TypeFor[time.Duration]():
			out[key] = time.Duration(v.Int()).String()
		case v.Kind() == reflect.Slice:
			b, _ := json.Marshal(v.Interface())
			out[key] = string(b)
		default:
			out[key] = fmt.Sprint(v.Interface())
		}
	}
	return out
}

func Settings(c *Config) []Setting {
	out := make([]Setting, 0)
	for key, value := range values(c) {
		if ProtectedSetting(key) {
			continue
		}
		secret := SecretSetting(key)
		entry := Setting{Key: key, Value: value, Secret: secret, Configured: value != ""}
		if secret {
			entry.Value = ""
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func ProtectedSetting(key string) bool {
	switch key {
	case "features.service_unit", "server.web.headscale_proxy", "server.web.token_file", "server.web.session_key_file",
		"server.web.tls_cert", "server.web.tls_key", "server.web.oidc_server.signing_key_file":
		return true
	}
	return strings.HasPrefix(key, "ipc.") || strings.HasPrefix(key, "headscale.") || strings.HasPrefix(key, "server.web.oidc_server.")
}

func set(c *Config, key, raw string) error {
	v, ok := fields(c)[key]
	if !ok {
		return fmt.Errorf("unknown or protected setting %q", key)
	}
	var err error
	switch {
	case v.Type() == reflect.TypeFor[time.Duration]():
		var n time.Duration
		n, err = time.ParseDuration(raw)
		if err == nil {
			v.SetInt(int64(n))
		}
	case v.Kind() == reflect.String:
		v.SetString(raw)
	case v.Kind() == reflect.Bool:
		var b bool
		b, err = strconv.ParseBool(raw)
		if err == nil {
			v.SetBool(b)
		}
	case v.Kind() == reflect.Int:
		var n int64
		n, err = strconv.ParseInt(raw, 10, 64)
		if err == nil {
			v.SetInt(n)
		}
	case v.Kind() == reflect.Slice:
		var list []string
		err = json.Unmarshal([]byte(raw), &list)
		if err == nil {
			v.Set(reflect.ValueOf(list))
		}
	}
	if err != nil {
		return fmt.Errorf("invalid value for %s", key)
	}
	return nil
}
