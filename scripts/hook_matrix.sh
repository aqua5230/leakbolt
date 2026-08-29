#!/usr/bin/env bash
# hook 共存相容矩陣：對每種情境建真的 repo，暫存一個假密鑰，看 commit 到底有沒有被擋。
#
# 判定的是「真的有沒有保護」，不是「檔案有沒有寫進去」。
# 每個情境三個結果：
#   BLOCKED  commit 被擋住（我們要的）
#   LEAKED   commit 過了，密鑰進 repo（保護失效）
#   WARNED   install 當下就明講不會生效（誠實的失敗，可接受）
# 注意：本腳本目前只在 macOS 驗證過。Windows／Linux 未測。
set -uo pipefail

LEAKBOLT_BIN="${LEAKBOLT_BIN:?請設定 LEAKBOLT_BIN 指向編譯好的 leakbolt}"
FAKE_SECRET='AWS_KEY = "AKIAIMNOJVGFDXXXE4OA"'
PASS=0; FAIL=0
RESULTS=()

new_repo() {
  local d; d="$(mktemp -d)/repo"; mkdir -p "$d"
  git -C "$d" init -q .
  git -C "$d" config user.email t@t
  git -C "$d" config user.name t
  echo hi > "$d/a.txt"
  git -C "$d" add a.txt
  git -C "$d" commit -qm init
  echo "$d"
}

# 回傳 BLOCKED / LEAKED / BLOCKED_BY_OTHER
#
# 只看 commit 有沒有失敗會產生假陽性：husky 預設的 .husky/pre-commit 內容是 npm test，
# 空專案跑起來一定失敗，commit 照樣被擋，但那不是我們擋的、我們的 hook 其實已經失效。
# 所以要在輸出裡找我們自己的指紋，確認是誰擋的。
LEAKBOLT_FINGERPRINT="暫存區掃描完成"

try_leak_commit() {
  local repo="$1" file="${2:-config.py}"
  printf '%s\n' "$FAKE_SECRET" > "$repo/$file"
  git -C "$repo" add "$file"
  local out rc
  out=$(git -C "$repo" commit -m "leak" 2>&1); rc=$?
  if [ "$rc" -eq 0 ]; then
    echo LEAKED
  elif printf '%s' "$out" | grep -q "$LEAKBOLT_FINGERPRINT"; then
    echo BLOCKED
  else
    echo BLOCKED_BY_OTHER
  fi
}

record() {
  local name="$1" want="$2" got="$3" note="${4:-}"
  if [ "$want" = "$got" ]; then
    PASS=$((PASS+1)); RESULTS+=("PASS|$name|期望 $want|實際 $got|$note")
  else
    FAIL=$((FAIL+1)); RESULTS+=("FAIL|$name|期望 $want|實際 $got|$note")
  fi
}

# ---------- 1. 乾淨 repo ----------
r=$(new_repo)
"$LEAKBOLT_BIN" install --local-only >/dev/null 2>&1
( cd "$r" && "$LEAKBOLT_BIN" install --local-only >/dev/null 2>&1 )
record "乾淨 repo，本機安裝" BLOCKED "$(try_leak_commit "$r")"

# ---------- 2. 先裝 husky，再裝 leakbolt ----------
r=$(new_repo)
( cd "$r" && npm init -y >/dev/null 2>&1 && npm pkg set scripts.prepare="husky" >/dev/null 2>&1 \
  && npm install --silent --no-audit --no-fund husky >/dev/null 2>&1 && npx husky init >/dev/null 2>&1 )
if [ -d "$r/.husky/_" ]; then
  ( cd "$r" && "$LEAKBOLT_BIN" install >/dev/null 2>&1 )
  record "先 husky 後 leakbolt" BLOCKED "$(try_leak_commit "$r")" "core.hooksPath=$(git -C "$r" config core.hooksPath)"
else
  record "先 husky 後 leakbolt" BLOCKED SKIP "husky 安裝失敗，跳過"
fi

# ---------- 3. 先裝 leakbolt，再裝 husky（husky 覆寫 core.hooksPath）----------
r=$(new_repo)
( cd "$r" && "$LEAKBOLT_BIN" install --local-only >/dev/null 2>&1 )
( cd "$r" && npm init -y >/dev/null 2>&1 && npm pkg set scripts.prepare="husky" >/dev/null 2>&1 \
  && npm install --silent --no-audit --no-fund husky >/dev/null 2>&1 && npx husky init >/dev/null 2>&1 )
if [ -d "$r/.husky/_" ]; then
  record "先 leakbolt 後 husky（被覆寫）" BLOCKED_BY_OTHER "$(try_leak_commit "$r")" "我們的 hook 已失效；擋住的是 husky 預設的 npm test"
else
  record "先 leakbolt 後 husky（被覆寫）" BLOCKED_BY_OTHER SKIP "husky 安裝失敗，跳過"
fi

# ---------- 3b. 隊友沒裝 leakbolt，不可以把別人的檢查關掉 ----------
# .husky/pre-commit 會進版控、傳給每個隊友。我們的守衛若用 exit 0 離開腳本，
# 沒裝 leakbolt 的隊友會連 npm test / lint-staged 一起被關掉，而且 commit 照樣成功。
r=$(new_repo)
( cd "$r" && npm init -y >/dev/null 2>&1 && npm pkg set scripts.prepare="husky" >/dev/null 2>&1 \
  && npm install --silent --no-audit --no-fund husky >/dev/null 2>&1 && npx husky init >/dev/null 2>&1 )
