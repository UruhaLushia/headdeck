package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func (c *Config) Check() []Check {
	checks := []Check{{Name: "config", Status: "ok"}}
	if err := c.Validate(); err != nil {
		checks[0].Status = "fail"
		checks[0].Message = err.Error()
	}

	if dir := filepath.Dir(c.IPC.Listen); dir != "." {
		info, err := os.Stat(dir)
		switch {
		case err != nil:
			checks = append(checks, Check{Name: "ipc.dir", Status: "fail", Message: err.Error()})
		case !info.IsDir():
			checks = append(checks, Check{
				Name: "ipc.dir", Status: "fail",
				Message: dir + " is not a directory",
			})
		default:
			checks = append(checks, Check{
				Name: "ipc.dir", Status: "ok",
				Message: fmt.Sprintf("%s mode %04o", dir, info.Mode().Perm()),
			})
		}
	}
	return checks
}

func ChecksFailed(checks []Check) bool {
	for _, c := range checks {
		if c.Status == "fail" {
			return true
		}
	}
	return false
}
