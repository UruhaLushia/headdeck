package config

import (
	"net/url"
	"strings"
)

func validWebPath(u *url.URL) bool {
	if u.RawPath != "" {
		return false
	}
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return u.Path == "" || u.Path == "/"
	}
	if u.Path != "/"+path && u.Path != "/"+path+"/" {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == "" {
			return false
		}
		for _, c := range part {
			allowed := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_'
			if !allowed {
				return false
			}
		}
	}
	return true
}
