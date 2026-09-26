package cmd

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/providerpresets"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "List ready-made provider presets",
	Long: `List ready-made provider presets.

Crush ships a small catalog of endpoints its own provider list does not cover:
keyless services, hobby proxies, and free tiers. A preset fills in the base
URL, protocol, and API key variable for one of them.

Most of these are free, which means someone else pays for the inference — and
sees whatever you send. Each preset's note says what happens to your requests.
Read it before pointing a coding agent at one.

Apply a preset from your crushrc with 'provider preset <id>'.`,
	Example: `# List every preset
crush providers

# Search presets
crush providers crax

# Then, in ~/.config/crush/crushrc:
#   provider preset pollinations
#   model large pollinations/openai-fast`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		term := strings.ToLower(strings.Join(args, " "))

		matched := make([]providerpresets.Preset, 0, len(providerpresets.List()))
		for _, p := range providerpresets.List() {
			if term == "" || presetMatches(p, term) {
				matched = append(matched, p)
			}
		}

		if len(matched) == 0 {
			if term != "" {
				return fmt.Errorf("no provider presets matching %q", term)
			}
			return fmt.Errorf("no provider presets available")
		}

		out := cmd.OutOrStdout()
		tty := isatty.IsTerminal(os.Stdout.Fd())

		// The note is the payload here, so it is never styled: it has to
		// stay readable when the output is piped into grep. Only the
		// identifiers get color, and only on a terminal.
		idStyle := lipgloss.NewStyle().Foreground(charmtone.Sapphire).Bold(true)
		nameStyle := lipgloss.NewStyle().Foreground(charmtone.Paprika)
		dimStyle := lipgloss.NewStyle().Foreground(charmtone.Smoke)
		style := func(s lipgloss.Style, v string) string {
			if !tty {
				return v
			}
			return s.Render(v)
		}

		for _, p := range matched {
			fmt.Fprintf(out, "  %s — %s (%s)\n",
				style(idStyle, p.ID),
				style(nameStyle, p.Name),
				p.KeyHint())
			fmt.Fprintf(out, "    %s\n", p.Note)
			fmt.Fprintf(out, "    %s\n\n", style(dimStyle, p.BaseURL))
		}

		fmt.Fprintln(out, style(dimStyle,
			"Add one with 'provider preset <id>' in your crushrc."))
		return nil
	},
}

// presetMatches reports whether a preset matches a search term, checking the
// same identifiers a user would plausibly type.
func presetMatches(p providerpresets.Preset, term string) bool {
	for _, s := range append([]string{p.ID, p.Name, p.BaseURL, p.HomeURL}, p.Aliases...) {
		if strings.Contains(strings.ToLower(s), term) {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.AddCommand(providersCmd)
}
