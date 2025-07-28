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
)

func checkVaultStatus() error {
    if config.Cfg.ActiveVault == "" {
        return fmt.Errorf("no active vault is set. Use 'vault.module vaults use <name>' to set one")
    }
    
    activeVault, exists := config.Cfg.Vaults[config.Cfg.ActiveVault]
    if !exists {
        return fmt.Errorf("active vault '%s' not found in configuration", config.Cfg.ActiveVault)
    }
    
    // Проверяем существование файлов
    if _, err := os.Stat(activeVault.KeyFile); os.IsNotExist(err) {
        return fmt.Errorf("vault key file not found: %s", activeVault.KeyFile)
    }
    
    if activeVault.Encryption == "yubikey" && activeVault.RecipientsFile != "" {
        if _, err := os.Stat(activeVault.RecipientsFile); os.IsNotExist(err) {
            return fmt.Errorf("recipients file not found: %s", activeVault.RecipientsFile)
        }
    }
    
    return nil
}

func askForInput(prompt string) (string, error) {
    fmt.Print(colors.SafeColor(prompt+": ", colors.Info))
    reader := bufio.NewReader(os.Stdin)
    input, err := reader.ReadString('\n')
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(input), nil
}

func askForSecretInput(prompt string) (string, error) {
    fmt.Print(colors.SafeColor(prompt+": ", colors.Info))
    
    bytePassword, err := term.ReadPassword(int(syscall.Stdin))
    if err != nil {
        return "", err
    }
    fmt.Println() // Новая строка после ввода пароля
    
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
