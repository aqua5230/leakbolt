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
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printUsage(stdout)
		return 0
	}

	switch args[0] {
	case "install":
		return runInstall(args[1:], stdout, stderr)
	case "scan":
		return runScanCommand(args[1:], stdout, stderr)
	case "allow":
		return runAllow(args[1:], stdout, stderr)
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
	fmt.Fprintln(w, "  leakbolt install [--local-only]")
	fmt.Fprintln(w, "  leakbolt scan --staged")
	fmt.Fprintln(w, "  leakbolt scan --history")
	fmt.Fprintln(w, "  leakbolt allow <指紋前綴>")
	fmt.Fprintln(w, "  leakbolt purge-backups")
	fmt.Fprintln(w, "  leakbolt untrack <檔案路徑>")
	fmt.Fprintln(w, "  leakbolt doctor")
}

func runInstall(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	localOnly := flags.Bool("local-only", false, "強制寫入 .git/hooks/pre-commit")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		if err == nil {
			fmt.Fprintln(stderr, "install 不接受位置參數")
		}
		return 2
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
	findings, err := scanHistory(repo, stderr)
	if err != nil {
		return reportScanError(stderr, err)
	}
	printHistorySummary(stdout, findings)
	if len(findings) > 0 {
		return 1
	}
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
			return 1
		}
		return 0
	}

	findings, err := scanHistory(repo, stderr)
	if err != nil {
		return reportScanError(stderr, err)
	}
	printHistorySummary(stdout, findings)
	if len(findings) > 0 {
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
