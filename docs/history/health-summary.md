# LeakBolt 專案體檢（2026-08-29，Claude 整合）

來源：Claude 親測 + Codex（程式碼一致性）+ agy（檔案實體盤點）兩份子代理報告交叉比對。
凡標「已驗證」者為 Claude 本人跑過或讀過原始碼確認。

> **狀態（2026-08-29 22:35）**：第二節列的四個問題**都已修復**，修復細節見 `fix-report.md`。
> 修復後 7 條驗收指令全過，並用變異測試確認新增測試會真的失敗（把 system scope 拿掉，
> 三個新測試全部 FAIL）。本節內容保留為當時的體檢紀錄，不隨修復改寫。

## 一、綠燈（已驗證）

- `go build ./...` / `go vet ./...` / `go test ./...` 於 macOS 全部 exit 0，測試 `ok leakbolt 1.418s`
- Windows 交叉編譯：`GOOS=windows GOARCH=amd64` 與 `GOOS=windows GOARCH=arm64` 各自 vet + build 皆乾淨
- `dist/` binary 架構正確：amd64 為 `PE32+ executable (console) x86-64`，arm64 為 `PE32+ executable (console) Aarch64`
- binary 打包來源（`go version -m`）：兩支皆 `go1.27.0`、`CGO_ENABLED=0`、
  `GOOS=windows`、`GOARCH=amd64` / `arm64`，toolchain 與現在本機相同；
  打包時間 20:56、原始碼最後修改 20:55。時間與 toolchain 都相符，但沒有 build script
  留下記錄，無法百分之百證明就是這份原始碼編的。
- `sh scripts/quality_gate.sh` 通過：真陽性 19、假陰性 0、真陰性 21、假陽性 0，漏報率 0%、誤報率 0%
- 無殘留垃圾檔（無 .DS_Store、無備份檔、無空目錄）
- scripts/ 四支 .sh 皆有執行權限與正確 shebang；windows_matrix.ps1 已內建 Git Bash 三處候選路徑偵測

## 二、真問題

### 1. core.hooksPath 沒查 system 層（已讀原始碼確認）

三處迴圈都只跑 `{"local", "global"}`，沒有 `system`：
- prototype/hooks.go:67（installHook 的佔用偵測）
- prototype/hooks.go:253（effectiveHookPath）
- prototype/doctor.go:58（doctor 輸出）

後果：若使用者的系統層 git 設定有 core.hooksPath，install 會寫進 `<repo>/.git/hooks/pre-commit`
並回報成功，doctor 也顯示正常，但 git 實際會去系統設定指的目錄找 hook —— hook 不會被執行。
這正是 PLAN.md:86 承諾要抓的情境。

註：程式面（三處迴圈缺 system）由 Claude 讀原始碼確認；git 會依 local > global > system
的順序採用 core.hooksPath 則是 git 官方記載的行為。兩者合起來即為完整結論。
Windows 特別要留意——Git for Windows 自帶一份 system 層 gitconfig，這個缺口在
Windows 上最容易真的踩到，測試時值得專門試一次。

### 2. quality_gate.sh 永遠偵測不到規則分岔（已讀原始碼確認）

scripts/quality_gate.sh:21-25 的順序是先 `cp` 再 `cmp`：

```sh
cp rules/leakbolt.toml cmd/corpusbench/rules/leakbolt.toml
if ! cmp -s rules/leakbolt.toml cmd/corpusbench/rules/leakbolt.toml; then
	echo "quality gate 失敗：補充規則同步失敗" >&2
```

`cmp` 在 `cp` 之後跑，只有 `cp` 本身失敗才會不一致。腳本註解寫的是「兩份分岔的話，
量到的就不是產品實際行為」，但這段程式碼偵測不到分岔——它是把分岔直接覆蓋掉。
PLAN.md 把誤報率列為一票否決，而唯一量誤報的關卡永遠不會回報規則漂移。

### 3. 補充規則寫暫存檔失敗會無聲降級（已讀原始碼確認）

prototype/rules.go:19-32 的 `writeSupplementaryRules` 在 MkdirTemp 或 WriteFile 失敗時
回傳 `ok=false`；prototype/scan.go:46-49 是 `if configPath, cleanup, ok := writeSupplementaryRules(); ok`，
不 ok 就直接不帶 `--config` 跑 gitleaks。註解明寫「寫失敗不算致命」——設計是刻意的，
問題在於完全沒有訊息告訴使用者這次掃描少了自帶規則。

### 4. gitleaks 版本沒鎖（已讀原始碼確認）

PLAN.md:81、329 要求鎖 v8.30.1；prototype/scan.go:40 只有 `exec.LookPath("gitleaks")`，
找到就用，沒有讀版本也沒有比對。本機剛好是 8.30.1，別台機器可能不是。

## 三、次要

- prototype/implementation-notes.md:11-13 的 Deviation 說要 `GO111MODULE=off` 才過 —— 已過期，
  現在無環境變數直接跑就通過。文件該更新。
- 誤報語料庫只由 `scripts/quality_gate.sh` + `go run ./cmd/corpusbench` 驅動，
  `go test ./...` 不會碰它（corpusbench 是 main 套件，`[no test files]`）。
