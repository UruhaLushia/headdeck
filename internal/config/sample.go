package config

import "encoding/json"

func Effective(c *Config) (string, error) {
	b, err := json.MarshalIndent(Settings(c), "", "  ")
	return string(b) + "\n", err
}
