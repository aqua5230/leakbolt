package main

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

const (
	acknowledgeDialogButton = "知道了"
	allowDialogButton       = "標為誤報並放行"
)

var (
	nativeDialogsSupported  = platformNativeDialogsSupported
	showBlockedCommitDialog = platformShowBlockedCommitDialog
)

func notifyBlockedCommit(repo string, findings []stagedFinding, stdout io.Writer) {
	if len(findings) == 0 || !nativeDialogsSupported() {
		return
	}
	if os.Getenv("CI") != "" || os.Getenv("LEAKBOLT_NO_GUI") != "" {
		return
	}
	gui, set, err := gitConfigBool(repo, "leakbolt.gui")
	if err != nil || (set && gui == "false") {
		return
	}

	button, err := showBlockedCommitDialog(findings)
	if err != nil || len(findings) != 1 || strings.TrimSpace(button) != allowDialogButton {
		return
	}
	entry, alreadyAllowed, err := allowRecentFingerprint(repo, findings[0].Fingerprint)
	if err != nil {
		return
	}
	if alreadyAllowed {
		fmt.Fprintf(stdout, "指紋 %s 已在 allowlist；請重新執行 commit。\n", shortFingerprint(entry.Fingerprint))
		return
	}
	fmt.Fprintf(stdout, "已將指紋 %s（規則 %s）標記為誤報；請重新執行 commit。\n", shortFingerprint(entry.Fingerprint), entry.RuleID)
}

func blockedCommitDialogScript(findings []stagedFinding) string {
	message := blockedCommitDialogMessage(findings)
	buttons := appleScriptStringLiteral(acknowledgeDialogButton)
	if len(findings) == 1 {
		buttons = appleScriptStringLiteral(allowDialogButton) + ", " + buttons
	}
	return "set dialogResult to display dialog " + appleScriptStringLiteral(message) +
		" with title " + appleScriptStringLiteral("LeakBolt") +
		" buttons {" + buttons + "} default button " + appleScriptStringLiteral(acknowledgeDialogButton) +
		" with icon caution giving up after 120\n" +
		"if gave up of dialogResult then return " + appleScriptStringLiteral(acknowledgeDialogButton) + "\n" +
		"return button returned of dialogResult"
}

func blockedCommitDialogMessage(findings []stagedFinding) string {
	copyFindings := append([]stagedFinding(nil), findings...)
	sort.Slice(copyFindings, func(i, j int) bool {
		if copyFindings[i].File != copyFindings[j].File {
			return copyFindings[i].File < copyFindings[j].File
		}
		return copyFindings[i].RuleID < copyFindings[j].RuleID
	})

	var message strings.Builder
	message.WriteString("commit 已中止：偵測到可能的金鑰。")
	limit := len(copyFindings)
	if limit > 5 {
		limit = 5
	}
	for i := 0; i < limit; i++ {
		finding := copyFindings[i]
		file := finding.File
		if file == "" {
			file = "（無檔案資訊）"
		}
		rule := finding.RuleID
		if rule == "" {
			rule = "（無規則 ID）"
		}
		fmt.Fprintf(&message, "\n\n檔案：%s\n規則：%s\n指紋：%s", file, rule, shortFingerprint(finding.Fingerprint))
	}
	if remaining := len(copyFindings) - limit; remaining > 0 {
		fmt.Fprintf(&message, "\n\n還有 %d 筆", remaining)
	}
	return message.String()
}

func appleScriptStringLiteral(value string) string {
	var literal strings.Builder
	literal.Grow(len(value) + 2)
	literal.WriteByte('"')
	for _, character := range value {
		switch character {
		case '\\':
			literal.WriteString("\\\\")
		case '"':
			literal.WriteString("\\\"")
		case '\n':
			literal.WriteString("\\n")
		case '\r':
			literal.WriteString("\\r")
		case '\t':
			literal.WriteString("\\t")
		default:
			literal.WriteRune(character)
		}
	}
	literal.WriteByte('"')
	return literal.String()
}
