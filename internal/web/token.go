package web

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

func (s *Server) loadToken() (string, error) {
	if s.cfg.TokenFile == "" {
		return "", errors.New("server.web.token_file is not configured")
	}
	raw, err := os.ReadFile(s.cfg.TokenFile)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(raw))
	if len(token) < 16 {
		return "", fmt.Errorf("%s must hold at least 16 characters", s.cfg.TokenFile)
	}
	return token, nil
}
