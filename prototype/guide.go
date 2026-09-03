package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// 引導式止血：認出這是哪個供應商的金鑰、給撤銷頁面與支出上限指引，使用者確認後
// 才開瀏覽器，使用者按下 Enter 後才複掃。不自動撤銷、不碰使用者的高權憑證、
// 不宣稱「已修復」——只回報「這串字還在不在暫存區」，金鑰本身有沒有真的失效
// 只有使用者去供應商網站才能確認。
var openBrowser = defaultOpenBrowser
var guideStdin io.Reader = os.Stdin

func runGuideCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "用法：leakbolt guide <指紋前綴>")
		return 2
	}
	repo, err := gitRepositoryRoot("")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return guideFingerprint(repo, args[0], stdout, stderr)
}

func guideFingerprint(repo, prefix string, stdout, stderr io.Writer) int {
	entry, err := lookupRecentFingerprint(repo, prefix)
	if err != nil {
		fmt.Fprintf(stderr, "找不到指紋：%v\n", err)
		return 2
	}

	provider, known := providerForRuleID(entry.RuleID)
	if !known {
		fmt.Fprintf(stdout, "規則 %s 目前沒有內建的處理指引。請自行到金鑰供應商的網站撤銷，並視情況設定支出上限。\n", entry.RuleID)
		return 0
	}

	fmt.Fprintf(stdout, "偵測到 %s 的金鑰（規則 %s｜指紋 %s）。建議動作：\n", provider.Name, entry.RuleID, shortFingerprint(entry.Fingerprint))
	fmt.Fprintf(stdout, "1. 撤銷或重新產生這把金鑰：%s\n", provider.RevokeURL)
	fmt.Fprintf(stdout, "2. %s\n", provider.SpendCapNote)
	fmt.Fprintln(stdout, "3. 從程式碼裡拿掉這個字串，不要只留在暫存區外")
	scanner := bufio.NewScanner(guideStdin)

	fmt.Fprintln(stdout)
	fmt.Fprint(stdout, "直接幫你開啟撤銷頁面？[y/N]：")
	if confirmYes(scanner) {
		if err := openBrowser(provider.RevokeURL); err != nil {
			fmt.Fprintf(stderr, "無法自動開啟瀏覽器（%v）；請自行前往：%s\n", err, provider.RevokeURL)
		}
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "撤銷、設好支出上限、拿掉字串之後，按 Enter 觸發複掃驗證（Ctrl+C 可略過）：")
	scanner.Scan()

	findings, err := scanStaged(repo, stderr)
	if err != nil {
		return reportScanError(stderr, err)
	}
	exitCode := finishStagedScan(repo, findings, stdout, stderr)

	state, err := loadState(repo)
	if err != nil {
		fmt.Fprintf(stderr, "複掃後讀取狀態失敗：%v\n", err)
		return exitCode
	}
	stillPresent := false
	for _, recent := range state.Recent {
		if recent.Fingerprint == entry.Fingerprint {
			stillPresent = true
			break
		}
	}

	fmt.Fprintln(stdout)
	if stillPresent {
		fmt.Fprintln(stdout, "複掃結果：這個字串還在暫存區裡，commit 會繼續被擋。撤銷金鑰跟拿掉字串是兩件事，兩個都要做。")
	} else {
		fmt.Fprintln(stdout, "複掃結果：這個字串已經不在暫存區了。但 LeakBolt 沒辦法確認金鑰本身有沒有真的被撤銷——那要你自己回剛剛開的頁面確認。")
	}
	return exitCode
}

func confirmYes(scanner *bufio.Scanner) bool {
	if !scanner.Scan() {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return answer == "y" || answer == "yes"
}

func defaultOpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
