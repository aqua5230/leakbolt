# LeakBolt pre-commit hook 修復報告

## 修改內容

- `prototype/hooks.go:12-33`：把固定 `hookGuard` 改為依 `VersionControlled` 產生守衛。
  - 不進版控：PATH 找不到 `leakbolt` 時顯示中止訊息並回傳非 0。
  - 進版控：PATH 找不到 `leakbolt` 且 `<git-common-dir>/leakbolt/state.json` 存在時擋下；沒有 state 時安靜放行。
- `prototype/hooks.go:113-120,131-143,170-183,202-226`：把 `VersionControlled` 傳入 script、lefthook、pre-commit 三條寫入路徑。pre-commit 的多行守衛轉為合法單行 shell 指令。
- `prototype/doctor.go:8,123-128`：用 `exec.LookPath("leakbolt")` 顯示執行檔路徑或 PATH 異常，找不到時讓 doctor 回報不健康。
- `prototype/hooks_test.go:67-98`：更新 lefthook 與 pre-commit 既有測試，確認進版控守衛含 git common dir state 判斷。
- `prototype/hooks_test.go:227-287`：新增三個測試，以 `testGitRepo` 建暫存 repo、保留 `/usr/bin:/bin` 並真的用 `/bin/sh` 執行產出的 hook。
- `prototype/state_features_test.go:170,191,212`：配合 `writeScriptHook` 新參數，標示 `.git/hooks/pre-commit` 不進版控。

`prototype/implementation-notes.md` 無新增 Deviations：實作未偏離指定設計。

## 驗證輸出

沙盒不允許使用系統 Go 快取，因此驗證時設定 `GOCACHE=/tmp/leakbolt-go-cache`；命令本身與成功標準相同。

### `cd prototype && go build ./...`

```text
exit 0
(no output)
```

### `cd prototype && go vet ./...`

```text
exit 0
(no output)
```

### `cd prototype && go test ./...`

```text
exit 0
ok  	leakbolt	(cached)
?   	leakbolt/cmd/corpusbench	[no test files]
```

### `cd prototype && GOOS=windows GOARCH=amd64 go build ./...`

```text
exit 0
(no output)
```

### `cd prototype && GOOS=windows GOARCH=arm64 go build ./...`

```text
exit 0
(no output)
```

### 三個新增 hook 測試

```text
exit 0
PASS
ok  	leakbolt	0.496s
```

### `sh scripts/quality_gate.sh`

```text
exit 0
真陽性：19
假陰性：0
真陰性：21
假陽性：0
漏報率：0.00%
誤報率：0.00%
CORPUSBENCH_MISS_RATE=0.000000
CORPUSBENCH_FALSE_POSITIVE_RATE=0.000000
quality gate 通過：漏報率 0.000000，誤報率 0.000000
```

## 自我審查

1. 不進版控、進版控無 state、進版控有 state：三種情況各有測試，全部通過。
2. 三個測試都經 `runHookWithSh` 呼叫 `/bin/sh` 執行產出的 hook，不只比對字串。
3. 暫時移除不進版控守衛的 `else` 後，`TestUnversionedHookBlocksWhenLeakboltMissing` 如預期失敗：

```text
--- FAIL: TestUnversionedHookBlocksWhenLeakboltMissing (0.08s)
    hooks_test.go:235: leakbolt 不在 PATH 時，不進版控的 hook 應失敗
FAIL
exit status 1
FAIL	leakbolt	0.569s
```

`else` 已還原，完整測試隨後通過。

4. 未修改禁止範圍：兩份 `leakbolt.toml`、`prototype/corpus/`、`PLAN.md`、`DECISION.md`、`research/`、`scripts/` 均未觸碰。未執行 git 操作，未 commit。
