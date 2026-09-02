package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func recordInstallation(repo, hookKind string) error {
	state, err := loadOrCreateState(repo)
	if err != nil {
		return err
	}
	localValue, localSet, err := gitConfig(repo, "local")
	if err != nil {
		return err
	}
	globalValue, globalSet, err := gitConfig(repo, "global")
	if err != nil {
		return err
	}
	systemValue, systemSet, err := gitConfig(repo, "system")
	if err != nil {
		systemValue, systemSet = "", false
	}
	localDigest, err := hookPathDigest(state, "local", localValue, localSet)
	if err != nil {
		return err
	}
	globalDigest, err := hookPathDigest(state, "global", globalValue, globalSet)
	if err != nil {
		return err
	}
	systemDigest, err := hookPathDigest(state, "system", systemValue, systemSet)
	if err != nil {
		return err
	}
	state.Install = &InstallState{
		InstalledOn:           dateToday(),
		HookKind:              hookKind,
		LocalHooksPathDigest:  localDigest,
		GlobalHooksPathDigest: globalDigest,
		SystemHooksPathDigest: systemDigest,
	}
	return writeState(repo, state)
}

func runDoctor(repo string, stdout io.Writer) int {
	fmt.Fprintln(stdout, versionString())
	globalDir, globalErr := globalHooksDirectory()
	if globalErr != nil {
		fmt.Fprintf(stdout, "保護模式：異常（%v）\n", globalErr)
		return 2
	}
	globalValue, globalSet, configErr := gitConfig(repo, "global")
	if configErr != nil {
		fmt.Fprintf(stdout, "保護模式：異常（%v）\n", configErr)
		return 2
	}
	if globalSet && sameHooksPath(globalValue, globalDir) {
		return runGlobalDoctor(repo, globalDir, stdout)
	}
	fmt.Fprintln(stdout, "保護模式：單一 repo 模式")
	state, err := loadState(repo)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintln(stdout, "安裝記錄：異常（找不到 state.json）")
			fmt.Fprintln(stdout, "建議：leakbolt install")
			return 1
		}
		fmt.Fprintf(stdout, "健康檢查失敗：%v\n", err)
		return 2
	}
	if state.Install == nil {
		fmt.Fprintln(stdout, "安裝記錄：異常（尚未記錄安裝）")
		fmt.Fprintln(stdout, "建議：leakbolt install")
		return 1
	}

	ok := true
	for _, scope := range []string{"local", "global", "system"} {
		var want string
		switch scope {
		case "local":
			want = state.Install.LocalHooksPathDigest
		case "global":
			want = state.Install.GlobalHooksPathDigest
		case "system":
			want = state.Install.SystemHooksPathDigest
		}
		if scope == "system" && want == "" {
			fmt.Fprintln(stdout, "core.hooksPath (system)：未記錄（舊版安裝記錄，請重新執行 leakbolt install）")
			ok = false
			continue
		}

		value, set, configErr := gitConfig(repo, scope)
		if configErr != nil {
			if scope == "system" {
				value, set, configErr = "", false, nil
			} else {
				fmt.Fprintf(stdout, "core.hooksPath (%s)：異常（%v）\n", scope, configErr)
				ok = false
				continue
			}
		}
		digest, digestErr := hookPathDigest(state, scope, value, set)
		if digestErr != nil {
			fmt.Fprintf(stdout, "core.hooksPath (%s)：異常（%v）\n", scope, digestErr)
			ok = false
			continue
		}
		if digest == want {
			fmt.Fprintf(stdout, "core.hooksPath (%s)：正常\n", scope)
		} else {
			fmt.Fprintf(stdout, "core.hooksPath (%s)：異常（可能被其他工具改掉）\n", scope)
			ok = false
		}
	}

	if _, reachable := verifyHookReachable(repo); reachable {
		fmt.Fprintln(stdout, "hook 可達性：正常")
	} else {
		fmt.Fprintln(stdout, "hook 可達性：異常（git 找不到可執行的 pre-commit）")
		ok = false
	}

	if hookGuardPresent(repo, state.Install.HookKind) {
		fmt.Fprintln(stdout, "hook 守衛：正常")
	} else {
		fmt.Fprintln(stdout, "hook 守衛：異常（hook 可能已被覆寫）")
		ok = false
	}

	leakboltEnabled, _, configErr := gitConfigBool(repo, "hooks.leakbolt")
	switch {
	case configErr != nil:
		fmt.Fprintf(stdout, "LeakBolt 啟用狀態：異常（%v）\n", configErr)
		ok = false
	case leakboltEnabled == "false":
		fmt.Fprintln(stdout, "LeakBolt 啟用狀態：異常（已由 git config hooks.leakbolt false 停用，commit 不會被檢查）")
		fmt.Fprintln(stdout, "  要恢復檢查：git config --unset hooks.leakbolt")
		ok = false
	default:
		fmt.Fprintln(stdout, "LeakBolt 啟用狀態：正常")
	}

	if path, pathErr := exec.LookPath("leakbolt"); pathErr == nil {
		fmt.Fprintf(stdout, "leakbolt 執行檔：正常（%s）\n", path)
	} else {
		fmt.Fprintln(stdout, "leakbolt 執行檔：異常（PATH 上找不到 leakbolt，hook 會擋下所有 commit）")
		ok = false
	}

	_, version, versionErr := gitleaksPathAndVersion()
	var missingGitleaks *MissingGitleaksError
	switch {
	case errors.As(versionErr, &missingGitleaks):
		fmt.Fprintln(stdout, "gitleaks 版本：異常（找不到 gitleaks）")
		ok = false
	case versionErr != nil:
		fmt.Fprintf(stdout, "gitleaks 版本：異常（無法查詢版本：%v）\n", versionErr)
		ok = false
	case version != requiredGitleaksVersion:
		fmt.Fprintf(stdout, "gitleaks 版本：異常（找到 %s，預期 %s）\n", version, requiredGitleaksVersion)
		ok = false
	default:
		fmt.Fprintln(stdout, "gitleaks 版本：正常")
	}

	if !ok {
		fmt.Fprintln(stdout, "建議：leakbolt install")
		return 1
	}
	return 0
}

