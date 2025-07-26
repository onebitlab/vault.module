package security

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// SecureClipboard предоставляет безопасную работу с clipboard
type SecureClipboard struct {
	lastData   string
	clearTimer *time.Timer
}

// NewSecureClipboard создает новый экземпляр SecureClipboard
func NewSecureClipboard() *SecureClipboard {
	return &SecureClipboard{}
}

// WriteAll безопасно копирует данные в clipboard с автоматической очисткой
func (sc *SecureClipboard) WriteAll(data string) error {
	// Очищаем предыдущий таймер если он есть
	if sc.clearTimer != nil {
		sc.clearTimer.Stop()
	}

	// Копируем данные в clipboard
	if err := sc.copyToClipboard(data); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %s", err.Error())
	}

	sc.lastData = data

	// Устанавливаем таймер на очистку через 30 секунд
	sc.clearTimer = time.AfterFunc(30*time.Second, func() {
		sc.Clear()
	})

	return nil
}

// Clear очищает clipboard
func (sc *SecureClipboard) Clear() error {
	if sc.clearTimer != nil {
		sc.clearTimer.Stop()
		sc.clearTimer = nil
	}

	// Очищаем clipboard
	if err := sc.copyToClipboard(""); err != nil {
		return fmt.Errorf("failed to clear clipboard: %s", err.Error())
	}

	sc.lastData = ""
	return nil
}

// ReadAll читает данные из clipboard
func (sc *SecureClipboard) ReadAll() (string, error) {
	return sc.readFromClipboard()
}

// copyToClipboard копирует данные в clipboard используя системные команды
func (sc *SecureClipboard) copyToClipboard(data string) error {
	switch runtime.GOOS {
	case "darwin":
		// macOS
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(data)
		return cmd.Run()
	case "linux":
		// Linux (требует xclip или xsel)
		if sc.hasCommand("xclip") {
			cmd := exec.Command("xclip", "-selection", "clipboard")
			cmd.Stdin = strings.NewReader(data)
			return cmd.Run()
		} else if sc.hasCommand("xsel") {
			cmd := exec.Command("xsel", "--clipboard", "--input")
			cmd.Stdin = strings.NewReader(data)
			return cmd.Run()
		}
		return fmt.Errorf("no clipboard tool available (install xclip or xsel)")
	case "windows":
		// Windows
		cmd := exec.Command("powershell", "-command", fmt.Sprintf("Set-Clipboard -Value '%s'", data))
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// readFromClipboard читает данные из clipboard
func (sc *SecureClipboard) readFromClipboard() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		// macOS
		cmd := exec.Command("pbpaste")
		output, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return string(output), nil
	case "linux":
		// Linux
		if sc.hasCommand("xclip") {
			cmd := exec.Command("xclip", "-selection", "clipboard", "-o")
			output, err := cmd.Output()
			if err != nil {
				return "", err
			}
			return string(output), nil
		} else if sc.hasCommand("xsel") {
			cmd := exec.Command("xsel", "--clipboard", "--output")
			output, err := cmd.Output()
			if err != nil {
				return "", err
			}
			return string(output), nil
		}
		return "", fmt.Errorf("no clipboard tool available (install xclip or xsel)")
	case "windows":
		// Windows
		cmd := exec.Command("powershell", "-command", "Get-Clipboard")
		output, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return string(output), nil
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// hasCommand проверяет наличие команды в системе
func (sc *SecureClipboard) hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// GetClipboard возвращает глобальный экземпляр SecureClipboard
func GetClipboard() *SecureClipboard {
	return &SecureClipboard{}
}

// CopyToClipboard копирует данные в clipboard с автоматической очисткой
func CopyToClipboard(data string) error {
	clipboard := GetClipboard()
	return clipboard.WriteAll(data)
}

// ReadClipboard читает данные из clipboard
func ReadClipboard() (string, error) {
	clipboard := GetClipboard()
	return clipboard.ReadAll()
}

// ClearClipboard очищает clipboard
func ClearClipboard() error {
	clipboard := GetClipboard()
	return clipboard.Clear()
}
