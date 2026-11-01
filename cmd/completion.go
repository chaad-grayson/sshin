package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate shell completion script for sshgo.

To load completions:

Bash:
  $ source <(sshgo completion bash)
  
  # To load completions for each session, add to your ~/.bashrc:
  # echo 'source <(sshgo completion bash)' >> ~/.bashrc

Zsh:
  $ source <(sshgo completion zsh)
  
  # To load completions for each session, add to your ~/.zshrc:
  # echo 'source <(sshgo completion zsh)' >> ~/.zshrc

Fish:
  $ sshgo completion fish | source
  
  # To load completions for each session:
  $ sshgo completion fish > ~/.config/fish/completions/sshgo.fish

PowerShell:
  PS> sshgo completion powershell | Out-String | Invoke-Expression
  
  # To load completions for each session, add to your PowerShell profile:
  # Add-Content $PROFILE 'sshgo completion powershell | Out-String | Invoke-Expression'
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
