package main

import (
	"fmt"
	"runtime/debug"
)

const version = "0.1.0"

func versionString() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return fmt.Sprintf("leakbolt %s，鎖定 gitleaks %s", version, requiredGitleaksVersion)
	}

	var revision string
	dirty := false
	for _, setting := range buildInfo.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}
	if revision == "" {
		return fmt.Sprintf("leakbolt %s，鎖定 gitleaks %s", version, requiredGitleaksVersion)
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if dirty {
		revision += "-dirty"
	}
	return fmt.Sprintf("leakbolt %s (%s)，鎖定 gitleaks %s", version, revision, requiredGitleaksVersion)
}
