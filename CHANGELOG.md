# 變更紀錄

格式依照 [Keep a Changelog](https://keepachangelog.com/zh-TW/1.1.0/)，版本號依照 [語意化版本](https://semver.org/lang/zh-TW/)。

只記使用者看得到的變化。內部重構、測試調整、文件錯字不列入。

## [未發布]

### 新增

- **`leakbolt guide <指紋前綴>`：引導式止血。** 認出金鑰屬於哪個供應商（OpenAI、Anthropic、AWS、Stripe、Groq、Replicate、OpenRouter、xAI、Fireworks、DeepSeek、Supabase、Clerk），印出撤銷頁面與支出上限設定連結；確認後才開瀏覽器，按 Enter 後才複掃驗證。只引導，不自動撤銷金鑰、不代設支出上限——LeakBolt 沒有也不會去要求你的供應商帳號權限。複掃只能確認「這個字串還在不在暫存區」，金鑰本身是否已撤銷仍要你自己到供應商網站確認。目前只涵蓋 `scan --staged` 留下的指紋。

### 修正

- **掃描出錯時不再放行 commit。** 兩條路徑會讓掃描失敗被當成「找到 0 筆」：
  - gitleaks 以非 0 非 1 的狀態離開時，只要它輸出合法 JSON 就被採信。實測 `exit 2` 配上 `[]` 會讓含金鑰的 commit 通過。現在只接受 gitleaks 約定的 0（乾淨）與 1（有命中）。
  - 補充規則暫存檔寫不出來時（例如 `TMPDIR` 不可寫），原本會降級成只用 gitleaks 預設規則掃，導致只有補充規則抓得到的金鑰（Groq、Supabase secret、Clerk secret 等）整批放行。現在改為中止並提示。
- **`leakbolt doctor` 不再把已失效的保護回報成正常。** worktree 層級的 `core.hooksPath`（`.git/config.worktree`）會蓋過全機保護，但先前完全沒被檢查——commit 不再被擋，`doctor` 卻回報一切正常。
- **被 husky 蓋掉後，`leakbolt install` 現在能正確復原。** 先前它會把 LeakBolt 自己的全域路徑誤判成「別人佔用」而拒裝，使用者照 `doctor` 的指示執行卻沒有任何效果。
- **與 husky 共存後 `doctor` 不再誤報異常。** 檢查已寫進 `.husky/pre-commit`、commit 確實被攔截，卻仍回報「蓋過全機保護」。
- **`leakbolt install` 不再因為歷史掃描有命中而回傳非 0。** hook 已經裝好，那些命中是資訊而非安裝失敗；非 0 會讓 `.pkg` 的安裝流程誤判成失敗。要取得歷史掃描的結果碼請用 `leakbolt scan --history`。

### 變更

- **`scan --history` 會濾掉測試資產路徑上的高噪音命中。** gitleaks 預設的 `generic-api-key`、`square-access-token`、`private-key` 這三條規則在測試檔與測試資料夾裡幾乎都是雜訊。實測 `golang/go` 從 257 筆降到 23 筆、`gin-gonic/gin` 從 4 筆降到 0 筆，兩者的真洩漏本來就都是 0。

  濾除只套用在歷史掃描，`scan --staged`（實際擋下 commit 的路徑）維持完整靈敏度；LeakBolt 自帶的補充規則一律不濾——測試檔裡出現真的供應商金鑰仍然是洩漏。設定 `LEAKBOLT_NO_TEST_FILTER=1` 可看完整結果。

- **`leakbolt doctor` 的訊息更明確**：指出是哪一層設定蓋掉全機保護、實際會執行哪個 hook、該跑什麼指令復原。

### 文件

- README 的 macOS Gatekeeper 指引已過時——Sequoia（15）移除了「按右鍵選打開」，改寫成目前唯一可行的「系統設定 → 隱私權與安全性 → 仍要打開」。
- 「誤報率 0%」的說法限定為基準測試集上的數字，並補上真實 repo 的實測結果。
- 補上實測效能：`scan --staged` 0.2～0.3 秒；`scan --history` 在 `golang/go`（4215 commits）53～56 秒。
- 新增「與其他 hook 工具共存」章節，列出 husky、lefthook、pre-commit 在全機保護已開啟時的實測行為與復原方式。
- 新增 `SECURITY.md`：漏洞回報管道與防護邊界。

## [0.1.0] - 2026-08-31

首個版本。

- `pre-commit` hook 攔截，掃暫存區與完整 git 歷史
- 全機保護模式（`install --global`），以及偵測式 per-repo 安裝，可與 husky、lefthook、pre-commit 共存
- 補充規則涵蓋 gitleaks 未收錄的供應商：Groq、Replicate、OpenRouter、xAI、Fireworks、DeepSeek、Supabase secret key 與 PAT、Clerk secret key
- 刻意排除設計上就公開的金鑰：Clerk / Stripe / Supabase publishable 與 anon key
- 誤報記憶（`allow`），只存規則 ID 與不可逆指紋
- 健康檢查（`doctor`）
- 把誤加進版控的憑證檔移出去（`untrack`）
- macOS 原生視窗提示
- macOS `.pkg` 安裝檔與 Homebrew tap；Windows 與 Linux 提供 CLI

[未發布]: https://github.com/aqua5230/leakbolt/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/aqua5230/leakbolt/releases/tag/v0.1.0