if [ -d "$r/.husky/_" ]; then
  ( cd "$r" && "$LEAKBOLT_BIN" install >/dev/null 2>&1 )
  # 模擬隊友：把 leakbolt 從 PATH 拿掉，暫存一個乾淨檔案（沒有密鑰）
  echo "clean" > "$r/teammate.txt"
  git -C "$r" add teammate.txt
  saved_path="$PATH"
  export PATH="/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin"
  if command -v leakbolt >/dev/null 2>&1; then
    record "隊友沒裝 leakbolt" BLOCKED_BY_OTHER SKIP "PATH 上仍找得到 leakbolt，無法模擬"
  elif git -C "$r" commit -qm teammate >/dev/null 2>&1; then
    record "隊友沒裝 leakbolt" BLOCKED_BY_OTHER LEAKED "husky 的 npm test 應該要擋下來，卻過了——我們把別人的檢查關掉了"
  else
    record "隊友沒裝 leakbolt" BLOCKED_BY_OTHER BLOCKED_BY_OTHER "husky 的檢查仍然生效"
  fi
  export PATH="$saved_path"
else
  record "隊友沒裝 leakbolt" BLOCKED_BY_OTHER SKIP "husky 安裝失敗，跳過"
fi

# ---------- 4. lefthook ----------
if command -v lefthook >/dev/null 2>&1; then
  r=$(new_repo)
  printf 'pre-commit:\n  commands:\n    noop:\n      run: true\n' > "$r/lefthook.yml"
  ( cd "$r" && lefthook install >/dev/null 2>&1 && "$LEAKBOLT_BIN" install >/dev/null 2>&1 )
  record "lefthook 已 install" BLOCKED "$(try_leak_commit "$r")"

  r=$(new_repo)
  printf 'pre-commit:\n  commands:\n    noop:\n      run: true\n' > "$r/lefthook.yml"
  out=$( cd "$r" && "$LEAKBOLT_BIN" install 2>&1 )
  got=$(try_leak_commit "$r")
  if echo "$out" | grep -q "lefthook install"; then got="WARNED"; fi
  record "lefthook 設定在但沒 install" WARNED "$got" "沒警告就是裝了沒保護"
else
  record "lefthook 已 install" BLOCKED SKIP "未安裝 lefthook"
  record "lefthook 設定在但沒 install" WARNED SKIP "未安裝 lefthook"
fi

# ---------- 5. pre-commit 框架 ----------
if command -v pre-commit >/dev/null 2>&1; then
  r=$(new_repo)
  printf 'repos:\n-   repo: meta\n    hooks:\n    -   id: check-useless-excludes\n' > "$r/.pre-commit-config.yaml"
  ( cd "$r" && pre-commit install >/dev/null 2>&1 && "$LEAKBOLT_BIN" install >/dev/null 2>&1 )
  record "pre-commit 已 install" BLOCKED "$(try_leak_commit "$r")"
else
  record "pre-commit 已 install" BLOCKED SKIP "未安裝 pre-commit"
fi

# ---------- 6. git worktree ----------
r=$(new_repo)
( cd "$r" && "$LEAKBOLT_BIN" install --local-only >/dev/null 2>&1 )
wt="$(mktemp -d)/wt"
git -C "$r" worktree add -q -b wtbranch "$wt" >/dev/null 2>&1
if [ -d "$wt" ]; then
  git -C "$wt" config user.email t@t; git -C "$wt" config user.name t
  record "worktree 裡 commit" BLOCKED "$(try_leak_commit "$wt")" "worktree 共用 .git/hooks"
else
  record "worktree 裡 commit" BLOCKED SKIP "worktree 建立失敗"
fi

# ---------- 7. --no-verify ----------
r=$(new_repo)
( cd "$r" && "$LEAKBOLT_BIN" install --local-only >/dev/null 2>&1 )
printf '%s\n' "$FAKE_SECRET" > "$r/config.py"; git -C "$r" add config.py
if git -C "$r" commit -qm leak --no-verify >/dev/null 2>&1; then got=LEAKED; else got=BLOCKED; fi
record "--no-verify 繞過" LEAKED "$got" "git 的設計，擋不住，只能偵測與回報"

# ---------- 8. 別的工具佔用 core.hooksPath ----------
r=$(new_repo)
mkdir -p "$r/.githooks"; git -C "$r" config core.hooksPath .githooks
out=$( cd "$r" && "$LEAKBOLT_BIN" install 2>&1 )
got=$(try_leak_commit "$r")
if echo "$out" | grep -q "不會覆寫既有 hook 路徑"; then got=WARNED; fi
record "第三方佔用 core.hooksPath" WARNED "$got" "必須拒裝並講清楚"

# ---------- 9. 裝完之後 hooksPath 被改掉 ----------
r=$(new_repo)
( cd "$r" && "$LEAKBOLT_BIN" install --local-only >/dev/null 2>&1 )
mkdir -p "$r/.githooks"; git -C "$r" config core.hooksPath .githooks
record "裝完後 hooksPath 被改" LEAKED "$(try_leak_commit "$r")" "靜默失效，要有健康檢查才抓得到"

# ---------- 輸出 ----------
printf '\n%-34s %-10s %-10s %s\n' "情境" "期望" "實際" "備註"
printf '%s\n' "--------------------------------------------------------------------------------------"
for row in "${RESULTS[@]}"; do
  IFS='|' read -r st name want got note <<< "$row"
  printf '%-4s %-30s %-10s %-10s %s\n' "$st" "$name" "${want#期望 }" "${got#實際 }" "$note"
done
printf '\n通過 %d，失敗 %d\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
