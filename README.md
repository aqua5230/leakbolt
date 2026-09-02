# LeakBolt

在 git commit 之前自動攔截並防範 API 金鑰與密鑰外洩。

![LeakBolt 攔截 commit 裡的 AWS 金鑰示範](docs/demo.gif)

## 這是什麼／解決什麼問題

LeakBolt 幫你在 git commit 之前擋下不小心寫進程式碼的 API 金鑰與密鑰。裝一次，這台機器上所有 repo（包含之後新建的）都受保護——除非某個 repo 用了 husky／lefthook 這類 hook 工具，它們會接管 hook 設定；那種 repo 執行一次 `leakbolt install` 就能讓兩者共存（詳見[與其他 hook 工具共存](#與其他-hook-工具共存)）。

它不自己做偵測——偵測引擎用的是 gitleaks。LeakBolt 做的是 gitleaks 沒做的那一層：一個安裝檔裝好全部（含 gitleaks）、一次設定保護所有 repo、檢查 hook 有沒有失效、擋下時跳原生視窗說明、處理誤報、把誤加進版控的憑證檔移出去。

**全部在本機執行。** 不需要註冊帳號、不需要 API 金鑰、沒有掃描次數上限，程式碼不會離開這台電腦，離線也能用。

## 前置需求

用 `.pkg` 安裝的話沒有前置需求，gitleaks v8.30.1 已經包在裡面，裝到 `/usr/local/leakbolt/bin/gitleaks`，不會覆蓋你自己裝的那份。

自行編譯的話要**自己先裝 gitleaks，版本 v8.30.1**：

- macOS 安裝方式：`brew install gitleaks`
- 官方專案頁面：https://github.com/gitleaks/gitleaks

若找不到 gitleaks，執行 `leakbolt scan` 會回報錯誤並中止：
`找不到 gitleaks。請安裝 gitleaks v8.30.1 後重試：https://github.com/gitleaks/gitleaks`

版本若不是 8.30.1，每次執行會印出警告，但仍會繼續掃描。

## 安裝

### 方式一：Homebrew（推薦，開發者最少摩擦）

```
brew install aqua5230/leakbolt/leakbolt
```

會自動裝好 leakbolt 本體，並透過 `depends_on` 一併裝好 gitleaks，不會跳 Gatekeeper 警告。裝完後執行 `leakbolt install --global` 開啟全機保護（Homebrew 版不會自動開啟，需手動執行一次）。

### 方式二：`.pkg` 安裝檔（不用開終端機）

到 [Releases 頁面](https://github.com/aqua5230/leakbolt/releases/latest) 下載 `LeakBolt-<版本>.pkg`，內含 leakbolt 與 gitleaks，兩者都是 universal binary，Intel 與 Apple Silicon 共用同一個檔案。也可以自行執行 `sh scripts/build_pkg.sh` 本機產生。

安裝檔會做三件事：把 `leakbolt` 放進 `/usr/local/bin`、把 gitleaks 放進 `/usr/local/leakbolt/bin`、以目前登入的使用者身分執行 `leakbolt install --global` 開啟全機保護。

安裝檔目前**未經 Apple 簽章與公證**，從瀏覽器下載後點兩下會被 Gatekeeper 擋，且 macOS Sequoia（15）起已移除「按右鍵選打開」這條捷徑，只剩「系統設定」這條路：點兩下讓它被擋 → 開啟「系統設定 → 隱私權與安全性」→ 捲到「安全性」區塊 → 按被擋項目旁的「仍要打開」→ 在重新跳出的警告按「打開」並輸入管理者密碼。

這幾步對不常開終端機的人相當勸退，所以**目前建議優先走 Homebrew**。簽章需要 Apple Developer Program 帳號（年費 US$99，公證不另外收費也不需要硬體金鑰）；`scripts/build_pkg.sh` 已預留 `LEAKBOLT_SIGN_IDENTITY` 環境變數，設了才會簽。

解除安裝：`sudo sh /usr/local/leakbolt/uninstall.sh`。

### 方式三：自行編譯

需要 Go 1.21 或更新版本，執行 `cd prototype && go build -o leakbolt .`，然後將 `leakbolt` 放進 PATH。也可以到 [Releases 頁面](https://github.com/aqua5230/leakbolt/releases/latest) 直接下載對應平台的預編譯執行檔。

### macOS Gatekeeper 警告
若執行檔或安裝檔是從瀏覽器下載或他人傳送，macOS 會因為缺乏 Apple 簽章而阻擋執行，顯示「Apple 無法驗證…是否為惡意軟體」。用 `curl` 下載則不會被貼隔離標記，不會出現此警告。解法二選一：
- 至「系統設定 → 隱私權與安全性」，捲到「安全性」區塊點擊「仍要打開」（macOS Sequoia 起這是唯一的圖形介面途徑，舊版的「按右鍵選打開」已被 Apple 移除）
- 於終端機執行：`xattr -d com.apple.quarantine <檔案路徑>` 移除隔離標記

自行用 `go build` 編譯的執行檔不會遇到此問題。

## 快速開始

用 `.pkg` 安裝的話，裝完就生效，這台機器上所有 repo（包含之後新建的）都受保護，不必再做任何事。

自行編譯的話，執行一次 `leakbolt install --global` 開啟全機保護；若只想保護單一 repo，在該 repo 根目錄執行 `leakbolt install`。

驗證是否生效：新增一個包含假金鑰的檔案並加入暫存區，執行 `git commit`。commit 會被攔截中斷，終端機顯示偵測到的金鑰警告，macOS 上同時跳出提示視窗。

## 指令

```
用法：
  leakbolt install [--local-only | --global]
  leakbolt uninstall --global
  leakbolt scan --staged
  leakbolt scan --history
  leakbolt allow <指紋前綴>
  leakbolt purge-backups
  leakbolt untrack <檔案路徑>
  leakbolt doctor
```

版本查詢指令：支援 `leakbolt --version`、`leakbolt -v`、`leakbolt version`，輸出格式如 `leakbolt 0.1.0 (3ba8f46)，鎖定 gitleaks 8.30.1`。

- `install`：在目前 repo 裝上 pre-commit hook，並強制跑一次完整 git 歷史掃描把結果印出來。歷史掃描找到東西時**不會**讓 `install` 回傳非 0——hook 已經裝好了，那些命中是資訊，不是安裝失敗。要取得歷史掃描本身的 exit code 請用 `leakbolt scan --history`。
- `install --global`：設定 `core.hooksPath` 指向 `~/.leakbolt/hooks`，一次保護這台機器上所有 repo。全域 hook 會先執行該 repo 自己的 `.git/hooks/pre-commit`（若存在且可執行）並傳遞其 exit code，再跑 LeakBolt 檢查，不會靜默停掉既有 hook。若 global `core.hooksPath` 已被其他工具佔用，會拒絕安裝而不覆寫。用了 husky／lefthook 的 repo 會蓋過這個設定，見[與其他 hook 工具共存](#與其他-hook-工具共存)。
- `uninstall --global`：解除全域保護。只在 global `core.hooksPath` 確實指向 LeakBolt 時才解除，指向他人設定則拒絕並保留原值。
- `scan --staged`：掃暫存區（staged，指已 `git add` 但還沒 commit 的內容）。
- `scan --history`：掃完整 git 歷史。
- `allow <指紋前綴>`：把某一筆標成誤報並記住，之後不再擋。指紋前綴至少 8 個字元。
- `untrack <檔案路徑>`：把誤被 git 追蹤的憑證檔移出版控。
- `doctor`：健康檢查，看 hook 是不是還有效，並回報目前是全域模式或單一 repo 模式。
- `purge-backups`：清掉明文備份。

## 視窗提示

macOS 上 commit 被擋時，除了既有的終端機輸出，會額外跳出原生對話框，顯示檔案、規則與指紋（不顯示命中的原始內容）。單筆命中時提供「標為誤報並放行」按鈕，預設按鈕是「知道了」；兩筆以上只提供「知道了」。對話框 120 秒無回應自動關閉，視同「知道了」。

視窗只是附加提示：非 macOS、`CI` 環境變數非空、無圖形介面的環境，或 `osascript` 失敗時都不會顯示，且**不影響掃描結果與 exit code**——擋下的仍然照擋。

關閉方式：設定環境變數 `LEAKBOLT_NO_GUI`，或執行 `git config leakbolt.gui false`。

## 停用

- **單一 repo 停用**：執行 `git config hooks.leakbolt false`。停用期間每次 commit 會印出一行提醒，`leakbolt doctor` 會回報異常並提供恢復指令。恢復：`git config --unset hooks.leakbolt`。
- **解除全機保護**：執行 `leakbolt uninstall --global`。
- **只關視窗提示**：設定 `LEAKBOLT_NO_GUI` 環境變數，或執行 `git config leakbolt.gui false`。掃描與攔截不受影響。
- **單次跳過**：使用 `git commit --no-verify`，但會同時跳過該 repo 的所有其他 hook。

## 偵測範圍

除了 gitleaks 內建規則，LeakBolt 自帶補充規則，涵蓋 gitleaks 沒收但官方有公開格式的供應商：Groq、Replicate、OpenRouter、xAI、Fireworks、DeepSeek、Supabase secret key 與 PAT、Clerk secret key。規則內嵌在執行檔裡，不依賴外部檔案。

同時**刻意排除**設計上就該公開的金鑰以避免誤報：Clerk publishable、Stripe publishable、Supabase publishable 與 anon key。但排除規則不涵蓋「只寫 `NEXT_PUBLIC_`」的情況——把 service_role key 放進 `NEXT_PUBLIC_` 變數本身就是嚴重洩漏，照常抓取。

偵測品質基準語料庫共 40 個樣本（19 個真陽性、21 個假陽性），**在這組基準上**漏報率 0%、誤報率 0%，可用 `sh scripts/quality_gate.sh` 重跑。這是基準測試集的數字，不是真實世界的保證——實際 repo 的誤報情形見下面「已知限制」第 6 條。

**測試資產路徑濾除**：gitleaks 預設的 `generic-api-key`、`square-access-token`、`private-key` 這三條屬於高熵猜測型規則，在測試檔與測試資料夾裡命中率極高但幾乎都是雜訊。`scan --history` 會濾掉這三條規則落在測試路徑（`*_test.go`、`*.test.js`、`*.spec.ts`、`test_*`、`testdata/`、`fixtures/`、`__tests__/`、`tests/`、`spec/` 等）的命中，並在摘要末尾回報濾掉幾筆；設定 `LEAKBOLT_NO_TEST_FILTER=1` 可看完整結果。

濾除**只套用在歷史掃描**，`scan --staged`（真正擋下 commit 的那條路徑）維持完整靈敏度不濾。LeakBolt 自帶的補充規則也一律不濾——測試檔裡出現真的供應商金鑰仍然是洩漏。

## 平台狀態

- **macOS**：相容矩陣 11/11 通過，有預編譯檔與 `.pkg` 安裝檔（未簽章）。視窗提示僅在 macOS 上運作。
- **Linux**：相容矩陣 11/11 通過，但**沒有預編譯檔**，要自己編。無視窗提示。
- **Windows**：真實 Windows + Git Bash 上 `go test ./...` 全數通過（7 個 POSIX-only 測試依設計 SKIP），並已驗證 install、scan、密鑰攔截基本流程與 `install --global` hook 串接。完整 `scripts/windows_matrix.ps1` 相容矩陣及 husky/lefthook/pre-commit 整合情境尚未測試。

## 與其他 hook 工具共存

git 的 `core.hooksPath` 是單一值，設了就完全取代原本的 hook 目錄，不會合併。husky、lefthook、pre-commit 都靠這個設定運作，所以**在同一個 repo 裡，後設定的那個會接管全部**。

實測（2026-09-02，macOS）三種工具的行為：

| 工具 | 在全機保護已開啟時執行它的 install | 後果 |
|---|---|---|
| husky v9 | 照裝不誤，設 repo 層級 `core.hooksPath=.husky/_` | **全機保護被蓋掉**，該 repo 的 commit 不再經過 LeakBolt |
| lefthook 2.1.12 | 不拒裝，印警告並提供 `lefthook install --reset-hooks-path` 選項 | 照那個提示做會**刪掉全域設定**，全機保護整台機器失效 |
| pre-commit 4.5.1 | 拒裝，回報 `[ERROR] Cowardly refusing to install hooks with core.hooksPath set.` | 保護仍在，但使用者裝不了 pre-commit |

**遇到這些情況怎麼辦**：在那個 repo 執行一次 `leakbolt install`。它會偵測到該工具，把檢查寫進對應的位置（`.husky/pre-commit`、`lefthook.yml`、`.pre-commit-config.yaml`），插在既有指令前面，兩者共存。之後 `leakbolt doctor` 會回報正常。

**怎麼知道自己中了**：執行 `leakbolt doctor`。它會指出是哪一層的設定蓋掉全機保護、該跑什麼指令。但它不會自動執行——裝完 husky／lefthook 之後請主動跑一次。

**已知無解**：`lefthook install --reset-hooks-path` 會直接移除全域 `core.hooksPath`，靜默解除整台機器的保護。git 層沒有辦法攔截，只能事後靠 `leakbolt doctor` 發現。

## 掃描出錯時的行為

掃描過程出錯一律**中止 commit**（fail-closed），不會因為「掃不動」就放行：

- gitleaks 以非 0、非 1 的狀態離開（設定損毀、repo 讀不到、內部錯誤）——即使它同時輸出了合法的 JSON 報告，也視為掃描失敗，不當成「找到 0 筆」。
- 補充規則暫存檔寫不出來（例如 `TMPDIR` 不可寫）——中止並提示，不會退回只用 gitleaks 預設規則掃。降級會讓只有補充規則抓得到的金鑰（Groq、Supabase secret、Clerk secret 等）整批通過，而畫面上仍寫著「找到 0 筆」。
- 找不到 gitleaks、JSON 解析失敗、`state.json` 讀寫失敗——同樣中止。

唯一會放行的例外是使用者明確要求的：`git config hooks.leakbolt false` 與 `git commit --no-verify`，兩者都會留下訊息。

## 已知限制

1. **hook 擋不住所有路徑。** `git commit --no-verify`、不經 git 的部署、部分 GUI git 客戶端、hook 被其他工具改寫，都能繞過。請把它當**第一層**防護，不是保證。

   特別注意 **repo 層級或 worktree 層級的 `core.hooksPath` 會蓋過全機保護**，而且是靜默的——除非你主動跑 `leakbolt doctor`，不會有任何提示。詳見上面的[與其他 hook 工具共存](#與其他-hook-工具共存)。
2. **把金鑰搬進 `.env` 不等於修好。** 如果那個金鑰曾經進過 git 歷史，就算搬走也必須去供應商那邊作廢重發。另外某些前端框架會把特定前綴的環境變數編進瀏覽器 bundle。
3. **規則會過期。** 鎖定 gitleaks 版本能給穩定基線，但新的供應商與新的 key 格式會繼續出現，不更新就會漏報。
4. **`scan --history` 在大型 repo 上很慢。** 在 `golang/go` 的 4215 個 commit 上實測 53～56 秒（Apple Silicon）。`install` 會跑一次這個掃描，所以大型 repo 的安裝會等上將近一分鐘。相對地 `scan --staged`（每次 commit 實際跑的那個）實測 0.2～0.3 秒，日常 commit 感覺不到延遲。
5. **誤報記錄只存規則 ID 與不可逆指紋**，不存原始命中內容。
6. **真實 repo 仍會有誤報。** 實測 5 個公開專案的完整歷史：`gin-gonic/gin` 4 筆、`caddyserver/caddy` 5 筆、`golang/go` 257 筆，`expressjs/express`、`sharkdp/bat`、`junegunn/fzf` 各 0 筆——命中的全部是測試資產，真洩漏 0 筆。加入測試路徑濾除後，gin 降到 0 筆、golang/go 降到 23 筆（剩下的是密碼學實作檔裡的高熵常數，不在測試路徑上）。也就是說：誤報變少了，但沒有歸零，這類專案仍需搭配 `leakbolt allow` 使用。

## 授權

MIT。詳見 repo 根目錄的 `LICENSE`。

gitleaks 本身也是 MIT，LeakBolt 是以子程序方式呼叫它的執行檔，不是鏈結函式庫。
