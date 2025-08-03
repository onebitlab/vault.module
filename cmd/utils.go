// cmd/utils.go
package cmd

import (
    "bufio"
    "fmt"
    "os"
    "strings"
    "syscall"
    
    "golang.org/x/term"
    "vault.module/internal/colors"
    "vault.module/internal/config"
    "vault.module/internal/errors"
)

func checkVaultStatus() error {
    if config.Cfg.ActiveVault == "" {
        return errors.NewActiveVaultNotSetError()
    }
    
    activeVault, exists := config.Cfg.Vaults[config.Cfg.ActiveVault]
    if !exists {
        return errors.NewVaultNotFoundError(config.Cfg.ActiveVault)
    }
    
    // Check file existence
    if _, err := os.Stat(activeVault.KeyFile); os.IsNotExist(err) {
        return errors.NewFileSystemError("access", activeVault.KeyFile, err).
            WithDetails("vault key file not found")
    }
    
    if activeVault.Encryption == "yubikey" && activeVault.RecipientsFile != "" {
        if _, err := os.Stat(activeVault.RecipientsFile); os.IsNotExist(err) {
            return errors.NewFileSystemError("access", activeVault.RecipientsFile, err).
                WithDetails("recipients file not found")
        }
    }
    
    return nil
}

func askForInput(prompt string) (string, error) {
    fmt.Print(colors.SafeColor(prompt+": ", colors.Info))
    reader := bufio.NewReader(os.Stdin)
    input, err := reader.ReadString('\n')
    if err != nil {
        return "", errors.NewInvalidInputError("console input", "failed to read from stdin")
    }
    return strings.TrimSpace(input), nil
}

func askForSecretInput(prompt string) (string, error) {
    fmt.Print(colors.SafeColor(prompt+": ", colors.Info))
    
    bytePassword, err := term.ReadPassword(int(syscall.Stdin))
    if err != nil {
        return "", errors.NewInvalidInputError("secret input", "failed to read password from stdin")
    }
    fmt.Println() // New line after password input
    
    return string(bytePassword), nil
}

func askForConfirmation(prompt string) bool {
    fmt.Printf("%s [y/N]: ", prompt)
    reader := bufio.NewReader(os.Stdin)
    response, err := reader.ReadString('\n')
    if err != nil {
        return false
    }
    
    response = strings.TrimSpace(strings.ToLower(response))
    return response == "y" || response == "yes"
}
