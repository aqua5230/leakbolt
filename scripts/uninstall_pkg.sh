#!/bin/sh
set -u

if [ "$(/usr/bin/id -u)" -ne 0 ]; then
	printf 'STOPPED: 原因＝請用 sudo 執行這個解除安裝腳本\n'
	exit 1
fi

warning=""
console_user=$(/usr/bin/stat -f '%Su' /dev/console 2>/dev/null || true)

case "$console_user" in
	""|root|loginwindow|_mbsetupuser)
		warning="找不到目前登入的使用者，未能移除其 Git 全域設定"
		;;
	*)
		if [ -x /usr/local/bin/leakbolt ]; then
			console_uid=$(/usr/bin/id -u "$console_user" 2>/dev/null || true)
			console_home=$(/usr/bin/dscl . -read "/Users/$console_user" NFSHomeDirectory 2>/dev/null | /usr/bin/sed 's/^NFSHomeDirectory: //' || true)
			if [ -n "$console_uid" ] && [ -n "$console_home" ]; then
				if ! /bin/launchctl asuser "$console_uid" /usr/bin/sudo -u "$console_user" \
					/usr/bin/env HOME="$console_home" USER="$console_user" LOGNAME="$console_user" \
					PATH="/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin" \
					/usr/local/bin/leakbolt uninstall --global >/dev/null 2>&1; then
					warning="未能移除 $console_user 的 Git 全域設定"
				fi
			else
				warning="無法取得 $console_user 的帳號資料，未能移除其 Git 全域設定"
			fi
		fi
		;;
esac

if ! /bin/rm -f /usr/local/bin/leakbolt; then
	printf 'STOPPED: 原因＝無法移除 /usr/local/bin/leakbolt\n'
	exit 1
fi
if ! /bin/rm -rf /usr/local/leakbolt; then
	printf 'STOPPED: 原因＝無法移除 /usr/local/leakbolt/\n'
	exit 1
fi

if [ -n "$warning" ]; then
	printf 'DONE: %s\n' "$warning"
else
	printf 'DONE\n'
fi
