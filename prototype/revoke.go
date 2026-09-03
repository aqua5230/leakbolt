package main

// Provider 描述一個會花錢的金鑰供應商：去哪裡撤銷、去哪裡設支出上限。
//
// 只收「撤銷頁面＋支出上限頁面都有穩定固定網址」的供應商。查不到穩定頁面的規則
// 寧可不收進表裡，讓 runGuide 印出「找不到已知的處理指引」，也不要塞一個可能過時
// 的網址——這是引導使用者去做正確的事，不是幫使用者按按鈕，網址錯了使用者自己會發現，
// 但錯誤的自信（宣稱有指引其實是舊的）比誠實說不知道更危險。
type Provider struct {
	Name         string
	RevokeURL    string
	SpendCapNote string
}

var providersByRuleID = map[string]Provider{
	"openai-api-key": {
		Name:         "OpenAI",
		RevokeURL:    "https://platform.openai.com/api-keys",
		SpendCapNote: "在 Settings → Limits 設定每月支出上限：https://platform.openai.com/settings/organization/limits",
	},
	"anthropic-api-key": {
		Name:         "Anthropic",
		RevokeURL:    "https://console.anthropic.com/settings/keys",
		SpendCapNote: "在 Settings → Billing 設定用量上限：https://console.anthropic.com/settings/billing",
	},
	"anthropic-admin-api-key": {
		Name:         "Anthropic",
		RevokeURL:    "https://console.anthropic.com/settings/keys",
		SpendCapNote: "在 Settings → Billing 設定用量上限：https://console.anthropic.com/settings/billing",
	},
	"aws-access-token": {
		Name:         "AWS",
		RevokeURL:    "https://console.aws.amazon.com/iam/home#/security_credentials",
		SpendCapNote: "AWS 沒有硬性支出上限，只能設提醒：在 Billing → Budgets 建立預算警示：https://console.aws.amazon.com/billing/home#/budgets。若權限範圍不明，優先直接停用或刪除該組 access key。",
	},
	"stripe-access-token": {
		Name:         "Stripe",
		RevokeURL:    "https://dashboard.stripe.com/apikeys",
		SpendCapNote: "Stripe 沒有自助支出上限；金鑰外洩請立即撤銷，並檢查 Radar 規則與近期交易：https://dashboard.stripe.com/radar/rules",
	},
	"leakbolt-groq-api-key": {
		Name:         "Groq",
		RevokeURL:    "https://console.groq.com/keys",
		SpendCapNote: "Groq 目前無自助支出上限，請直接撤銷金鑰。",
	},
	"leakbolt-replicate-token": {
		Name:         "Replicate",
		RevokeURL:    "https://replicate.com/account/api-tokens",
		SpendCapNote: "在 Billing 設定用量提醒：https://replicate.com/account/billing",
	},
	"leakbolt-openrouter-key": {
		Name:         "OpenRouter",
		RevokeURL:    "https://openrouter.ai/keys",
		SpendCapNote: "在 Settings → Credits 設定每月限額：https://openrouter.ai/settings/credits",
	},
	"leakbolt-xai-key": {
		Name:         "xAI",
		RevokeURL:    "https://console.x.ai",
		SpendCapNote: "在 console 的 Billing 頁面設定用量上限。",
	},
	"leakbolt-fireworks-key": {
		Name:         "Fireworks AI",
		RevokeURL:    "https://app.fireworks.ai/settings/users/api-keys",
		SpendCapNote: "在 Billing 設定用量上限：https://app.fireworks.ai/settings/billing",
	},
	"leakbolt-deepseek-key": {
		Name:         "DeepSeek",
		RevokeURL:    "https://platform.deepseek.com/api_keys",
		SpendCapNote: "在 Usage 頁面查看與設定額度：https://platform.deepseek.com/usage",
	},
	"leakbolt-supabase-secret-key": {
		Name:         "Supabase",
		RevokeURL:    "https://supabase.com/dashboard/project/_/settings/api",
		SpendCapNote: "用量上限在 Organization → Billing 設定：https://supabase.com/dashboard/org/_/billing",
	},
	"leakbolt-supabase-pat": {
		Name:         "Supabase",
		RevokeURL:    "https://supabase.com/dashboard/account/tokens",
		SpendCapNote: "用量上限在 Organization → Billing 設定：https://supabase.com/dashboard/org/_/billing",
	},
	"leakbolt-clerk-secret-key": {
		Name:         "Clerk",
		RevokeURL:    "https://dashboard.clerk.com/",
		SpendCapNote: "Clerk 依月活躍使用者計費，用量在 Billing 頁面查看，無單一硬上限開關。",
	},
}

func providerForRuleID(ruleID string) (Provider, bool) {
	provider, ok := providersByRuleID[ruleID]
	return provider, ok
}