- 本次體檢跑 `sh scripts/quality_gate.sh` 時，該腳本的 `cp` 動作改寫了
  prototype/cmd/corpusbench/rules/leakbolt.toml（時間戳變成 22:08）。內容與
  prototype/rules/leakbolt.toml 相同，等於沒有實質變更，但檔案確實被動到了。
- PLAN.md:221-224 寫的 pre-push 與 CI 同一掃描指令尚未實作；CLI 目前只有
  install / scan / allow / untrack / doctor / purge-backups。
- dist/ 只有 Windows binary，沒有 macOS 與 Linux 產出物。
- 專案不是 git repo（`fatal: not a git repository`）。PLAN.md 22KB、20 份 research、
  整個 prototype 都沒版本控制。

---

## 四、2026-08-29 實機試用時新發現的問題

### hook 在 leakbolt 不在 PATH 上時無聲放行

`prototype/hooks.go` 寫進去的 pre-commit 內容是：

```sh
#!/bin/sh
# Added by LeakBolt
if command -v leakbolt >/dev/null 2>&1; then
  leakbolt scan --staged || exit 1
fi
```

`command -v leakbolt` 找不到執行檔時，整個 if 區塊跳過，hook 回 exit 0，commit 照常通過。
實測：把 leakbolt 放在 PATH 外，帶著 `AWS_ACCESS_KEY_ID = "AKIAIMNOJVGFDXXXE4OA"` 的檔案
commit 成功、完全沒有任何訊息；同一個檔案手動跑 `leakbolt scan --staged` 抓得到（exit 1）。

`leakbolt doctor` 也不會抓到這件事——它的「hook 守衛」只檢查 hook 檔案內容有沒有包含
`leakbolt scan --staged` 字串，不檢查 leakbolt 本身找不找得到。

這個 `command -v` 保護傘應該是為了讓沒裝 leakbolt 的隊友照樣能 commit（hook 設定
可能進版控，例如 husky 或 lefthook 的設定檔）。但對 `.git/hooks/pre-commit` 這種
只在本機生效、不進版控的情況，跑過 install 的人本來就有 leakbolt，之後找不到就是異常，
無聲放行等於 DECISION.md 三件不能忍第 2 條的「錯誤的安全感」。

**已修（2026-08-29 22:42）**，細節見 `fix-report-2.md`：

- hook 依「會不會進版控」分兩種。不進版控的 `.git/hooks/pre-commit` 找不到 leakbolt
  就直接擋；會進版控的 husky／lefthook／pre-commit 設定則先看
  `<git-common-dir>/leakbolt/state.json` 在不在——在就擋（這台機器裝過），
  不在就放行（沒裝過 leakbolt 的隊友）。
- `leakbolt doctor` 新增一行「leakbolt 執行檔」可達性檢查。

決定擋而不是放行的理由：`git commit --no-verify` 是 git 內建的萬用跳過方式，
擋下來不會把任何人卡死。而且沒有保護傘才是 shell hook 的預設行為——
指令找不到，sh 回 127，commit 本來就會被擋；原本那句 `command -v` 保護傘才是特例。

實機驗過四種情況：leakbolt 不在 PATH → 擋且訊息清楚；`--no-verify` → 放行；
leakbolt 在 PATH ＋ 有密鑰 → 正常擋下並給指紋；leakbolt 在 PATH ＋ 乾淨檔案 → 通過。


---

## 五、2026-08-29 依業界慣例補上的兩件事

查證過程與對照結論見 `research/24-對照結論-缺工具處理.md`，實作細節見 `fix-report-3.md`。

1. **停用開關 `git config hooks.leakbolt false`**（抄 gitleaks 的 `hooks.gitleaks`）。
   原因：`git commit --no-verify` 會關掉所有 hook，使用者同時有 lint 與測試時代價太大。
   停用檢查排在「找不到執行檔」檢查**前面**——停用之後就算執行檔被刪掉也不該報錯。
   停用中每次 commit 都印一行提醒，`doctor` 報 `LeakBolt 啟用狀態：異常` 並給恢復指令。
2. **找不到執行檔的訊息印出 `PATH=$PATH`**（抄 husky 的
   `command not found in PATH=$PATH`），方便診斷 GUI git 工具 PATH 不同的情況。

實機驗過：預設啟用＋有密鑰 → 擋；停用後 → 放行並印提醒；停用＋執行檔不在 PATH → 一樣放行
（不會誤報找不到）；doctor 停用時 exit 1、恢復後 exit 0。
變異測試：把停用檢查搬到執行檔檢查後面，`TestUnversionedHookAllowsDisabledLeakboltWhenBinaryMissing` FAIL。

## 六、尚未處理，留給之後

`gitleaks --help`（8.30.1）列出的指令只有 `git` / `dir` / `stdin`，
**`protect` 與 `detect` 已從說明中隱藏**（仍能執行，實測輸出與新指令一致）。
LeakBolt 三處在用：`prototype/scan.go` 的 `scanStaged`（`protect --staged`）、
`scanHistory`（`detect`）、`prototype/cmd/corpusbench/main.go` 的 `scanFile`
（`detect --no-git --source`）。鎖版 8.30.1 期間不影響，升版前要換成
`gitleaks git --pre-commit --staged` 這類新寫法。詳見 `research/24` 第五節。
