package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-v" || args[0] == "version") {
		fmt.Fprintln(stdout, versionString())
		return 0
	}
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printUsage(stdout)
		return 0
	}

	switch args[0] {
	case "install":
		return runInstall(args[1:], stdout, stderr)
	case "uninstall":
		return runUninstall(args[1:], stdout, stderr)
	case "scan":
		return runScanCommand(args[1:], stdout, stderr)
	case "allow":
		return runAllow(args[1:], stdout, stderr)
	case "guide":
		return runGuideCommand(args[1:], stdout, stderr)
	case "untrack":
		return runUntrackCommand(args[1:], stdout, stderr)
	case "doctor":
		return runDoctorCommand(args[1:], stdout, stderr)
	case "purge-backups":
		return runPurgeBackupsCommand(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "未知子指令：%s\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "用法：")
	fmt.Fprintln(w, "  leakbolt install [--local-only | --global]")
	fmt.Fprintln(w, "  leakbolt uninstall --global")
	fmt.Fprintln(w, "  leakbolt scan --staged")
	fmt.Fprintln(w, "  leakbolt scan --history")
	fmt.Fprintln(w, "  leakbolt allow <指紋前綴>")
	fmt.Fprintln(w, "  leakbolt guide <指紋前綴>")
	fmt.Fprintln(w, "  leakbolt purge-backups")
	fmt.Fprintln(w, "  leakbolt untrack <檔案路徑>")
	fmt.Fprintln(w, "  leakbolt doctor")
}

func runInstall(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	localOnly := flags.Bool("local-only", false, "強制寫入 .git/hooks/pre-commit")
	global := flags.Bool("global", false, "保護這台機器上所有 repo")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		if err == nil {
			fmt.Fprintln(stderr, "install 不接受位置參數")
		}
		return 2
	}
	if *localOnly && *global {
		fmt.Fprintln(stderr, "install 的 --local-only 與 --global 不能同時使用")
		return 2
	}
	if *global {
		path, err := installGlobalHook()
		if err != nil {
			var occupied *HooksPathOccupiedError
			if errors.As(err, &occupied) {
				fmt.Fprintf(stderr, "安裝失敗：git config --global core.hooksPath 目前是 %q，已被其他工具使用；LeakBolt 不會覆寫。請自行處理衝突後重試。\n", occupied.Value)
				return 2
			}
			fmt.Fprintf(stderr, "安裝失敗：%v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "已安裝到 %s。\n", path)
		fmt.Fprintln(stdout, "這台機器上所有 repo（包含之後新建的）都會受 LeakBolt 保護。")
		fmt.Fprintln(stdout, "要解除：leakbolt uninstall --global")
		return 0
	}

	repo, err := gitRepositoryRoot("")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	result, err := installHook(repo, *localOnly)
	if err != nil {
		var occupied *HooksPathOccupiedError
		if errors.As(err, &occupied) {
			fmt.Fprintf(stderr, "警告：未安裝 LeakBolt。git config --%s core.hooksPath 已設為 %q；LeakBolt 不會覆寫既有 hook 路徑。\n", occupied.Scope, occupied.Value)
			return 0
		}
		fmt.Fprintf(stderr, "安裝失敗：%v\n", err)
		return 2
	}
	if err := recordInstallation(repo, result.Kind); err != nil {
		fmt.Fprintf(stderr, "安裝失敗：無法記錄本機狀態：%v\n", err)
		return 2
	}

	if result.VersionControlled {
		fmt.Fprintf(stdout, "已安裝到 %s。提醒：這個 hook 設定會進版控，隊友看得到。\n", result.Path)
	} else {
		fmt.Fprintf(stdout, "已安裝到 %s（只在本機生效）。\n", result.Path)
	}

	if path, ok := verifyHookReachable(repo); !ok {
		fmt.Fprintf(stderr, "警告：設定已寫入，但 git 實際會執行的 %s 不存在或不可執行。\n", path)
		switch {
		case strings.HasSuffix(result.Path, "lefthook.yml"), strings.HasSuffix(result.Path, "lefthook.yaml"):
			fmt.Fprintln(stderr, "  要先執行 lefthook install，這個設定才會生效。")
		case strings.HasSuffix(result.Path, ".pre-commit-config.yaml"):
			fmt.Fprintln(stderr, "  要先執行 pre-commit install，這個設定才會生效。")
		default:
			fmt.Fprintln(stderr, "  hook 目前不會被觸發，請檢查 core.hooksPath 與該目錄下的 pre-commit。")
		}
	}

	fmt.Fprintln(stdout, "開始掃描完整 git 歷史…")
	findings, filtered, err := scanHistory(repo, stderr)
	if err != nil {
		return reportScanError(stderr, err)
	}
	printHistorySummary(stdout, findings, filtered)
	// hook 已經裝好了，歷史掃描的結果是資訊而非安裝失敗。回非 0 會讓
	// .pkg 的 postinstall 誤判成「自動開啟全域保護失敗」，也會中斷使用者
	// 串接的安裝腳本。要單獨取得歷史掃描的 exit code 請用 leakbolt scan --history。
	if len(findings) > 0 {
		fmt.Fprintln(stdout, "hook 已安裝完成。上列為既有歷史中的命中，hook 只擋之後的 commit，這些要另外處理。")
	}
	return 0
}

func runUninstall(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	flags.SetOutput(stderr)
	global := flags.Bool("global", false, "解除全域保護")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || !*global {
		if err == nil {
			fmt.Fprintln(stderr, "用法：leakbolt uninstall --global")
		}
		return 2
	}

	dir, err := uninstallGlobalHook()
	if err != nil {
		var occupied *HooksPathOccupiedError
		if errors.As(err, &occupied) {
			if occupied.Value == "" {
				fmt.Fprintln(stderr, "解除失敗：git config --global core.hooksPath 未指向 LeakBolt；未變更任何設定。")
			} else {
				fmt.Fprintf(stderr, "解除失敗：git config --global core.hooksPath 目前是 %q，不是 LeakBolt 的目錄；未變更任何設定。\n", occupied.Value)
			}
			return 2
		}
		fmt.Fprintf(stderr, "解除失敗：%v\n", err)
		return 2
	}
	fmt.Fprintf(stdout, "已解除全域保護（%s 內的 hook 檔案仍保留）。\n", dir)
	return 0
}

func runScanCommand(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	flags.SetOutput(stderr)
	staged := flags.Bool("staged", false, "掃描暫存區")
	history := flags.Bool("history", false, "掃描完整 git 歷史")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || (*staged == *history) {
		if err == nil {
			fmt.Fprintln(stderr, "scan 必須且只能指定 --staged 或 --history")
		}
		return 2
	}

	repo, err := gitRepositoryRoot("")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	if *staged {
		findings, err := scanStaged(repo, stderr)
		if err != nil {
			return reportScanError(stderr, err)
		}
		return finishStagedScan(repo, findings, stdout, stderr)
	}

	findings, filtered, err := scanHistory(repo, stderr)
	if err != nil {
		return reportScanError(stderr, err)
	}
	printHistorySummary(stdout, findings, filtered)
	if len(findings) > 0 {
		return 1
	}
	return 0
}

func finishStagedScan(repo string, findings []Finding, stdout, stderr io.Writer) int {
	state, err := loadOrCreateState(repo)
	if err != nil {
		fmt.Fprintf(stderr, "讀取本機狀態失敗：%v\n", err)
		return 2
	}
	visible, err := prepareStagedFindings(state, findings)
	if err != nil {
		fmt.Fprintf(stderr, "處理掃描結果失敗：%v\n", err)
		return 2
	}
	if err := writeState(repo, state); err != nil {
		fmt.Fprintf(stderr, "寫入本機狀態失敗：%v\n", err)
		return 2
	}
	printStagedSummary(stdout, visible, len(findings))
	if len(visible) > 0 {
		notifyBlockedCommit(repo, visible, stdout)
		return 1
	}
	return 0
}

func runAllow(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "用法：leakbolt allow <指紋前綴>")
		return 2
	}
	repo, err := gitRepositoryRoot("")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	entry, alreadyAllowed, err := allowRecentFingerprint(repo, args[0])
	if err != nil {
		fmt.Fprintf(stderr, "標記誤報失敗：%v\n", err)
		return 2
	}
	if alreadyAllowed {
		fmt.Fprintf(stdout, "指紋 %s 已在 allowlist。\n", shortFingerprint(entry.Fingerprint))
		return 0
	}
	fmt.Fprintf(stdout, "已將指紋 %s（規則 %s）標記為誤報。\n", shortFingerprint(entry.Fingerprint), entry.RuleID)
	return 0
}

func runUntrackCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "用法：leakbolt untrack <檔案路徑>")
		return 2
	}
	repo, err := gitRepositoryRoot("")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return runUntrack(repo, args[0], stdout)
}

func runDoctorCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "doctor 不接受位置參數")
		return 2
	}
	repo, err := gitRepositoryRoot("")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return runDoctor(repo, stdout)
}

func runPurgeBackupsCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "purge-backups 不接受位置參數")
		return 2
	}
	repo, err := gitRepositoryRoot("")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return runPurgeBackups(repo, stdout)
}

func reportScanError(w io.Writer, err error) int {
	var missing *MissingGitleaksError
	if errors.As(err, &missing) {
		fmt.Fprintln(w, err)
		return 2
	}
	fmt.Fprintf(w, "掃描失敗：%v\n", err)
	return 2
}
