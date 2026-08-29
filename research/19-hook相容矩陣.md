# hook 共存相容矩陣（2026-08-29 實測）

> **平台狀態（2026-08-29）**
> - macOS 26（arm64）：**11/11 通過**，`./scripts/hook_matrix.sh`
> - Linux（Ubuntu 24.04 arm64 容器，git 2.43.0）：**11/11 通過**，`./scripts/linux_matrix.sh`
> - Windows：**待驗證**。測試包已備好（`dist/` 與 `scripts/windows_matrix.ps1`），
>   等在實體 Windows 機器上跑完回填。`PLAN.md` 階段一寫著首日支援三平台——
>   在結果回來之前，Windows 這塊沒有憑證，文案不能宣稱測過。
>
> **動手前已先修掉一個 Windows 必壞的 bug**：`verifyHookReachable` 原本用
> `info.Mode()&0o111 != 0` 判斷 hook 可不可執行，但 Go 在 Windows 從不設執行位元
> （`os.Stat` 只回 `0666` 或 `0444`），會讓**每一次 Windows 安裝都跳假警報**。
> 已改成 Windows 上只確認檔案存在——Git for Windows 靠副檔名與 Git Bash 決定能不能跑，
> 不看 POSIX 權限位元。`TestVerifyHookReachableIgnoresExecBitOnWindows` 鎖住這個分支。
>
> Windows 測試要回答的關鍵未知：**從 PowerShell 直接 `git commit` 時 hook 會不會被執行**。
> 如果不會，Windows 上等於沒有保護，階段一的三平台承諾就得改。

`PLAN.md` 階段一寫著「寫任何介面之前，先做 hook 共存的故障注入測試」。這是那道閘門的結果。

可重跑：`LEAKBOLT_BIN=<執行檔路徑> ./scripts/hook_matrix.sh`

測法：每個情境都建一個真的 git repo、真的裝上對應工具、暫存一個假的 AWS key、真的跑 `git commit`，
看密鑰有沒有進 repo。**不是檢查檔案內容有沒有寫對。**

## 結果：11 個情境全過

| 情境 | 結果 | 說明 |
|---|---|---|
| 乾淨 repo，本機安裝 | BLOCKED | |
| 先裝 husky，再裝 leakbolt | BLOCKED | `core.hooksPath=.husky/_`，我們寫進 `.husky/pre-commit` |
| 先裝 leakbolt，再裝 husky | BLOCKED_BY_OTHER | **我們失效了**，擋住的是 husky 預設的 `npm test` |
| 隊友沒裝 leakbolt | BLOCKED_BY_OTHER | 我們不可以把別人的檢查關掉 |
| lefthook 已 install | BLOCKED | |
| lefthook 設定在但沒跑過 install | WARNED | 安裝時就明講不會生效 |
| pre-commit 框架已 install | BLOCKED | |
| 在 git worktree 裡 commit | BLOCKED | worktree 共用 `.git/hooks` |
| `git commit --no-verify` | LEAKED | git 的設計，擋不住 |
| 第三方佔用 `core.hooksPath` | WARNED | 拒裝並說明現況 |
| 裝完之後 `core.hooksPath` 被改掉 | LEAKED | 靜默失效 |

## 累計抓到的五個真問題

### 1. 量測工具自己會給假陽性（最重要）

第一版只看「commit 有沒有失敗」就判定被擋住。但 husky 預設的 `.husky/pre-commit` 內容是 `npm test`，
空專案跑起來必定失敗，commit 照樣被擋——**我們的 hook 其實已經失效，測試卻回報成功**。

修法：在 commit 輸出裡找我們自己的指紋（`暫存區掃描完成`），分出 `BLOCKED` 與 `BLOCKED_BY_OTHER`。
沒有這一步，整份矩陣的結論都不可信。

### 2. 追加到 hook 尾端會被前面的失敗遮蔽

原本的實作把守衛行**追加**到 `.husky/pre-commit` 尾端。husky 的 `npm test` 先跑、先失敗，
腳本就結束了，我們那行根本沒機會執行。使用者以為裝好了，其實從來沒掃過。

修法：改成插在 shebang 之後、**最先執行**。資安檢查要最先跑，也不該被別人的失敗遮蔽。
`hooks_test.go` 的 `TestWriteScriptHookRunsFirst` 鎖住這個順序。

### 3. install 會回報「已安裝」但其實不會被觸發

寫進 `lefthook.yml` 或 `.pre-commit-config.yaml`，但使用者從沒跑過那些工具的 `install`，
`.git/hooks/` 裡沒東西，設定永遠不觸發。

修法：`verifyHookReachable` 解析 git 實際會執行的路徑（尊重 `core.hooksPath`），
不存在或不可執行就警告並說明要先跑哪個指令。

### 4. 前插之後，守衛把隊友的檢查一起關掉（改法自己引進的）

修完問題 2 之後，守衛變成腳本的第一段：

```sh
command -v leakbolt >/dev/null 2>&1 || exit 0
```

`exit 0` 結束的是**整個腳本**，不是跳過我們這段。所以在 husky 專案裡，
沒裝 leakbolt 的隊友會走到這行 → `exit 0` → 後面的 `npm test`、`lint-staged` 全部不執行，
**而且 commit 照樣成功**。`.husky/pre-commit` 進版控，會傳給每個隊友——
這比原本的問題嚴重，而且是我修問題 2 時自己弄出來的。

修法：換成 if 區塊，沒裝就往下走，不干擾任何人。

```sh
if command -v leakbolt >/dev/null 2>&1; then
  leakbolt scan --staged || exit 1
fi
```

**為什麼原本的十個情境測不到**：每個情境都有 leakbolt 在 PATH 上，
守衛永遠通過，那條路徑從沒被執行過。補上「隊友沒裝 leakbolt」這一列才抓得到。

這一列驗證過真的有效：把守衛改回 `|| exit 0` 重跑，立刻得到 `LEAKED`；改回 if 區塊就通過。

### 5. worktree 裡 `.git` 是檔案不是目錄（加誤報記憶時引進的）

新的狀態檔邏輯把路徑拼成 `<repo>/.git/leakbolt/`。在 git worktree 裡 `.git` 是一個**檔案**
（內容是指向真正 git 目錄的指標），拼出來就變成「往檔案裡面找目錄」，開檔直接失敗：

```
讀取本機狀態失敗：open .../wt/.git/leakbolt/state.json: not a directory
```

hook 因此報錯退出、commit 被擋——**但不是因為掃到密鑰，是因為我們自己壞了**。
矩陣的指紋判定正確地把它抓成 `BLOCKED_BY_OTHER`，這條規則救了一次。

修法：不要自己拼路徑，問 git 要 —— `git rev-parse --git-common-dir`。
worktree 共用主 repo 的 `.git/hooks`，狀態檔放同一個共用目錄才一致。
`state_features_test.go` 的 `TestStateDirectoryInWorktree` 鎖住這個行為。

## 兩個擋不住的情境，要靠健康檢查

`--no-verify` 與「裝完後 `core.hooksPath` 被改掉」都是 LEAKED，而且**都是靜默的**。
這證實了 `PLAN.md` 第四節缺口 #1 的判斷：危險的不是安裝失敗，是安裝當下成功、之後被靜默改掉。

對策（尚未實作）：
- 定期健康檢查，比對目前 `core.hooksPath` 與安裝時的狀態，變了就提示重裝
- `--no-verify` 擋不住，只能在後續掃描時偵測到並回報
- 這兩件事都要在文案裡明講，不能宣稱不可繞過
