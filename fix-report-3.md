# LeakBolt 修復報告 3

日期：2026-08-29

## 結果

- 新增 repo 專用停用開關：`git config hooks.leakbolt false`。
- hook 先檢查停用狀態，再找 `leakbolt` 執行檔；未設定仍預設啟用。
- 找不到執行檔時印出字面的 `PATH=$PATH` 與永久停用指令。
- doctor 在執行檔項目前回報 LeakBolt 啟用狀態；停用時回 1。
- 新增 `gitConfigBool`，保留既有 `gitConfig` 簽名。
- 新增四個指定測試，全部用 `/bin/sh` 執行產出的 hook 腳本。

## 變更檔案

- `prototype/hooks.go`
- `prototype/doctor.go`
- `prototype/git.go`
- `prototype/hooks_test.go`
- `prototype/state_features_test.go`
- `fix-report-3.md`

`prototype/implementation-notes.md` 已檢查；本次沒有被迫偏離規格，`Deviations` 無新增項目。

## 驗收輸出

沙盒不允許寫入預設 Go 快取，因此各 Go 命令只額外指定可寫的 `GOCACHE=/private/tmp/...`；建置與測試內容不變。

### `cd prototype && go build ./...`

```text
(無輸出)
exit 0
```

### `cd prototype && go vet ./...`

```text
(無輸出)
exit 0
```

### `cd prototype && go test ./...`

```text
ok  	leakbolt	2.950s
?   	leakbolt/cmd/corpusbench	[no test files]
exit 0
```

### `cd prototype && GOOS=windows GOARCH=amd64 go build ./...`

```text
(無輸出)
exit 0
```

### `cd prototype && GOOS=windows GOARCH=arm64 go build ./...`

```text
(無輸出)
exit 0
```

### 四個新測試

```text
=== RUN   TestUnversionedHookAllowsDisabledLeakboltWhenBinaryMissing
--- PASS: TestUnversionedHookAllowsDisabledLeakboltWhenBinaryMissing (0.07s)
=== RUN   TestVersionControlledHookAllowsDisabledLeakboltWhenBinaryMissing
--- PASS: TestVersionControlledHookAllowsDisabledLeakboltWhenBinaryMissing (0.09s)
=== RUN   TestUnversionedHookMissingLeakboltReportsPATH
--- PASS: TestUnversionedHookMissingLeakboltReportsPATH (0.07s)
=== RUN   TestDoctorReportsLeakboltDisabled
--- PASS: TestDoctorReportsLeakboltDisabled (0.26s)
PASS
ok  	leakbolt	1.099s
?   	leakbolt/cmd/corpusbench	[no test files]
exit 0
```

### `sh scripts/quality_gate.sh`

```text
真陽性：19
假陰性：0
真陰性：21
假陽性：0
漏報率：0.00%
誤報率：0.00%
CORPUSBENCH_MISS_RATE=0.000000
CORPUSBENCH_FALSE_POSITIVE_RATE=0.000000
quality gate 通過：漏報率 0.000000，誤報率 0.000000
exit 0
```

## 自我審查

1. **停用檢查順序：通過。** 停用條件在 `command -v leakbolt` 前。暫時對調兩段後，`TestUnversionedHookAllowsDisabledLeakboltWhenBinaryMissing` 如預期失敗：

   ```text
   hooks_test.go:267: 停用檢查必須排在執行檔檢查前面：
   --- FAIL: TestUnversionedHookAllowsDisabledLeakboltWhenBinaryMissing (0.09s)
   FAIL
   exit 1
   ```

   還原後同一測試通過，exit 0。

2. **三種 hook 目標：通過。** `TestAppendScriptHookIsIdempotent`、`TestAppendLefthookAddsToPreCommitBlock`、`TestAppendPreCommitConfigExpandsEmptyRepos` 都讀取產出檔並確認含 `git config --bool --get hooks.leakbolt`。

3. **字面 PATH：通過。** 上述三種目標測試都讀取產出檔並確認含字面 `PATH=$PATH`，不是建置時 PATH。

4. **禁止範圍：通過。** 未改規則檔、語料、manifest、`PLAN.md`、`DECISION.md`、`research/`、`scripts/` 或 `prototype/scan.go`；未對工作區執行 git 操作。測試要求的 git 初始化與設定只發生在 `t.TempDir()`。
