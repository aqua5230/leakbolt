# LeakBolt — 定位與命名決策（2026-08-29）

## 一、產品名稱：LeakBolt

**語意**：bolt 兼三義——門閂（閂住洩漏）、閃電（引擎 <0.1 秒）、動詞 bolt（拴死）。
**唸法**：兩音節，英中皆好唸。中文暫定不取譯名，直接用 LeakBolt。

**可用性（2026-08-29 實測，非子代理自述）**
| 項目 | 結果 |
|---|---|
| npm `leakbolt` | 未被佔用（E404） |
| GitHub 搜尋 `leakbolt` | 零撞名（`gh search repos` 回 NONE） |
| leakbolt.com | 未註冊（RDAP 404） |
| leakbolt.dev | 未註冊（RDAP 404） |
| github.com/leakbolt（帳號名） | 未被佔用（404）→ 可拿 `github.com/leakbolt/leakbolt` |

**落選名與原因**：keyfuse（npm 0.2.6 已被佔）、keybolt（4 個同名 repo）、keylatch（有人佔了 org＋homebrew tap，明顯要做工具）、keyfence/keyward/seclip/keyproof/keyflare（GitHub 皆有撞名）、vibesafe/keytrap/hushkit/keyloom/leakless（npm 已被佔）。
**硬否決規則**：`*guard` / `*shield` / `secret*` / vault / sentry / keeper 系一律不取——語意飽和，SEO 排不上、GEO 上字串不會指向我們。

---

## 二、市場證據（來源：research/ 三份報告）

### 賽道空缺（交叉三份後成立的）
1. **gitleaks 官方已宣告功能凍結**，作者 Zachary Rice 轉去做 betterleaks（1807 stars，仍只有 CLI）。上層產品化空間沒人佔。
2. **官方無 GUI、無 MCP server**；gitleaks 的 GUI 包裝確實全是 0–17 star、停更一年以上的 PoC。
   **但 AI agent 防護那條線不空、對手還活著**（2026-08-29 我自己讀 README 核實，非子代理轉述）：
   - `cc-safety-net` 1512★，今天還在更新——但它擋的是危險指令與敏感**檔案路徑存取**，不掃 diff 內容裡的高熵字串與 API key。
   - `medusa` 971★——`secrets scan` 掃的是 AI 對話紀錄與 shell history，不是 git commit / diff，也沒有 pre-commit gate。
   - `ship-safe` 826★、`Claudoscope` 232★（macOS app）。
   **結論**：楔子仍然成立，但差異化要收窄成三件——確定性 pre-commit 攔截（掃 index + history，不只 HEAD）、誤報記憶、一鍵修復引導。AI agent hook 是入場券，不是賣點，不要拿它當主打。
3. **沒有任何產品做給非專業工程師**：GitGuardian（企業客製報價）、Doppler（$8/人/月起）、Infisical（$20/identity/月）、GitHub Secret Protection（$19/committer/月，私有 repo 才要錢）全部面向企業與資安工程師。< $10/月、零設定、有圖形介面的消費級產品：查無。
4. **GitHub 免費 Push Protection 只保公開 repo**，私有 repo 要 $19/committer/月；而且只擋 push 當下、規則限於合作廠商特徵、被擋時終端機直接給 bypass 連結一鍵繞過。

### 使用者真正要的（X 上第一人稱證據，非推測）
1. **要在 commit/push 之前擋，而且要掃 git history 與已被 track 的檔**——不是事後寄信。（@siyadhkc：「警報時已經進 repo」；@Nqspq 量過 Lovable/Bolt/Replit 公開 repo，25 個裡 1 個現況乾淨但 history 有 key）
2. **洩了之後主路徑是作廢舊 key ＋ 設花費上限**，不是「請去儀表板輪換」。（@rish_jain14：$145k／小時〔帳號存在已核實，該則貼文金額未逐條核實，對外文案不得引用此數字〕；@shazcodes：前端公開三個月 $12k；@theozbuilds：v0 把 key 打進瀏覽器，$4,000）
3. **AI 編碼路徑預設不碰真密鑰**：不讀 `.env`、不 commit `.env`、不把 key 寫進前端 bundle。（@swarogan：Cursor 讀 `.env` 無法自證；@geraldokolo_DC：Claude「commit 整個目錄」把 env 提交了）

### 三件不能忍
1. 帳單沒有天花板，幾分鐘到幾小時就能從學生專案變六位數。
2. 錯誤的安全感——刪行、刪 commit、加 gitignore、private repo，其實都沒修好。
3. 工具太晚／太吵／關得掉：誤報疲勞把真警報淹沒（@aashishhq 一次 34 個 gitleaks 誤報），push protection 一鍵可關（@0xmaharshi）。

---

## 三、產品方向（已經 sol 審查後修訂）

