package app

import "cohub-mygo/internal/i18n"

// m is i18n.M, kept short for the views' hundreds of calls.
func m(key string, params ...any) string { return i18n.M(key, params...) }
