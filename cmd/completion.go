package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate shell completion script for sshin.

To load completions:

Bash:
  $ source <(sshin completion bash)
  
  # To load completions for each session, add to your ~/.bashrc:
  # echo 'source <(sshin completion bash)' >> ~/.bashrc

Zsh:
  $ source <(sshin completion zsh)
  
  # To load completions for each session, add to your ~/.zshrc:
  # echo 'source <(sshin completion zsh)' >> ~/.zshrc

Fish:
  $ sshin completion fish | source
  
  # To load completions for each session:
  $ sshin completion fish > ~/.config/fish/completions/sshin.fish

PowerShell:
  PS> sshin completion powershell | Out-String | Invoke-Expression
  
  # To load completions for each session, add to your PowerShell profile:
  # Add-Content $PROFILE 'sshin completion powershell | Out-String | Invoke-Expression'
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletionV2(os.Stdout, true)
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

func init() {
	RootCmd.AddCommand(completionCmd)
}