**一句話**：給用 AI 寫 code 的人的密鑰保險絲——在 commit 之前擋住，擋到就一步一步帶你把它處理乾淨。

**四個必要能力**
1. 確定性攔截：pre-commit / pre-push git hook，掃 working tree + index + 近期 history，不只 HEAD。這是產品的骨，不外包。
   安裝機制見下面「安裝方式」——**不設全域 `core.hooksPath`**，且 `install` 必須含一次全歷史掃描（hook 只擋未來的 commit，舊 commit 裡的 key 每次都會通過）。
2. 低誤報＋可操作：擋下來時給選項（一鍵移進 `.env`、一鍵標誤報且記住），不是 exit 1 加一串路徑。誤報記憶是留存的關鍵。
3. 止血閉環（**改為引導式，不自動代打**）：認出是 OpenAI / Anthropic / AWS / Stripe 這類會花錢的 key，直接開對應的 revoke 頁面、附上該家設花費上限的步驟與連結，並在使用者確認後複掃驗證。
4. AI agent 防線：Claude Code / Cursor 的 hook，預設 deny 讀寫 `.env`、`*credentials*`、`claude_desktop_config`；agent 要 commit 整個目錄時強制排除。

**技術基座：gitleaks（鎖版本 + 自帶補充規則）**
MIT 授權可自由封裝與分叉；官方功能凍結反而給我們一個穩定不會亂動的基線。betterleaks 太新，授權、介面穩定性、誤報改善都還沒有證據，先不押。自己寫規則引擎會把公司變成規則維護公司，不做。

**商業模式：核心免費開源 + 桌面版一次買斷 US$39**
CLI 與 git hook 免費開源（拿散播與信任）；macOS 桌面版買斷 $39，賣的是零設定安裝、互動式修復、誤報記憶。純本地工具沒有持續性成本，收月費在心理上站不住，而且會輸給免費方案。

**首發順序**
1. 先做 pre-commit 攔截 + 一鍵移進 `.env`（免費開源，這是入口）。
2. 第一個付費功能做「一鍵修掉前端洩漏的 key」——把 API 呼叫搬到後端、移除瀏覽器端 key、改引用、複掃。偵測免費、修復收費，用付款轉換率直接驗證有沒有人要買。

**安裝方式：偵測式 per-repo，不碰全域 `core.hooksPath`**（2026-08-29 決，證據與原始碼核對見 research/05、research/06）

事實：`core.hooksPath` 是單一值，設了就完全取代 `.git/hooks`，不合併；husky v9 安裝時必定執行 `git config core.hooksPath .husky/_`（repo 層級），會蓋掉全域設定；lefthook 與 pre-commit（Python）偵測到 `core.hooksPath` 已設定時直接拒裝報錯。

所以「裝一次全機器生效」不成立，而且反向會害使用者以後裝不了 lefthook／pre-commit。`leakbolt install` 改成偵測這個 repo 已有什麼 hook 工具，寫到對的位置：無工具 → `.git/hooks/pre-commit`（本機檔，不進版控）；有 husky → `.husky/pre-commit`；有 lefthook／pre-commit → 各自的設定檔。全域 `core.hooksPath` 只留作進階選項，且已被佔用就不動。

四個分支裡有三個寫進版控檔，會傳給沒裝 leakbolt 的隊友（`sh` 找不到指令回 exit 127，非 0 就中止 commit）。所以寫進版控的那行一律帶守衛：

```sh
command -v leakbolt >/dev/null 2>&1 || exit 0
leakbolt scan --staged || exit 1
```

安裝時要明講「這行會進版控、隊友看得到」，並提供 `--local-only` 走不進版控的 `.git/hooks/pre-commit`。
「正確地跟既有 hook 工具共存」本身就是免費層的賣點——gitleaks 官方沒做這件事。

