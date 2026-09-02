# 安全性政策

## 回報漏洞

**請不要開公開 issue 回報安全漏洞。**

用 GitHub 的私密回報：到 [Security 頁面](https://github.com/aqua5230/leakbolt/security/advisories/new) 提交。這個管道只有維護者看得到。

回報時請盡量附上：受影響的版本（`leakbolt --version` 的輸出）、作業系統、重現步驟，以及你認為的影響範圍。

**不要在回報內容裡貼真的金鑰。** 這個專案處理的就是密鑰洩漏，回報時請用假值或截斷後的樣本。

處理時程：收到後會盡快確認並回覆，但這是單人維護的專案，請預期回覆時間以天計而非以小時計。

## 支援的版本

只有最新的釋出版本會收到安全性修正。目前沒有維護舊版分支的計畫。

## 這個工具本身的防護邊界

LeakBolt 是 **git commit 之前的第一層攔截**，不是保證。以下情況它擋不住，這些屬於已知設計限制，不是漏洞：

- `git commit --no-verify`，以及使用者自己執行 `git config hooks.leakbolt false`
- 不經 git 的部署路徑，例如直接上傳檔案、CI 從別處取得密鑰
- 部分 GUI git 客戶端不執行 hook
- 其他 hook 工具（husky、lefthook）接管 `core.hooksPath` 而把 LeakBolt 排擠掉——詳見 README 的「與其他 hook 工具共存」
- 已經進入 git 歷史的金鑰。把它從程式碼移進 `.env` 不等於修好，必須去供應商那邊作廢重發
- 偵測規則涵蓋不到的新供應商或新格式

**以下算漏洞，請回報**：

- 應該被擋下的金鑰卻通過了掃描（漏報），而它符合現有規則
- 掃描出錯卻讓 commit 通過（fail-open）
- `leakbolt doctor` 回報正常，但保護實際上已經失效
- LeakBolt 自己洩漏了它掃到的內容——誤報記錄只該存規則 ID 與不可逆指紋，若發現原始命中內容被寫進任何檔案或日誌，那是漏洞
- 安裝流程被利用來提權或寫入非預期的路徑

## 相依元件

偵測引擎是 [gitleaks](https://github.com/gitleaks/gitleaks)，版本鎖定在 v8.30.1，以子程序方式呼叫，不鏈結為函式庫。

gitleaks 本身的漏洞請回報給該專案。若某個問題同時牽涉 LeakBolt 對它的呼叫方式，兩邊都回報比較好。
