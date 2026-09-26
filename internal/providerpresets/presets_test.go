package providerpresets

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListIsNonEmpty(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, List())
}

func TestListIsSortedAndUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, len(presets))
	var prev string
	for _, p := range presets {
		require.NotContains(t, seen, p.ID, "duplicate preset id %q", p.ID)
		seen[p.ID] = struct{}{}
		if prev != "" {
			require.Less(t, prev, p.ID, "presets must be sorted by ID")
		}
		prev = p.ID
	}
}

func TestNamesMatchList(t *testing.T) {
	t.Parallel()

	require.Equal(t, Names(), func() []string {
		var ids []string
		for _, p := range List() {
			ids = append(ids, p.ID)
		}
		return ids
	}())
}

// TestPresetsAreWellFormed guards the catalog against the mistakes that
// would otherwise only show up as a broken request at runtime: a preset with
// no endpoint, an unparseable URL, a model with no ID, or a free provider
// that quietly accumulates cost.
func TestPresetsAreWellFormed(t *testing.T) {
	t.Parallel()

	for _, p := range presets {
		t.Run(p.ID, func(t *testing.T) {
			t.Parallel()

			require.NotEmpty(t, p.Name, "preset needs a display name")
			require.NotEmpty(t, p.Type, "preset needs a provider type")
			require.NotEmpty(t, p.HomeURL, "preset needs somewhere to get a key")
			require.NotEmpty(t, p.Note, "preset must explain what it does with requests")

			u, err := url.Parse(p.BaseURL)
			require.NoError(t, err, "base_url must parse")
			require.Equal(t, "https", u.Scheme, "base_url must be https")
			require.NotEmpty(t, u.Host, "base_url needs a host")

			if p.APIKeyEnv != "" {
				require.Equal(t, "$"+p.APIKeyEnv, p.Config()["api_key"],
					"api_key must reference the preset's env var")
			} else {
				require.NotContains(t, p.Config(), "api_key",
					"a keyless preset must not write an api_key")
			}

			// Every preset in this catalog is free, so cost accounting has
			// to be off or `crush stats` reports money the user never
			// spent.
			require.True(t, p.FlatRate, "free preset must set flat_rate")

			ids := make(map[string]struct{}, len(p.Models))
			for _, m := range p.Models {
				require.NotEmpty(t, m.ID, "seed model needs an id")
				require.NotContains(t, ids, m.ID, "duplicate seed model %q", m.ID)
				ids[m.ID] = struct{}{}
				require.NotEmpty(t, m.Name, "seed model needs a display name")
				require.Positive(t, m.ContextWindow, "seed model needs a context window")
			}
		})
	}
}

func TestLookupByIDAndAlias(t *testing.T) {
	t.Parallel()

	for _, p := range presets {
		got, ok := Lookup(p.ID)
		require.True(t, ok, "lookup by id %q", p.ID)
		require.Equal(t, p.ID, got.ID)

		// Lookup is what a user types, so it should not be case- or
		// whitespace-sensitive.
		got, ok = Lookup("  " + upper(p.ID) + " ")
		require.True(t, ok, "lookup by id %q should ignore case and padding", p.ID)
		require.Equal(t, p.ID, got.ID)

		for _, alias := range p.Aliases {
			got, ok = Lookup(alias)
			require.True(t, ok, "lookup by alias %q", alias)
			require.Equal(t, p.ID, got.ID)
		}
	}
}

func TestLookupMiss(t *testing.T) {
	t.Parallel()

	_, ok := Lookup("definitely-not-a-provider")
	require.False(t, ok)

	_, ok = Lookup("")
	require.False(t, ok)
}

// TestLookupDoesNotShadowIDs guards the two-pass lookup: an alias may not
// claim another preset's ID, or the match would depend on catalog order.
func TestLookupDoesNotShadowIDs(t *testing.T) {
	t.Parallel()

	ids := make(map[string]struct{}, len(presets))
	for _, p := range presets {
		ids[p.ID] = struct{}{}
	}
	for _, p := range presets {
		for _, alias := range p.Aliases {
			require.NotContains(t, ids, alias,
				"alias %q collides with a preset id", alias)
		}
	}
}

func TestConfigShape(t *testing.T) {
	t.Parallel()

	c := Lookup2(t, "crax")
	require.Equal(t, "crax-gpt", c["name"])
	require.Equal(t, "openai-compat", c["type"])
	require.Equal(t, "https://gpt.crax.lol/v1", c["base_url"])
	require.Equal(t, "$CRAX_API_KEY", c["api_key"])
	require.Equal(t, true, c["flat_rate"])
	require.NotEmpty(t, c["models"])

	// Discovery is on by default, so the preset must not pin it.
	require.NotContains(t, c, "discover_models")

	models := c["models"].([]any)
	first := models[0].(map[string]any)
	require.Equal(t, "gpt-6-luna", first["id"])
	require.Contains(t, first, "context_window")
}

func TestConfigHonorsDisabledDiscovery(t *testing.T) {
	t.Parallel()

	c := Lookup2(t, "github-models")
	require.Equal(t, false, c["discover_models"])
	require.Equal(t, "$GITHUB_TOKEN", c["api_key"])
	require.NotContains(t, c, "models", "a preset with no seed list writes no models key")
}

// TestKeyHint covers both hint forms. No catalog preset is keyless today, so
// the keyless branch is exercised with a synthetic preset rather than by
// pinning a real one to it.
func TestKeyHint(t *testing.T) {
	t.Parallel()

	require.Equal(t, "no key", Preset{ID: "x"}.KeyHint())
	require.Equal(t, "$X_API_KEY", Preset{ID: "x", APIKeyEnv: "X_API_KEY"}.KeyHint())

	for _, p := range presets {
		if p.APIKeyEnv != "" {
			require.Equal(t, "$"+p.APIKeyEnv, p.KeyHint())
		}
	}
}

// TestDescribeListsEveryPreset guards the help output: a preset missing from
// it is undiscoverable, which is the whole point of shipping a catalog.
func TestDescribeListsEveryPreset(t *testing.T) {
	t.Parallel()

	got := Describe()
	for _, p := range presets {
		require.Contains(t, got, p.ID)
		require.Contains(t, got, p.Note)
		require.Contains(t, got, p.KeyHint())
	}
}

func upper(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'a' && r <= 'z' {
			out[i] = r - 32
		}
	}
	return string(out)
}

// Lookup2 is a test helper that fails instead of returning a zero Preset.
func Lookup2(t *testing.T, name string) map[string]any {
	t.Helper()

	p, ok := Lookup(name)
	require.True(t, ok, "unknown preset %q", name)
	return p.Config()
}
