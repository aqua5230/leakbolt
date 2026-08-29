#!/bin/sh
# 偵測品質閘門。上限的由來與收緊紀錄：
#
# 2026-08-29 初版：漏報率 0%、誤報率 10%（語料庫還只有預設規則時定的）
# 2026-08-29 收緊：漏報率 0%、誤報率 0%
#   理由：接上補充規則後實測就是 0%／0%（19 真陽性、21 真陰性）。
#   PLAN.md 判準表把誤報率列為一票否決，留 10% 的空間等於允許無聲退步。
#   之後若新增樣本合理地觸發誤報，正確做法是修規則或明確改 manifest，不是放寬這個上限。
#   任何放寬都要在 PLAN.md 留紀錄。

set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
report=$(mktemp "${TMPDIR:-/tmp}/leakbolt-quality-gate.XXXXXX")
trap 'rm -f "$report"' EXIT HUP INT TERM
GOCACHE=${GOCACHE:-"${TMPDIR:-/tmp}/leakbolt-go-cache"}
export GOCACHE

cd "$root/prototype"

# corpusbench 是獨立的 main 套件，內嵌自己的一份規則。先確認兩份一致；
# 分岔時直接失敗，避免量到的不是產品實際行為。
if ! cmp -s rules/leakbolt.toml cmd/corpusbench/rules/leakbolt.toml; then
	echo "quality gate 失敗：補充規則不一致；請執行：cp prototype/rules/leakbolt.toml prototype/cmd/corpusbench/rules/leakbolt.toml" >&2
	exit 2
fi

if ! go run ./cmd/corpusbench >"$report"; then
	cat "$report"
	exit 2
fi
cat "$report"

miss_rate=$(sed -n 's/^CORPUSBENCH_MISS_RATE=//p' "$report")
false_positive_rate=$(sed -n 's/^CORPUSBENCH_FALSE_POSITIVE_RATE=//p' "$report")
if [ -z "$miss_rate" ] || [ -z "$false_positive_rate" ]; then
	echo "quality gate 無法讀取 corpusbench 指標" >&2
	exit 2
fi

if awk -v miss="$miss_rate" -v fp="$false_positive_rate" 'BEGIN { exit !(miss <= 0 && fp <= 0) }'; then
	echo "quality gate 通過：漏報率 ${miss_rate}，誤報率 ${false_positive_rate}"
	exit 0
fi

echo "quality gate 失敗：漏報率上限 0.000000，誤報率上限 0.000000" >&2
exit 1
