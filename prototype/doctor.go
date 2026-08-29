package main

import (
	"fmt"
	"io"
	"os"
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
	localDigest, err := hookPathDigest(state, "local", localValue, localSet)
	if err != nil {
		return err
	}
	globalDigest, err := hookPathDigest(state, "global", globalValue, globalSet)
	if err != nil {
		return err
	}
	state.Install = &InstallState{
		InstalledOn:           dateToday(),
		HookKind:              hookKind,
		LocalHooksPathDigest:  localDigest,
		GlobalHooksPathDigest: globalDigest,
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
	for _, scope := range []string{"local", "global"} {
		value, set, configErr := gitConfig(repo, scope)
		if configErr != nil {
			fmt.Fprintf(stdout, "core.hooksPath (%s)：異常（%v）\n", scope, configErr)
			ok = false
			continue
		}
		digest, digestErr := hookPathDigest(state, scope, value, set)
		if digestErr != nil {
			fmt.Fprintf(stdout, "core.hooksPath (%s)：異常（%v）\n", scope, digestErr)
			ok = false
			continue
		}
		want := state.Install.LocalHooksPathDigest
		if scope == "global" {
			want = state.Install.GlobalHooksPathDigest
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