func runGlobalDoctor(repo, globalDir string, stdout io.Writer) int {
	fmt.Fprintln(stdout, "保護模式：全域模式")
	fmt.Fprintln(stdout, "core.hooksPath (global)：正常")
	ok := true
	hookPath := filepath.Join(globalDir, "pre-commit")

	// global 值正確不代表它生效：repo-local 與 worktree 層級的 core.hooksPath
	// 都會蓋過 global，讓這個 repo 靜默失去保護（husky init 就會這樣做）。
	// 必須比對 git 解析出來的有效值，且要指出是誰蓋掉的。
	for _, scope := range []string{"local", "worktree"} {
		value, set, err := gitConfig(repo, scope)
		if err != nil || !set {
			continue
		}
		if !sameHooksPath(filepath.Join(value, "pre-commit"), hookPath) &&
			!sameHooksPath(filepath.Join(repo, value, "pre-commit"), hookPath) {
			fmt.Fprintf(stdout, "core.hooksPath (%s)：異常（設為 %q，蓋過全域保護，這個 repo 的 commit 不會被檢查）\n", scope, value)
			fmt.Fprintf(stdout, "  這通常是 husky 之類的工具設的。要恢復保護：git config --unset --%s core.hooksPath，或改用 leakbolt install 讓 LeakBolt 與該工具共存。\n", scope)
			ok = false
		}
	}

	effectivePath, reachable := verifyHookReachable(repo)
	if reachable && sameHooksPath(effectivePath, hookPath) {
		fmt.Fprintln(stdout, "hook 可達性：正常")
	} else {
		fmt.Fprintf(stdout, "hook 可達性：異常（git 實際會執行的是 %s，不是全域的 LeakBolt hook）\n", effectivePath)
		ok = false
	}
	data, err := os.ReadFile(hookPath)
	if err == nil && strings.Contains(string(data), "leakbolt scan --staged") && strings.Contains(string(data), "repo_hook") {
		fmt.Fprintln(stdout, "hook 守衛：正常")
	} else {
		fmt.Fprintln(stdout, "hook 守衛：異常（hook 可能已被覆寫）")
		ok = false
	}

	leakboltEnabled, _, configErr := gitConfigBool(repo, "hooks.leakbolt")
	switch {
	case configErr != nil:
		fmt.Fprintf(stdout, "LeakBolt 啟用狀態：異常（%v）\n", configErr)
		ok = false
	case leakboltEnabled == "false":
		fmt.Fprintln(stdout, "LeakBolt 啟用狀態：異常（已由 git config hooks.leakbolt false 停用，commit 不會被檢查）")
		fmt.Fprintln(stdout, "  要恢復檢查：git config --unset hooks.leakbolt")
		ok = false
	default:
		fmt.Fprintln(stdout, "LeakBolt 啟用狀態：正常")
	}

	if path, pathErr := exec.LookPath("leakbolt"); pathErr == nil {
		fmt.Fprintf(stdout, "leakbolt 執行檔：正常（%s）\n", path)
	} else {
		fmt.Fprintln(stdout, "leakbolt 執行檔：異常（PATH 上找不到 leakbolt，hook 會擋下所有 commit）")
		ok = false
	}

	_, version, versionErr := gitleaksPathAndVersion()
	var missingGitleaks *MissingGitleaksError
	switch {
	case errors.As(versionErr, &missingGitleaks):
		fmt.Fprintln(stdout, "gitleaks 版本：異常（找不到 gitleaks）")
		ok = false
	case versionErr != nil:
		fmt.Fprintf(stdout, "gitleaks 版本：異常（無法查詢版本：%v）\n", versionErr)
		ok = false
	case version != requiredGitleaksVersion:
		fmt.Fprintf(stdout, "gitleaks 版本：異常（找到 %s，預期 %s）\n", version, requiredGitleaksVersion)
		ok = false
	default:
		fmt.Fprintln(stdout, "gitleaks 版本：正常")
	}

	if !ok {
		fmt.Fprintln(stdout, "建議：leakbolt install --global")
		return 1
	}
	return 0
}

func hookGuardPresent(repo, expectedKind string) bool {
	target, err := detectHookTarget(repo, false)
	if err != nil || target.Kind != expectedKind {
		return false
	}
	data, err := os.ReadFile(target.Path)
	return err == nil && strings.Contains(string(data), "leakbolt scan --staged")
}
