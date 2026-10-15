package main

import (
	"runtime/debug"
	"strings"
)

// version is stamped by release builds with -ldflags "-X main.version=1.2.3".
var version = "dev"

// buildVersion returns version, or the module version from debug.ReadBuildInfo when version is "dev".
func buildVersion() string {
	if version != "dev" {
		return normalizeVersion(version)
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	return normalizeVersion(info.Main.Version)
}

func normalizeVersion(v string) string {
	if v == "" || v == "(devel)" {
		return "dev"
	}
	if rest, ok := strings.CutPrefix(v, "v"); ok && rest != "" && rest[0] >= '0' && rest[0] <= '9' {
		return rest
	}
	return v
}
