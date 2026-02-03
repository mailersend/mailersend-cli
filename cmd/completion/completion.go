package completion

import (
	"os"

	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for mailersend.

To load completions:

Bash:
  $ source <(mailersend completion bash)
  # Or for permanent:
  $ mailersend completion bash > /etc/bash_completion.d/mailersend

Zsh:
  $ mailersend completion zsh > "${fpath[1]}/_mailersend"

Fish:
  $ mailersend completion fish | source
  # Or for permanent:
  $ mailersend completion fish > ~/.config/fish/completions/mailersend.fish

PowerShell:
  PS> mailersend completion powershell | Out-String | Invoke-Expression
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}
