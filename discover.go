package llmgateway

import (
	"os"
	"strings"
)

// KnownProviders maps environment variable prefixes to their base URLs.
// Any *_API_KEY environment variable whose prefix is found here will be
// auto-registered. Unknown providers get a best-effort URL derived from
// the env var name.
var KnownProviders = map[string]string{
	"OPENROUTER":      "https://openrouter.ai/api/v1",
	"GROQ":            "https://api.groq.com/openai/v1",
	"DEEPSEEK":        "https://api.deepseek.com/v1",
	"GEMINI":          "https://generativelanguage.googleapis.com/v1beta",
	"CEREBRAS":        "https://api.cerebras.ai/v1",
	"HUGGINGFACE":     "https://api-inference.huggingface.co",
	"NVIDIA":          "https://integrate.api.nvidia.com/v1",
	"MISTRAL":         "https://api.mistral.ai/v1",
	"CODESTRAL":       "https://codestral.mistral.ai/v1",
	"COHERE":          "https://api.cohere.ai/v2",
	"KIMI":            "https://api.moonshot.cn/v1",
	"SILICONFLOW":     "https://api.siliconflow.cn/v1",
	"FIREWORKS":       "https://api.fireworks.ai/inference/v1",
	"REPLICATE":       "https://api.replicate.com/v1",
	"SAMBANOVA":       "https://api.sambanova.ai/v1",
	"HYPERBOLIC":      "https://api.hyperbolic.xyz/v1",
	"NOVITA":          "https://api.novita.ai/v3",
	"UPSTAGE":         "https://api.upstage.ai/v1",
	"CLOUDFLARE":      "https://api.cloudflare.com/client/v4",
	"CHUTES":          "https://api.chutes.ai/v1",
	"GITHUB_MODELS":   "https://models.inference.ai.azure.com",
	"VENICE":          "https://api.venice.ai/api/v1",
	"ZAI":             "https://open.bigmodel.cn/api/paas/v4",
	"ZHIPU":           "https://open.bigmodel.cn/api/paas/v4",
	"OPENAI":          "https://api.openai.com/v1",
	"ANTHROPIC":       "https://api.anthropic.com/v1",
	"INFERENCE":       "https://api.inference.net/v1",
	"NLP":             "https://api.nlpcloud.io/v1",
	"PUBLICAI":        "https://api.publicai.io/v1",
	"SARVAM":          "https://api.sarvam.ai/v1",
	"BASETEN":         "https://api.baseten.co/v1",
	"VERCEL":          "https://api.vercel.ai/v1",
}

// DiscoverFromEnv scans environment variables for *_API_KEY patterns
// and returns Gateway Options that register each discovered provider.
// Non-LLM keys (PATREON_*, WEBHOOK_*, etc.) are excluded.
//
// This matches the HelixAgent env_scanner.go auto-discovery pattern:
// any environment variable ending in _API_KEY with a non-empty value
// is treated as an LLM provider.
func DiscoverFromEnv() []Option {
	var opts []Option

	for _, entry := range os.Environ() {
		idx := strings.IndexByte(entry, '=')
		if idx < 0 {
			continue
		}
		key := entry[:idx]
		val := entry[idx+1:]

		if !strings.HasSuffix(key, "_API_KEY") || val == "" {
			continue
		}
		if isExcluded(key) {
			continue
		}
		// Skip unexpanded shell variables
		if strings.HasPrefix(val, "$") {
			continue
		}

		prefix := strings.TrimSuffix(key, "_API_KEY")
		name := deriveName(prefix)
		baseURL := deriveBaseURL(prefix)

		opts = append(opts, WithProvider(name, baseURL, val))
	}

	return opts
}

// NewFromEnv creates a Gateway pre-populated with all providers
// discovered from environment variables.
func NewFromEnv(extraOpts ...Option) *Gateway {
	discovered := DiscoverFromEnv()
	all := append(discovered, extraOpts...)
	return New(all...)
}

func isExcluded(key string) bool {
	excluded := []string{
		"PATREON_", "WEBHOOK_", "ADMIN_", "SNYK_", "SONAR_",
		"SEMGREP_", "TAVILY_", "LLMSVERIFIER_", "GITHUB_TOKEN",
		"GITLAB_TOKEN", "GITFLIC_TOKEN", "GITVERSE_TOKEN",
	}
	for _, prefix := range excluded {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func deriveName(prefix string) string {
	parts := strings.Split(strings.ToLower(prefix), "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

func deriveBaseURL(prefix string) string {
	if url, ok := KnownProviders[prefix]; ok {
		return url
	}
	return "https://api." + strings.ToLower(prefix) + ".com/v1"
}
