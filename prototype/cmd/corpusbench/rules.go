package main

import (
	_ "embed"
	"os"
	"path/filepath"
)

// 跟主程式同一份補充規則。corpusbench 是獨立的 main 套件，所以要自己內嵌一份。
// rules/leakbolt.toml 由 scripts/quality_gate.sh 在跑之前從 prototype/rules/ 同步過來，
// 避免兩份規則悄悄分岔。
//
//go:embed rules/leakbolt.toml
var supplementaryRules []byte

func writeSupplementaryRules() (string, func(), bool) {
	dir, err := os.MkdirTemp("", "leakbolt-rules-")
	if err != nil {
		return "", func() {}, false
	}
	cleanup := func() { os.RemoveAll(dir) }
	path := filepath.Join(dir, "leakbolt.toml")
	if err := os.WriteFile(path, supplementaryRules, 0o600); err != nil {
		cleanup()
		return "", func() {}, false
	}
	return path, cleanup, true
}
