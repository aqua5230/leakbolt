# LeakBolt

在 git commit 之前自動攔截並防範 API 金鑰與密鑰外洩。

## 這是什麼／解決什麼問題

LeakBolt 幫你在 git commit 之前擋下不小心寫進程式碼的 API 金鑰與密鑰。裝一次，這台機器上所有 repo（包含之後新建的）都受保護。

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

安裝檔目前**未經 Apple 簽章與公證**，從瀏覽器下載後點兩下會被 Gatekeeper 擋。第一次請按右鍵選「打開」。簽章需要 Apple Developer Program 帳號；`scripts/build_pkg.sh` 已預留 `LEAKBOLT_SIGN_IDENTITY` 環境變數，設了才會簽。

解除安裝：`sudo sh /usr/local/leakbolt/uninstall.sh`。

### 方式三：自行編譯

需要 Go 1.21 或更新版本，執行 `cd prototype && go build -o leakbolt .`，然後將 `leakbolt` 放進 PATH。也可以到 [Releases 頁面](https://github.com/aqua5230/leakbolt/releases/latest) 直接下載對應平台的預編譯執行檔。

### macOS Gatekeeper 警告
若執行檔或安裝檔是從瀏覽器下載或他人傳送，macOS 會因為缺乏 Apple 簽章而阻擋執行，顯示「Apple 無法驗證…是否為惡意軟體」。用 `curl` 下載則不會被貼隔離標記，不會出現此警告。解法三選一：
- 按右鍵選「打開」
- 於終端機執行：`xattr -d com.apple.quarantine <檔案路徑>` 移除隔離標記
- 至「系統設定 → 隱私權與安全性」點擊「仍要打開」

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

- `install`：在目前 repo 裝上 pre-commit hook，並強制跑一次完整 git 歷史掃描把結果印出來。
- `install --global`：設定 `core.hooksPath` 指向 `~/.leakbolt/hooks`，一次保護這台機器上所有 repo。全域 hook 會先執行該 repo 自己的 `.git/hooks/pre-commit`（若存在且可執行）並傳遞其 exit code，再跑 LeakBolt 檢查，不會靜默停掉既有 hook。若 global `core.hooksPath` 已被其他工具佔用，會拒絕安裝而不覆寫。
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

偵測品質基準語料庫共 40 個樣本（19 個真陽性、21 個假陽性），目前漏報率 0%、誤報率 0%，可用 `sh scripts/quality_gate.sh` 重跑。

## 平台狀態

- **macOS**：相容矩陣 11/11 通過，有預編譯檔與 `.pkg` 安裝檔（未簽章）。視窗提示僅在 macOS 上運作。
- **Linux**：相容矩陣 11/11 通過，但**沒有預編譯檔**，要自己編。無視窗提示。
- **Windows**：**完全沒有實機驗證過**。`dist/` 裡有 Windows 執行檔，但沒有人在 Windows 上跑過。

## 已知限制

1. **hook 擋不住所有路徑。** `git commit --no-verify`、不經 git 的部署、部分 GUI git 客戶端、hook 被其他工具改寫，都能繞過。請把它當**第一層**防護，不是保證。
2. **把金鑰搬進 `.env` 不等於修好。** 如果那個金鑰曾經進過 git 歷史，就算搬走也必須去供應商那邊作廢重發。另外某些前端框架會把特定前綴的環境變數編進瀏覽器 bundle。
3. **規則會過期。** 鎖定 gitleaks 版本能給穩定基線，但新的供應商與新的 key 格式會繼續出現，不更新就會漏報。
4. **大型 repo 的 commit 延遲還沒量過。**
5. **誤報記錄只存規則 ID 與不可逆指紋**，不存原始命中內容。

## 授權

MIT。詳見 repo 根目錄的 `LICENSE`。

gitleaks 本身也是 MIT，LeakBolt 是以子程序方式呼叫它的執行檔，不是鏈結函式庫。
