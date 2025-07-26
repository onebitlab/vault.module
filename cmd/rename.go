// File: cmd/rename.go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"vault.module/internal/config"
	"vault.module/internal/vault"

	"github.com/spf13/cobra"
)

var renameYesFlag bool

var renameCmd = &cobra.Command{
	Use:   "rename <OLD_PREFIX> <NEW_PREFIX>",
	Short: "Safely renames a wallet in the active vault.",
	Long: `Safely renames a wallet in the active vault.

This command will rename the wallet while preserving all its data.
You will be prompted for confirmation unless --yes flag is used.

Examples:
  vault.module rename A1 A2
  vault.module rename oldwallet newwallet --yes
`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Проверяем состояние vault перед выполнением команды
		if err := checkVaultStatus(); err != nil {
			return err
		}

		oldPrefix := args[0]
		newPrefix := args[1]
		activeVault, err := config.GetActiveVault()
		if err != nil {
			return err
		}
		v, err := vault.LoadVault(activeVault)
		if err != nil {
			return err
		}
		if _, exists := v[oldPrefix]; !exists {
			return fmt.Errorf("wallet with prefix '%s' not found", oldPrefix)
		}
		if _, exists := v[newPrefix]; exists {
			return fmt.Errorf("wallet with prefix '%s' already exists", newPrefix)
		}
		if !renameYesFlag {
			fmt.Printf("Are you sure you want to rename wallet '%s' to '%s'? [y/N]: ", oldPrefix, newPrefix)
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))
			if answer != "y" && answer != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}
		v[newPrefix] = v[oldPrefix]
		delete(v, oldPrefix)
		if err := vault.SaveVault(activeVault, v); err != nil {
			return err
		}
		fmt.Printf("Wallet '%s' renamed to '%s'.\n", oldPrefix, newPrefix)
		return nil
	},
}

func init() {
	// Регистрация перенесена в root.go

	// Настройка флагов
	renameCmd.Flags().BoolVar(&renameYesFlag, "yes", false, "Rename without confirmation prompt")
}
