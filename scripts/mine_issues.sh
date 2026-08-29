#!/usr/bin/env bash
# 民怨排行榜：找星數中等 (500~8000)、已停更 (>6 個月無 commit) 的 repo，挖出 👍 最多的開放 issue
set -uo pipefail

STALE="${STALE:-2026-02-28}"       # 這個日期之前沒有 commit 的才算停更（> 6 個月）
MIN_UP="${MIN_UP:-15}"             # 少於這個 👍 數的 issue 丟掉
REPOS_PER_TOPIC="${REPOS_PER_TOPIC:-10}" # 每個 topic 抓取候選 repo 數
SLEEP_SEC="${SLEEP_SEC:-3}"        # 每次搜尋間隔秒數（GitHub 搜尋上限 30 次/分）
OUT="${1:-/tmp/mined_issues.tsv}"  # 輸出 TSV 路徑

# 涵蓋至少 20 個 topic（涵蓋開發工具、自架服務、媒體處理、資料、網路、生產力、瀏覽器擴充、行動開發、桌面應用、機器學習工具等）
TOPICS=(
  cli
  self-hosted
  developer-tools
  automation
  scraper
  media
  video
  audio
  database
  networking
  productivity
  browser-extension
  chrome-extension
  firefox-addon
  desktop
  electron
  mobile
  android
  ios
  machine-learning
  deep-learning
  nlp
  ocr
  image-processing
  terminal
  markdown
  devops
  monitoring
  security
  pdf
)

check_rate_limit() {
  local search_info
  search_info=$(gh api rate_limit --jq '.resources.search | "\(.remaining) \(.reset)"' 2>/dev/null || echo "30 0")
  local remaining reset
  remaining=$(echo "$search_info" | awk '{print $1}')
  reset=$(echo "$search_info" | awk '{print $2}')
  
  if [ -n "$remaining" ] && [ "$remaining" -lt 3 ]; then
    local now
    now=$(date +%s)
    local sleep_time=$(( reset - now + 3 ))
    if [ "$sleep_time" -gt 0 ] && [ "$sleep_time" -le 120 ]; then
      echo "[RATE LIMIT] 搜尋額度剩餘 ${remaining}，暫停 ${sleep_time} 秒至重置..." >&2
      sleep "$sleep_time"
    else
      echo "[RATE LIMIT] 搜尋額度剩餘 ${remaining}，暫停 60 秒..." >&2
      sleep 60
    fi
  fi
}

api_call_with_retry() {
  local max_retries=5
  local count=0
  local output=""
  local exit_code=0

  while [ "$count" -lt "$max_retries" ]; do
    check_rate_limit
    output=$("$@" 2>&1)
    exit_code=$?
    
    if [ $exit_code -eq 0 ]; then
      echo "$output"
      return 0
    fi

    # 檢查是否為 403、secondary rate limit 或 abuse detection
    if echo "$output" | grep -qiE "403|rate limit|secondary rate limit|abuse"; then
      count=$(( count + 1 ))
      echo "[WARN] 觸發 GitHub 403 / Rate limit（第 ${count}/${max_retries} 次），休眠 120 秒後重試..." >&2
      sleep 120
    else
      # 其他錯誤重試一次後跳過
      count=$(( count + 1 ))
      if [ "$count" -ge "$max_retries" ]; then
        echo "[ERROR] API 呼叫失敗（已達最大重試次數）：$output" >&2
        return $exit_code
      fi
      sleep "$SLEEP_SEC"
    fi
  done
  return 1
}

echo "=== 步驟 1：建立候選 Repository 清單（星數 500..8000、pushed < ${STALE}）===" >&2
: > "${OUT}.repos"

for t in "${TOPICS[@]}"; do
  echo "正在檢索主題：$t ..." >&2
  res=$(api_call_with_retry gh search repos --topic "$t" --stars 500..8000 "pushed:<${STALE}" --limit "$REPOS_PER_TOPIC" \
    --json fullName,stargazersCount,pushedAt \
    --jq ".[] | \"\(.fullName)\t\(.stargazersCount)\t\(.pushedAt[0:10])\"")
  
  if [ -n "$res" ]; then
    echo "$res" >> "${OUT}.repos"
  fi
  sleep "$SLEEP_SEC"
done

sort -u "${OUT}.repos" -o "${OUT}.repos"
total_repos=$(wc -l < "${OUT}.repos" | tr -d ' ')
echo "候選集建立完成，共取得 ${total_repos} 個獨立 Repository" >&2

echo "=== 步驟 2：掃描開放 Issue 並篩選 👍 >= ${MIN_UP} ===" >&2
: > "$OUT"
: > "${OUT}.jsonl"

curr=0
while IFS=$'\t' read -r repo stars pushed; do
  [ -z "$repo" ] && continue
  curr=$(( curr + 1 ))
  echo "[${curr}/${total_repos}] 正在掃描：${repo} (星數: ${stars}, 最後更新: ${pushed}) ..." >&2
  
  raw_json=$(api_call_with_retry gh api -X GET search/issues \
    -f q="repo:${repo} is:issue is:open" -f sort=reactions -f order=desc -f per_page=3)
  
  if [ -n "$raw_json" ]; then
    # 紀錄 raw JSON 供詳細分析與分類檢驗
    echo "$raw_json" | jq -c --arg repo "$repo" --arg stars "$stars" --arg pushed "$pushed" \
      '.items[]? | {repo: $repo, stars: ($stars|tonumber), pushed: $pushed, title: .title, url: .html_url, thumbs_up: (.reactions["+1"] // 0), comments: .comments, body: .body, labels: [.labels[]?.name], created_at: .created_at}' \
      >> "${OUT}.jsonl" 2>/dev/null || true

    # 輸出符合 👍 門檻之 TSV
    echo "$raw_json" | jq -r --arg repo "$repo" --arg stars "$stars" --arg pushed "$pushed" --arg min_up "$MIN_UP" \
      '.items[]? | select((.reactions["+1"] // 0) >= ($min_up|tonumber)) | "\(.reactions["+1"] // 0)\t\(.comments)\t\($repo)\t\($stars)\t\($pushed)\t\(.title)\t\(.html_url)"' \
      >> "$OUT" 2>/dev/null || true
  fi

  sleep "$SLEEP_SEC"
done < "${OUT}.repos"

sort -rn "$OUT" -o "$OUT"
total_issues=$(wc -l < "$OUT" | tr -d ' ')
echo "掃描完成！實際掃描 repo 數：${total_repos}，符合 👍 >= ${MIN_UP} 留下 issue 數：${total_issues}" >&2
