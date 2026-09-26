package shellconfig

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/providerpresets"
	"github.com/stretchr/testify/require"
)

// runBuiltin calls a config builtin with a fresh builder and captures its
// output. loadScript cannot be used for anything that prints: it runs the
// script through shell.Run with no Stdout, so a top-level builtin's output
// goes nowhere. (Output captured inside a command substitution is a
// different story — the interpreter supplies its own pipe there.)
func runBuiltin(t *testing.T, args ...string) (*ConfigBuilder, string, string) {
	t.Helper()

	b := newConfigBuilder()
	var stdout, stderr bytes.Buffer
	err := handleProvider(withConfigBuilder(t.Context(), b), args, nil, &stdout, &stderr)
	require.NoError(t, err)
	return b, stdout.String(), stderr.String()
}

func TestProviderPresetList(t *testing.T) {
	t.Parallel()

	_, stdout, _ := runBuiltin(t, "provider", "preset")
	for _, p := range providerpresets.List() {
		require.Contains(t, stdout, p.ID)
		require.Contains(t, stdout, p.Note)
	}
}

func TestProviderPresetListSubcommand(t *testing.T) {
	t.Parallel()

	_, stdout, _ := runBuiltin(t, "provider", "preset", "list")
	require.Contains(t, stdout, "pollinations")
}

func TestProviderPresetListWritesNoConfig(t *testing.T) {
	t.Parallel()

	b, _, _ := runBuiltin(t, "provider", "preset")
	require.True(t, b.empty(), "listing must not write config")
}

func TestProviderPresetAppliesCatalog(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider preset pollinations`)

	p := result["providers"].(map[string]any)["pollinations"].(map[string]any)
	require.Equal(t, "Pollinations", p["name"])
	require.Equal(t, "openai-compat", p["type"])
	require.Equal(t, "https://gen.pollinations.ai/v1", p["base_url"])
	require.Equal(t, "$POLLINATIONS_API_KEY", p["api_key"])
	require.Equal(t, true, p["flat_rate"])
}

func TestProviderPresetKeyedProvider(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider preset logfare`)

	p := result["providers"].(map[string]any)["logfare"].(map[string]any)
	require.Equal(t, "$LOGFARE_API_KEY", p["api_key"])
}

func TestProviderPresetSeedsModels(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider preset crax`)

	p := result["providers"].(map[string]any)["crax"].(map[string]any)
	require.NotEmpty(t, p["models"])
}

func TestProviderPresetNoSeedModels(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider preset crax --no-seed-models`)

	p := result["providers"].(map[string]any)["crax"].(map[string]any)
	require.NotContains(t, p, "models")
	require.Contains(t, p, "base_url", "dropping seeds must not drop the provider")
}

// TestProviderPresetNoSeedModelsNotLeaked guards the scratch key: it is a
// directive, and must not survive into the provider entry.
func TestProviderPresetNoSeedModelsNotLeaked(t *testing.T) {
	t.Parallel()

	for _, script := range []string{
		`provider preset crax --no-seed-models`,
		`provider preset crax`,
		`provider preset nvidia`,
	} {
		result := loadScript(t, script)
		for _, raw := range result["providers"].(map[string]any) {
			require.NotContains(t, raw.(map[string]any), "no_seed_models")
		}
	}
}

func TestProviderPresetFlagsOverrideCatalog(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider preset pollinations \
  --name "My Pollinations" \
  --base-url "http://localhost:8080/v1" \
  --api-key "literal-key" \
  --discover-models false \
  --extra-header X-Trace on`)

	p := result["providers"].(map[string]any)["pollinations"].(map[string]any)
	require.Equal(t, "My Pollinations", p["name"])
	require.Equal(t, "http://localhost:8080/v1", p["base_url"])
	require.Equal(t, "literal-key", p["api_key"])
	require.Equal(t, false, p["discover_models"])
	require.Equal(t, map[string]any{"X-Trace": "on"}, p["extra_headers"])
}

func TestProviderPresetAlias(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider preset gpt.crax.lol`)

	require.Contains(t, result["providers"].(map[string]any), "crax",
		"an alias must register under the preset's own id")
}

func TestProviderPresetUnknown(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "crushrc")
	_, err := LoadShellConfig(t.Context(), path, []byte(`provider preset nope`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown preset")
	require.Contains(t, err.Error(), "pollinations", "the error should list what is available")
}

func TestProviderPresetUnknownFlag(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "crushrc")
	_, err := LoadShellConfig(t.Context(), path, []byte(`provider preset nvidia --nope`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown flag")
}

// TestProviderPresetKeepsExistingConfig pins the preset-as-default rule: a
// preset fills gaps, it never overwrites what the script already set. Without
// this, re-running `provider preset crax` would silently drop every model the
// user registered on it.
func TestProviderPresetKeepsExistingConfig(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider add crax --api-key "my-own-key"
model add crax/my-own-model --name "Mine"
provider preset crax`)

	p := result["providers"].(map[string]any)["crax"].(map[string]any)
	require.Equal(t, "my-own-key", p["api_key"],
		"an existing api_key must survive re-applying the preset")
	require.Len(t, p["models"], 1, "registered models must survive re-applying the preset")
	require.Equal(t, "https://gpt.crax.lol/v1", p["base_url"],
		"unset fields should still be filled in")
}

func TestProviderPresetFillsMissingFields(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider add pollinations --name "Renamed"
provider preset pollinations`)

	p := result["providers"].(map[string]any)["pollinations"].(map[string]any)
	require.Equal(t, "Renamed", p["name"], "an existing name must survive")
	require.Equal(t, "https://gen.pollinations.ai/v1", p["base_url"],
		"a missing base_url should be filled in")
	require.Equal(t, "$POLLINATIONS_API_KEY", p["api_key"],
		"a missing api_key should be filled in")
}

func TestProviderPresetThenRemove(t *testing.T) {
	t.Parallel()

	result := loadScript(t, `provider preset pollinations
provider remove pollinations`)

	require.NotContains(t, result["providers"].(map[string]any), "pollinations")
}

// TestProviderPresetAppliesEveryCatalogEntry keeps the builtin and the
// catalog from drifting: a preset that the builtin cannot turn into a
// provider entry is a broken feature, and this is the only place that would
// notice.
func TestProviderPresetAppliesEveryCatalogEntry(t *testing.T) {
	t.Parallel()

	for _, preset := range providerpresets.List() {
		t.Run(preset.ID, func(t *testing.T) {
			t.Parallel()

			result := loadScript(t, "provider preset "+preset.ID)

			p, ok := result["providers"].(map[string]any)[preset.ID].(map[string]any)
			require.True(t, ok, "preset must register under its own id")
			require.Equal(t, preset.Name, p["name"])
			require.Equal(t, preset.BaseURL, p["base_url"])
			require.Equal(t, preset.Type, p["type"])
			require.NotContains(t, p, noSeedModelsKey)
		})
	}
}

func TestProviderUnknownSubcommand(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "crushrc")
	_, err := LoadShellConfig(t.Context(), path, []byte(`provider bogus`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown subcommand")
}
