package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

// 跟主程式同一份補充規則。corpusbench 是獨立的 main 套件，所以要自己內嵌一份。
// scripts/quality_gate.sh 會確認 rules/leakbolt.toml 與主程式規則一致，
// 避免兩份規則悄悄分岔。
//
//go:embed rules/leakbolt.toml
var supplementaryRules []byte

func writeSupplementaryRules() (string, func(), error) {
	dir, err := os.MkdirTemp("", "leakbolt-rules-")
	if err != nil {
		return "", func() {}, fmt.Errorf("建立暫存目錄失敗：%w", err)
	}
	cleanup := func() { os.RemoveAll(dir) }
	path := filepath.Join(dir, "leakbolt.toml")
	if err := os.WriteFile(path, supplementaryRules, 0o600); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("寫入 %s 失敗：%w", path, err)
	}
	return path, cleanup, nil
}
