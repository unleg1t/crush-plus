// Package providerpresets ships a small catalog of ready-made provider
// definitions that `provider preset` applies in a single command.
//
// Crush's built-in provider list comes from charm.land/catwalk, which covers
// the commercial providers people pay for. This package covers the other
// end of the spectrum: keyless endpoints, hobby proxies, and free tiers that
// are not worth a catwalk entry of their own.
//
// A preset is a starting point, not a special case. Applying one writes an
// ordinary provider entry into the config, so every preset field stays
// overridable with `provider add` flags and everything downstream — the
// models picker, `crush models`, discovery — treats the result exactly like a
// hand-written provider.
package providerpresets

import (
	"fmt"
	"slices"
	"strings"
)

// Model is a model seeded by a preset. Seeds exist so a provider is usable
// before (or without) a successful `/models` probe.
//
// Every field is optional. Leaving one unset keeps it out of the generated
// config rather than writing a zero, so a later discovery pass or a
// `model add` override can still fill it in.
type Model struct {
	ID             string
	Name           string
	ContextWindow  int
	MaxTokens      int
	CanReason      bool
	SupportsImages bool
}

// config renders the model as the JSON shape ProviderConfig.Models expects.
func (m Model) config() map[string]any {
	c := map[string]any{"id": m.ID}
	if m.Name != "" {
		c["name"] = m.Name
	}
	if m.ContextWindow > 0 {
		c["context_window"] = m.ContextWindow
	}
	if m.MaxTokens > 0 {
		c["default_max_tokens"] = m.MaxTokens
	}
	if m.CanReason {
		c["can_reason"] = true
	}
	if m.SupportsImages {
		c["supports_attachments"] = true
	}
	return c
}

// Preset is a ready-made provider definition.
type Preset struct {
	// ID is the provider ID the preset registers under. It is also the
	// name `provider preset` accepts.
	ID string
	// Name is the display name shown in the models picker.
	Name string
	// Aliases are extra names `provider preset` resolves to this preset.
	Aliases []string
	// Type is the wire protocol, e.g. "openai-compat".
	Type string
	// BaseURL is the API root, including any /v1 suffix.
	BaseURL string
	// APIKeyEnv names the environment variable the API key is read from.
	// An empty value means the endpoint needs no credentials; a key can
	// still be attached with `provider preset <id> --api-key`.
	APIKeyEnv string
	// DiscoverModels forces model discovery on or off. Nil leaves the
	// default, which probes /models and merges the result.
	DiscoverModels *bool
	// FlatRate zeroes cost accounting, which is what a free tier wants.
	FlatRate bool
	// HomeURL is where a user goes to get a key or read the terms.
	HomeURL string
	// Note is the one-line summary shown by `provider preset list`, and
	// the place to record anything a user must know before sending code
	// to this endpoint.
	Note string
	// Models are seeded onto the provider.
	Models []Model
}

// Config renders the preset as provider config keys — the same shape
// `provider add` writes. Keys are omitted when the preset has nothing to
// say about them so the generated config stays readable.
func (p Preset) Config() map[string]any {
	c := map[string]any{
		"name":     p.Name,
		"type":     p.Type,
		"base_url": p.BaseURL,
	}
	if p.APIKeyEnv != "" {
		c["api_key"] = "$" + p.APIKeyEnv
	}
	if p.DiscoverModels != nil {
		c["discover_models"] = *p.DiscoverModels
	}
	if p.FlatRate {
		c["flat_rate"] = true
	}
	if len(p.Models) > 0 {
		models := make([]any, 0, len(p.Models))
		for _, m := range p.Models {
			models = append(models, m.config())
		}
		c["models"] = models
	}
	return c
}

// KeyHint describes where the preset's credential comes from, for display.
func (p Preset) KeyHint() string {
	if p.APIKeyEnv == "" {
		return "no key"
	}
	return "$" + p.APIKeyEnv
}

// List returns every preset, ordered by ID.
func List() []Preset {
	return slices.Clone(presets)
}

// Lookup resolves a preset by ID or alias. Matching is case-insensitive so
// `provider preset Crax` behaves like `provider preset crax`.
func Lookup(name string) (Preset, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, p := range presets {
		if strings.ToLower(p.ID) == want {
			return p, true
		}
	}
	for _, p := range presets {
		if slices.ContainsFunc(p.Aliases, func(a string) bool {
			return strings.ToLower(a) == want
		}) {
			return p, true
		}
	}
	return Preset{}, false
}

// Names returns every preset ID, for error messages and completions.
func Names() []string {
	ids := make([]string, 0, len(presets))
	for _, p := range presets {
		ids = append(ids, p.ID)
	}
	return ids
}

// Bool returns a *bool for the given value, for the DiscoverModels field.
func Bool(v bool) *bool { return &v }

