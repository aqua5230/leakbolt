package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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

func hookGuardPresent(repo, expectedKind string) bool {
	target, err := detectHookTarget(repo, false)
	if err != nil || target.Kind != expectedKind {
		return false
	}
	data, err := os.ReadFile(target.Path)
	return err == nil && strings.Contains(string(data), "leakbolt scan --staged")
}
