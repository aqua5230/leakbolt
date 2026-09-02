package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

// 補充規則內嵌進執行檔。
//
// 「單一可攜執行檔」是 PLAN.md 第六節拍板的技術基座，不能靠使用者機器上有某個 .toml。
// 規則內容與來源見 research/20-補充規則盤點.md。
//
//go:embed rules/leakbolt.toml
var supplementaryRules []byte

// writeSupplementaryRules 把內嵌規則寫到暫存檔，回傳路徑、清理函式與錯誤。
//
// 寫失敗即中止掃描，不退回 gitleaks 預設規則。原本的降級設計會讓只有補充規則
// 抓得到的金鑰（Groq、Supabase secret、Clerk secret 等）在暫存檔寫不出來時整批
// 放行，實測 TMPDIR 不可寫就會發生，且 commit 照樣成功——只有一行終端機警告。
// 掃描範圍縮小卻回報「找到 0 筆」，比明講掃不動更危險。
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
