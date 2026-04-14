package llmgateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiscoverFromEnv(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "gsk_test123")
	t.Setenv("DEEPSEEK_API_KEY", "sk-test456")

	opts := DiscoverFromEnv()
	gw := New(opts...)

	assert.GreaterOrEqual(t, gw.ProviderCount(), 2)
	names := gw.ProviderNames()
	assert.Contains(t, names, "Groq")
	assert.Contains(t, names, "Deepseek")
}

func TestDiscoverFromEnv_SkipsExcluded(t *testing.T) {
	t.Setenv("PATREON_API_KEY", "should-skip")
	t.Setenv("WEBHOOK_API_KEY", "should-skip")
	t.Setenv("SNYK_API_KEY", "should-skip")

	opts := DiscoverFromEnv()
	gw := New(opts...)

	for _, name := range gw.ProviderNames() {
		assert.NotEqual(t, "Patreon", name)
		assert.NotEqual(t, "Webhook", name)
		assert.NotEqual(t, "Snyk", name)
	}
}

func TestDiscoverFromEnv_SkipsEmpty(t *testing.T) {
	t.Setenv("EMPTY_API_KEY", "")
	before := New(DiscoverFromEnv()...).ProviderCount()

	t.Setenv("EMPTY_API_KEY", "notempty")
	after := New(DiscoverFromEnv()...).ProviderCount()

	assert.Greater(t, after, before)
}

func TestDiscoverFromEnv_SkipsShellVars(t *testing.T) {
	t.Setenv("SHELLREF_API_KEY", "$SomeOtherVar")

	opts := DiscoverFromEnv()
	gw := New(opts...)

	for _, name := range gw.ProviderNames() {
		assert.NotEqual(t, "Shellref", name)
	}
}

func TestNewFromEnv(t *testing.T) {
	t.Setenv("TESTPROVIDER_API_KEY", "test-key-123")

	gw := NewFromEnv(WithMaxRetry(5))
	assert.GreaterOrEqual(t, gw.ProviderCount(), 1)
	assert.Equal(t, 5, gw.maxRetry)
}

func TestDeriveName(t *testing.T) {
	tests := []struct {
		prefix string
		want   string
	}{
		{"OPENROUTER", "Openrouter"},
		{"GITHUB_MODELS", "GithubModels"},
		{"DEEPSEEK", "Deepseek"},
		{"NLP", "Nlp"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, deriveName(tt.prefix))
	}
}

func TestDeriveBaseURL_Known(t *testing.T) {
	assert.Equal(t, "https://openrouter.ai/api/v1", deriveBaseURL("OPENROUTER"))
	assert.Equal(t, "https://api.groq.com/openai/v1", deriveBaseURL("GROQ"))
	assert.Equal(t, "https://api.deepseek.com/v1", deriveBaseURL("DEEPSEEK"))
}

func TestDeriveBaseURL_Unknown(t *testing.T) {
	url := deriveBaseURL("MYPROVIDER")
	assert.Equal(t, "https://api.myprovider.com/v1", url)
}

func TestIsExcluded(t *testing.T) {
	assert.True(t, isExcluded("PATREON_API_KEY"))
	assert.True(t, isExcluded("WEBHOOK_API_KEY"))
	assert.True(t, isExcluded("ADMIN_API_KEY"))
	assert.True(t, isExcluded("SNYK_API_KEY"))
	assert.True(t, isExcluded("LLMSVERIFIER_API_KEY"))
	assert.True(t, isExcluded("GITHUB_TOKEN"))
	assert.False(t, isExcluded("OPENROUTER_API_KEY"))
	assert.False(t, isExcluded("GROQ_API_KEY"))
}