> **現況違反此決策**（2026-09-02 補記）
>
> `.pkg` 的 postinstall 會自動執行 `leakbolt install --global`，也就是實際上碰了全域
> `core.hooksPath`，與上面「不碰全域」的決定相反。README 據此對外承諾「裝一次，這台機器上
> 所有 repo 都受保護」。這條決策不刪除，因為它指出的風險是真的——以下是 2026-09-02 的實測結果。
>
> **上面三個事實，實測後兩對一錯：**
>
> | 原文的說法 | 實測（2026-09-02，macOS，隔離 HOME） |
> |---|---|
> | husky v9 安裝時必定設 repo 層級 `core.hooksPath`，會蓋掉全域設定 | **成立**。真的跑 `npx husky init` 後，`git config --get core.hooksPath` 回 `.husky/_`，全域值被蓋掉，該 repo 的 commit 不再經過 LeakBolt |
> | pre-commit（Python）偵測到已設定時直接拒裝報錯 | **成立**。`pre-commit 4.5.1` 回 `[ERROR] Cowardly refusing to install hooks with core.hooksPath set.` 並且不安裝 |
> | lefthook 偵測到已設定時直接拒裝報錯 | **不成立**。`lefthook 2.1.12` 不拒裝，它印警告並給三個選項，其中 `lefthook install --reset-hooks-path` 會直接刪掉全域設定。比拒裝更危險——使用者照著它的提示做，就會無聲關掉全機保護 |
>
> **處置：全域模式保留，但補上偵測與復原（不是靠文件提醒）**
>
> 1. `leakbolt doctor` 現在會抓到 repo-local 與 worktree 層級的遮蔽，指出是哪一層設的，並叫使用者跑 `leakbolt install`。
> 2. `leakbolt install` 原本在全域模式下會把 LeakBolt 自己的路徑誤判成「別人佔用」而拒裝，
>    使用者照 doctor 的指示做卻什麼都沒發生。已修正：現在會走 per-repo 偵測，把檢查寫進
>    `.husky/pre-commit` 之類的位置，與該工具共存——也就是這條決策原本主張的做法。
> 3. 共存之後 `doctor` 回報正常（先前會誤報異常，那種假警報會讓人學會忽略 doctor）。
>
> 換句話說：**全域是預設的入口（零設定，散播用），per-repo 共存是遇到衝突時的落地方式**，
> 兩者都保留。這條決策指出的「裝一次全機器生效不成立」在裝了 husky 的 repo 上仍然為真，
> 所以 README 不能無條件宣稱全機保護，必須寫出這個限制。
>
> **仍未解**：`lefthook install --reset-hooks-path` 會靜默解除全機保護，目前只有事後靠 `doctor`
> 才發現得了。沒有辦法在 git 層攔截這件事。

**平台：CLI 三平台首日支援，付費 GUI 先只做 macOS**（2026-08-29 決，取代原「未解」第 2 條，證據見 research/04）

| | macOS | Windows |
|---|---|---|
| 開發者佔比（Stack Overflow 2025 專業使用） | 32.9% | 49.5% |
| 簽章年費 | Apple Developer $99／年，公證不另收 | OV $129～$313／年、EV $349～$411／年 |
| 首年硬體金鑰 | 無 | $90～$379（2023-06 起強制，私鑰不得存成檔案） |

gitleaks v8.30.1（2026-03-21）官方就有 `windows_x64` / `windows_arm64` / `linux_x64` / `darwin_arm64` 二進位（`gh` 核過），引擎跨平台零成本；hook 腳本則不是——`.git/hooks/pre-commit` 是 shell script，Windows 上靠 Git for Windows 內附的 Git Bash 執行，不附 Git Bash 的 GUI 客戶端會失效，列為已知限制。

CLI 不用簽章，首日支援 macOS／Windows／Linux（Windows 佔近半，砍掉等於砍掉一半散播面）。付費桌面 GUI 先只做 macOS，Windows 版等需求出現再付簽章成本。前提是核心引擎寫成單一可攜執行檔（Go 或 Rust），介面殼才用平台原生，不要把邏輯綁死在 macOS API 上。

**可擴展邊界**（2026-08-29 定）

能加、不破壞 $39 買斷的：規則包、更多 AI 工具的 hook、更多修復器、更多平台——全部純本地跑。
一加就垮的：團隊共享誤報清單、跨機器同步、集中報表、稽核紀錄——任何需要共享伺服器狀態的功能都會拉出帳號系統與月費。要做就另開一條產品線，不塞進買斷版。

---

## 四、sol 審查意見與我的處置

| sol 的意見 | 處置 |
|---|---|
| LeakBolt 可用，但 bolt 可能被聯想成五金／漏水 | 採納名稱。中文對外文案要固定講「密鑰保險絲」定調，避開五金聯想 |
| 商用前補做商標查核 | 待辦，未做 |
| 最致命：承諾跨供應商自動作廢 key ＋ 設硬上限。各家權限不同、要索取高權憑證，做不到會失信，做錯會停掉正式服務 | **採納**。能力 3 從「自動代打」降級為「引導 + 複掃驗證」，不碰使用者的高權憑證 |
| 基座選 gitleaks 鎖版，不選 betterleaks、不自寫 | 採納 |
| 定價 $39 買斷，不收月費 | 採納 |
| 首發做「一鍵修掉前端 key」 | 部分採納。pre-commit 攔截仍是首發（X 證據最強的痛在這），前端修復列為第一個付費功能 |

**未解**：商標查核（2026-08-29 查核中，結果進 research/07-商標查核.md）。

**已結案**：Windows/Linux 桌面版 → 見上面「平台」。誤報記憶跨機器同步 → 見上面「可擴展邊界」，屬於一加就垮那類，不做。
