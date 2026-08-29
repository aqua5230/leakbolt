package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// runPurgeBackups 刪掉 untrack 留下的明文備份。
//
// 備份存的是原始檔案內容，也就是明文密鑰。留著是為了 untrack 失敗時能還原，
// 但它會一直累積，等於在使用者機器上建一個集中線索庫——這正是 PLAN.md 第四節缺口 5
// 要求「提供查看、刪除」的原因。使用者必須有辦法一次清乾淨。
func runPurgeBackups(repo string, stdout io.Writer) int {
	dir := filepath.Join(stateDirectory(repo), "backup")

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		fmt.Fprintln(stdout, "沒有備份可刪除。")
		return 0
	}
	if err != nil {
		fmt.Fprintf(stdout, "讀取備份目錄失敗：%v\n", err)
		return 2
	}
	if len(entries) == 0 {
		fmt.Fprintln(stdout, "沒有備份可刪除。")
		return 0
	}

	if err := os.RemoveAll(dir); err != nil {
		fmt.Fprintf(stdout, "刪除備份失敗：%v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "已刪除 %d 份備份。\n", len(entries))
	return 0
}