// craxModels is crax-gpt's published catalog, mirrored from the static
// fallback list in its web client. Discovery replaces these entries with
// live data whenever the service is up, but its API goes into a "locked"
// state often enough that shipping the list is worth it.
var craxModels = []Model{
	{ID: "gpt-6-luna", Name: "GPT 6 Luna", ContextWindow: 1_050_000, SupportsImages: true},
	{ID: "gpt-5-6-luna", Name: "GPT 5.6 Luna", ContextWindow: 1_050_000, SupportsImages: true},
	{ID: "grok-4-6", Name: "Grok 4.6", ContextWindow: 256_000, SupportsImages: true},
	{ID: "grok-4-3", Name: "Grok 4.3", ContextWindow: 1_000_000, SupportsImages: true},
	{ID: "grok-code-fast-1", Name: "Grok Code Fast 1", ContextWindow: 256_000, SupportsImages: true},
	{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash", ContextWindow: 1_000_000, SupportsImages: true},
	{ID: "qwen3-coder-480b", Name: "Qwen3 Coder 480B", ContextWindow: 262_144, SupportsImages: true},
	{ID: "kimi-k2-7-code", Name: "Kimi K2.7 Code", ContextWindow: 262_144, SupportsImages: true},
	{ID: "kimi-k2-6", Name: "Kimi K2.6", ContextWindow: 262_144, SupportsImages: true},
	{ID: "glm-5.3", Name: "GLM 5.3", ContextWindow: 1_000_000},
	{ID: "glm-5.3-flash", Name: "GLM 5.3 Flash", ContextWindow: 1_000_000},
	{ID: "glm-5.2", Name: "GLM 5.2", ContextWindow: 1_000_000},
	{ID: "llama-4-maverick", Name: "Llama 4 Maverick", ContextWindow: 1_000_000, SupportsImages: true},
	{ID: "gemma-3-12b", Name: "Gemma 3 12B", ContextWindow: 131_072, SupportsImages: true},
}

// presets is the catalog. It holds endpoints Crush does not ship in the
// catwalk catalog: keyless services, community proxies, and free tiers.
//
// The free providers here are not all first-party. Some are run by one
// person on a home server, and several log or train on requests. Each
// preset's Note says so, because a coding agent sends whatever the user
// reads into these endpoints. Anything a user would not paste into a public
// chat box should not go here without thinking about it first.
var presets = []Preset{
	{
		ID:        "crax",
		Name:      "crax-gpt",
		Aliases:   []string{"craxgpt", "gpt.crax.lol"},
		Type:      "openai-compat",
		BaseURL:   "https://gpt.crax.lol/v1",
		APIKeyEnv: "CRAX_API_KEY",
		FlatRate:  true,
		HomeURL:   "https://gpt.crax.lol",
		Note:      "Free key, one account per person, 28 models. Single-operator hobby service that locks its API at times; seeded models keep it usable meanwhile.",
		Models:    craxModels,
	},
	{
		ID:             "github-models",
		Name:           "GitHub Models",
		Aliases:        []string{"github"},
		Type:           "openai-compat",
		BaseURL:        "https://models.github.ai/inference",
		APIKeyEnv:      "GITHUB_TOKEN",
		DiscoverModels: Bool(false),
		FlatRate:       true,
		HomeURL:        "https://github.com/marketplace/models",
		Note:           "Free with a GitHub PAT that has models:read. GitHub serves no OpenAI-shaped model list, so register what you want with `model add`.",
	},
	{
		ID:        "logfare",
		Name:      "Logfare",
		Aliases:   []string{"logfare.ai"},
		Type:      "openai-compat",
		BaseURL:   "https://logfare.ai/v1",
		APIKeyEnv: "LOGFARE_API_KEY",
		FlatRate:  true,
		HomeURL:   "https://logfare.ai/register",
		Note:      "Free key, no rate limit, frontier models. Most of its models are flagged to train on your requests — do not send code you would not share.",
	},
	{
		ID:        "nvidia",
		Name:      "NVIDIA NIM",
		Aliases:   []string{"nim", "nvidia-nim"},
		Type:      "openai-compat",
		BaseURL:   "https://integrate.api.nvidia.com/v1",
		APIKeyEnv: "NVIDIA_API_KEY",
		FlatRate:  true,
		HomeURL:   "https://build.nvidia.com",
		Note:      "Free rate-limited tier with NVIDIA's hosted open models. The catalog rotates and retires models, so discovery is the source of truth.",
	},
	{
		ID:        "pollinations",
		Name:      "Pollinations",
		Aliases:   []string{"gen.pollinations.ai"},
		Type:      "openai-compat",
		BaseURL:   "https://gen.pollinations.ai/v1",
		APIKeyEnv: "POLLINATIONS_API_KEY",
		FlatRate:  true,
		HomeURL:   "https://enter.pollinations.ai/keys",
		Note:      "Free tier of 300+ models, including frontier ones, on Pollen credits. Requests are routed through community infrastructure, and so are logged.",
	},
}

// Describe renders the catalog for `provider preset list`. Each preset gets
// two lines: the identifiers, then the note wrapped underneath them.
func Describe() string {
	var b strings.Builder
	for _, p := range presets {
		fmt.Fprintf(&b, "  %-15s %-22s %s\n", p.ID, p.Name, p.KeyHint())
		fmt.Fprintf(&b, "  %-15s %s\n", "", p.Note)
	}
	return strings.TrimRight(b.String(), "\n")
}
