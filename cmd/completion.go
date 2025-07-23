// File: cmd/completion.go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate autocompletion script for the specified shell",
	Long: `To load completions:

Bash:
  $ source <(vault.module completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ vault.module completion bash > /etc/bash_completion.d/vault.module
  # macOS:
  $ vault.module completion bash > /usr/local/etc/bash_completion.d/vault.module

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ vault.module completion zsh > "${fpath[1]}/_vault.module"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ vault.module completion fish | source

  # To load completions for each session, execute once:
  $ vault.module completion fish > ~/.config/fish/completions/vault.module.fish
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	// FIX: Use the new, composable argument validator.
	Args: cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		if programmaticMode {
			return fmt.Errorf("this command is not available in programmatic mode")
		}
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletion(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
