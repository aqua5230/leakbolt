# LeakBolt 體檢修復報告

日期：2026-08-29

## 修復一：補齊 system 層 core.hooksPath

### 改動

- `prototype/hooks.go:67-81`：install 佔用偵測依 `local, global, system` 順序查詢；system 設定檔不可讀時視為未設定，不中止安裝。
- `prototype/hooks.go:255-264`：實際 hook 路徑依相同順序解析，讓只有 system 設定時仍找到 git 會執行的路徑。
- `prototype/state.go:40-46`：`InstallState` 新增 `system_hooks_path_digest`。
- `prototype/doctor.go:11-47`：安裝時記錄 system 摘要；system 設定讀取失敗時記錄「未設定」摘要。
- `prototype/doctor.go:67-106`：doctor 檢查三個 scope，使用三路 switch 取預期摘要。舊 state 缺 system 欄位時明確印「未記錄」並回異常。
- `prototype/hooks_test.go:144-160`：以暫存 system gitconfig 驗證 install 會拒絕佔用，且實際 hook 路徑採用 system 設定。
- `prototype/state_features_test.go:186-235`：以 `GIT_CONFIG_SYSTEM` 指向暫存 gitconfig，驗證安裝後 system hooksPath 改變會被 doctor 抓到；另驗證舊 state 的「未記錄」輸出。未寫真實系統設定。

### 原因

Git 依 local、global、system 決定 `core.hooksPath`。漏查 system 會把 hook 寫到 git 不會執行的位置，且 doctor 誤報正常。system 設定檔不可讀則依需求當未設定，避免 Windows 等環境因系統檔權限中止安裝。

### 驗證

- 新測試 `TestDoctorDetectsChangedSystemHooksPath` 通過。
- 新測試 `TestSystemHooksPathBlocksInstallAndControlsEffectivePath` 與 `TestDoctorReportsUnrecordedSystemHooksPathForOldState` 通過。
- 反向測試暫時把 doctor scope 改回 `local, global` 後，新測試失敗；還原 `system` 後通過。

## 修復二：quality gate 改為偵測規則分岔

### 改動

- `scripts/quality_gate.sh:21-26`：刪除自動 `cp`，先 `cmp`。不一致時 exit 2，訊息附上：
  `cp prototype/rules/leakbolt.toml prototype/cmd/corpusbench/rules/leakbolt.toml`
- 同段註解改為「只檢查、不覆寫」。開頭的上限由來與收緊紀錄未改。

### 原因

舊流程先覆寫再比較，永遠抓不到規則漂移。新流程保留兩份檔案原狀，讓品質閘門在量測前阻擋分岔。

### 驗證

- 兩份規則原本一致，未修改規則內容。
- `sh scripts/quality_gate.sh` 通過，漏報率與誤報率皆為 0%。

## 修復三：補充規則寫檔失敗不再無聲降級

### 改動

- `prototype/rules.go:18-32`：`writeSupplementaryRules` 改回傳實際 error，保留清理暫存目錄。
- `prototype/scan.go:33-57`：`scanStaged`、`scanHistory`、`runGitleaks` 接收 stderr。規則寫檔失敗時印出原因與降級範圍，仍用 gitleaks 預設規則繼續掃。
- `prototype/main.go:103,133,158`：install、staged scan、history scan 都把既有 stderr 傳入。
- `prototype/cmd/corpusbench/rules.go:17-29`：量測工具的規則寫檔函式也回傳 error。
- `prototype/cmd/corpusbench/main.go:142-150`：規則寫檔失敗立即回 error；上層回非零 exit code，不再用預設規則量測。

### 原因

正式掃描允許降級，但必須告知使用者缺少 LeakBolt 自帶規則；品質量測若降級，數字失真，所以直接中止。

### 驗證

- `go test ./...` 與 `go vet ./...` 通過所有新簽名呼叫鏈。
- 品質閘門成功載入補充規則並維持 19 真陽性、21 真陰性、0 假陰性、0 假陽性。

## 修復四：檢查 gitleaks 8.30.1

### 改動

- `prototype/scan.go:13`：新增 `requiredGitleaksVersion = "8.30.1"`。
- `prototype/scan.go:41-50,79-94`：每條掃描路徑先執行一次 `gitleaks version`；去除頭尾空白與小寫 `v` 前綴。版本不符只警告，繼續掃描。
- `prototype/doctor.go:122-136`：版本相符報正常；不符、找不到或查詢失敗都報異常並令 doctor 回 1。
- `prototype/scan_test.go:12-41`：驗證版本只查一次、容忍空白與 `v` 前綴，且版本不符只警告、掃描仍成功。

### 原因

品質基準由 gitleaks 8.30.1 產生。掃描維持可用性，不因版本差異中止；doctor 則明確指出環境偏離基準。

### 驗證

- 主程式每次只會走一個 scan 分支，`runGitleaks` 內只呼叫一次版本查詢；doctor 也只呼叫一次。未加 `sync.Once` 或快取。
- 本機 gitleaks 8.30.1 可完成完整品質閘門。

## 完整驗證輸出

沙盒的預設 Go 快取目錄不可寫，因此以下 Go 指令繼承 `GOCACHE=/tmp/leakbolt-go-cache`；指令本身與成功標準相同，沒有設定 `GO111MODULE`。

```text
$ cd prototype && go build ./...
exit 0

$ cd prototype && go vet ./...
exit 0

$ cd prototype && go test ./...
ok  	leakbolt	(cached)
?   	leakbolt/cmd/corpusbench	[no test files]
exit 0

$ GOOS=windows GOARCH=amd64 go build ./...
exit 0

$ GOOS=windows GOARCH=arm64 go build ./...
exit 0

$ sh scripts/quality_gate.sh
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

新增最後一批測試後也曾用 `go test ./...` 非快取執行，實際輸出：

```text
ok  	leakbolt	2.141s
?   	leakbolt/cmd/corpusbench	[no test files]
exit 0
```

## 自我審查

1. 四項修復皆完成：system scope 三處迴圈與 state/doctor、規則分岔閘門、規則寫檔錯誤傳遞、gitleaks 版本警告與 doctor 檢查都有程式碼和測試或品質閘門覆蓋。
2. 未修改禁止檔案：兩份 `leakbolt.toml` 內容未動且 `cmp` exit 0；`prototype/corpus/`、`PLAN.md`、`DECISION.md`、`research/`、`go.mod` 都未動。quality gate 的 0% 上限與開頭註解未改；沒有 git 操作、commit 或新增依賴。
3. 反向測試：暫時把 `prototype/doctor.go` 的迴圈改回 `local, global`，`TestDoctorDetectsChangedSystemHooksPath` 如預期 exit 1：

   ```text
   --- FAIL: TestDoctorDetectsChangedSystemHooksPath (0.29s)
       state_features_test.go:204: doctor 未偵測 system hooksPath 改動：core.hooksPath (local)：正常
           core.hooksPath (global)：正常
           hook 可達性：異常（git 找不到可執行的 pre-commit）
           hook 守衛：正常
           gitleaks 版本：正常
           建議：leakbolt install
   FAIL
   exit status 1
   FAIL	leakbolt	0.763s
   ```

   還原 `system` 後同一測試：

   ```text
   PASS
   ok  	leakbolt	0.475s
   exit 0
   ```

4. 版本查詢次數：`TestRunGitleaksChecksVersionOnceAndAcceptsVPrefix` 用假 gitleaks 記錄 `version` 呼叫，斷言恰好一次。程式中 scan 與 doctor 各只有一個 `gitleaksPathAndVersion()` 呼叫點，且兩條指令路徑互斥；未加快取。
