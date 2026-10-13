package policy

import "strings"

func OwnerSetting(key string) bool { return strings.HasPrefix(key, "server.web.") }
