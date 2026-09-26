package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/charmbracelet/crush/internal/providerpresets"
)

// handleProvider implements the `provider` builtin.
//
// Usage:
//
//	provider add <id> [--name NAME] [--type TYPE] [--api-key KEY]
//	    [--base-url URL] [--disable true|false] [--flat-rate true|false]
//	    [--discover-models true|false] [--system-prompt-prefix TEXT]
//	    [--extra-header KEY VALUE] [--extra-body JSON]
//	    [--provider-options JSON]
//	provider preset [<name>] [--no-seed-models] [flags]
//	provider remove <id>   (alias: rm)
//
// "add" defines or updates a provider; repeated calls with the same <id>
// update the same entry. "preset" fills in a provider from the built-in
// catalog, or lists that catalog when given no name. "remove" removes a
// provider and all its children.
func handleProvider(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: provider add <id> [flags] | provider preset [<name>] | provider remove <id>")
	}

	switch args[1] {
	case "add":
		return providerAdd(b, args, stderr)
	case "preset":
		return providerPreset(b, args, stdout, stderr)
	case "remove", "rm":
		return providerRemove(b, args, stderr)
	default:
		return usage(stderr, fmt.Sprintf("provider: unknown subcommand %q (expected add, preset, or remove)", args[1]))
	}
}

// providerAddFlags is the declarative flag surface for `provider add`.
var providerAddFlags = []flagSpec{
	{name: "--name", jsonKey: "name", kind: flagString, op: opSet},
	{name: "--type", jsonKey: "type", kind: flagString, op: opSet},
	{name: "--api-key", jsonKey: "api_key", kind: flagString, op: opSet},
	{name: "--base-url", jsonKey: "base_url", kind: flagString, op: opSet},
	{name: "--disable", jsonKey: "disable", kind: flagBool, op: opSet},
	{name: "--flat-rate", jsonKey: "flat_rate", kind: flagBool, op: opSet},
	{name: "--discover-models", jsonKey: "discover_models", kind: flagBool, op: opSet},
	{name: "--system-prompt-prefix", jsonKey: "system_prompt_prefix", kind: flagString, op: opSet},
	{name: "--extra-header", child: "extra_headers", kind: flagKeyValue, op: opSetChild},
	{name: "--extra-body", child: "extra_body", kind: flagJSONObject, op: opMergeChild},
	{name: "--provider-options", child: "provider_options", kind: flagJSONObject, op: opMergeChild},
}

func providerAdd(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: provider add <id> [--name NAME] [--type TYPE] [--api-key KEY] [--base-url URL] [--disable true|false] [--flat-rate true|false] [--discover-models true|false] [--system-prompt-prefix TEXT] [--extra-header KEY VALUE] [--extra-body JSON] [--provider-options JSON]")
	}
	id := args[2]
	slog.Info("Provider defined in shell config", "provider", id)
	p := childMap(b.section("providers"), id)

	if err := applyFlags(providerAddFlags, args, 3, p, "provider add", stderr); err != nil {
		return err
	}

	slog.Debug("Provider recorded", "provider", id)
	return nil
}

func providerRemove(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: provider remove <id>")
	}
	id := args[2]
	delete(b.section("providers"), id)
	slog.Info("Provider removed in shell config", "provider", id)
	return nil
}

// providerPresetFlags overrides fields of the applied preset. It covers the
// subset of `provider add` that makes sense while scaffolding a preset; the
// preset itself already supplies a base URL, type, and key variable.
//
// --no-seed-models is a directive rather than a config key: it drops the
// preset's seeded models so discovery or `model add` is the only source. It
// is read back out and deleted before the entry is stored.
var providerPresetFlags = []flagSpec{
	{name: "--name", jsonKey: "name", kind: flagString, op: opSet},
	{name: "--type", jsonKey: "type", kind: flagString, op: opSet},
	{name: "--api-key", jsonKey: "api_key", kind: flagString, op: opSet},
	{name: "--base-url", jsonKey: "base_url", kind: flagString, op: opSet},
	{name: "--discover-models", jsonKey: "discover_models", kind: flagBool, op: opSet},
	{name: "--flat-rate", jsonKey: "flat_rate", kind: flagBool, op: opSet},
	{name: "--no-seed-models", jsonKey: noSeedModelsKey, kind: flagBoolTrue, op: opSet},
	{name: "--extra-header", child: "extra_headers", kind: flagKeyValue, op: opSetChild},
	{name: "--provider-options", child: "provider_options", kind: flagJSONObject, op: opMergeChild},
}

// noSeedModelsKey is the scratch key --no-seed-models parses into. It never
// reaches the config.
const noSeedModelsKey = "no_seed_models"

// providerPreset implements `provider preset`, which turns a catalog entry
// into a real provider entry. With no name it prints the catalog, notes
// included, so a user can see what a provider does with their code before
// wiring it up.
func providerPreset(b *ConfigBuilder, args []string, stdout, stderr io.Writer) error {
	// `provider preset` and `provider preset list` both mean "show me".
	if len(args) < 3 || args[2] == "list" {
		fmt.Fprintln(stdout, providerpresets.Describe())
		return nil
	}

	name := args[2]
	preset, ok := providerpresets.Lookup(name)
	if !ok {
		return usage(stderr, fmt.Sprintf(
			"provider preset: unknown preset %q (available: %s)",
			name, strings.Join(providerpresets.Names(), ", "),
		))
	}

	// The preset goes down first so any flag overrides it.
	p := childMap(b.section("providers"), preset.ID)
	applyPresetDefaults(p, preset.Config())
	if err := applyFlags(providerPresetFlags, args, 3, p, "provider preset", stderr); err != nil {
		return err
	}

	if noSeed, _ := p[noSeedModelsKey].(bool); noSeed {
		delete(p, "models")
	}
	delete(p, noSeedModelsKey)

	slog.Info("Provider preset applied in shell config", "preset", preset.ID, "provider", preset.ID)
	return nil
}

// applyPresetDefaults fills in the keys target is missing from cfg and
// leaves the rest alone.
//
// Presets are defaults, not overrides. A script that runs
// `provider preset crax` and then registers a model on it — or sets its own
// api_key — must not lose that work to a later `provider preset` call, so a
// key the script has already set always wins over the catalog.
func applyPresetDefaults(target, cfg map[string]any) {
	for k, v := range cfg {
		if _, exists := target[k]; !exists {
			target[k] = v
		}
	}
}
