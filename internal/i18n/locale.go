package i18n

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Locale string

//go:embed zh-CN.json
var chineseJSON []byte

var chinese = func() map[string]string {
	var messages map[string]string
	if err := json.Unmarshal(chineseJSON, &messages); err != nil {
		panic(err)
	}
	return messages
}()

func Resolve(preference string) (Locale, error) {
	if preference != "" && preference != "auto" {
		if language := normalize(preference); language != "" {
			return language, nil
		}
		return "", fmt.Errorf("unsupported language %q; choose auto, en or zh-CN", preference)
	}
	for _, name := range []string{"HDKT_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := os.Getenv(name); value != "" {
			if name == "HDKT_LANG" && value == "auto" {
				continue
			}
			if language := normalize(value); language != "" {
				return language, nil
			}
			return "en", nil
		}
	}
	return "en", nil
}

func normalize(value string) Locale {
	value = strings.ToLower(strings.ReplaceAll(value, "_", "-"))
	value = strings.Split(strings.Split(value, ".")[0], "@")[0]
	switch {
	case value == "en" || strings.HasPrefix(value, "en-") || value == "c" || value == "posix":
		return "en"
	case value == "zh" || value == "zh-cn" || value == "zh-sg" || strings.HasPrefix(value, "zh-hans"):
		return "zh-CN"
	}
	return ""
}

func (l Locale) Text(message string, args ...any) string {
	if l == "zh-CN" {
		if translated, ok := chinese[message]; ok {
			message = translated
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(message, args...)
	}
	return message
}
