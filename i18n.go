package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// The web app's Paraglide messages (apps/web/messages), so the copy is
// the web's word for word, in both its languages.
//
//go:embed i18n/*.json
var messageFiles embed.FS

var messages = map[string]map[string]string{}

// locale is the language the window shows: "zh-CN" or "en".
var locale = "zh-CN"

func init() {
	for _, l := range []string{"zh-CN", "en"} {
		b, _ := messageFiles.ReadFile("i18n/" + l + ".json")
		m := map[string]string{}
		json.Unmarshal(b, &m)
		messages[l] = m
	}
}

// systemLocale is the desktop's language, as the browser's navigator.language.
func systemLocale() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			if strings.HasPrefix(v, "zh") {
				return "zh-CN"
			}
			return "en"
		}
	}
	return "zh-CN"
}

func setLocale(pref string) {
	switch pref {
	case "en", "zh-CN":
		locale = pref
	default:
		locale = systemLocale()
	}
}

// m returns the message key in the window's language, its {params} filled
// from pairs of name and value.
func m(key string, params ...any) string {
	s, ok := messages[locale][key]
	if !ok {
		s, ok = messages["en"][key]
	}
	if !ok {
		return key
	}
	for i := 0; i+1 < len(params); i += 2 {
		s = strings.ReplaceAll(s, "{"+fmt.Sprint(params[i])+"}", fmt.Sprint(params[i+1]))
	}
	return s
}
