package security

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// SecureClipboard provides secure clipboard operations
type SecureClipboard struct {
	lastData   string
	clearTimer *time.Timer
	mutex      sync.Mutex // Добавляем мьютекс для безопасности
}

// NewSecureClipboard creates a new SecureClipboard instance
func NewSecureClipboard() *SecureClipboard {
	return &SecureClipboard{}
}

// WriteAll safely copies data to clipboard with automatic cleanup
func (sc *SecureClipboard) WriteAll(data string) error {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	// Clear previous timer if exists
	if sc.clearTimer != nil {
		sc.clearTimer.Stop()
		sc.clearTimer = nil
	}

	// Copy data to clipboard
	if err := sc.copyToClipboard(data); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %s", err.Error())
	}

	sc.lastData = data

	// Set timer to clear after 30 seconds
	sc.clearTimer = time.AfterFunc(30*time.Second, func() {
		sc.clearIfMatches(data) // Очищаем только если данные совпадают
	})

	return nil
}

// clearIfMatches clears clipboard only if it contains the expected data
func (sc *SecureClipboard) clearIfMatches(expectedData string) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	// Читаем текущее содержимое clipboard
	currentData, err := sc.readFromClipboard()
	if err != nil {
		// Если не можем прочитать, все равно попробуем очистить
		sc.forceClipboardClear()
		return
	}

	// Очищаем только если данные совпадают с ожидаемыми
	if currentData == expectedData {
		if err := sc.copyToClipboard(""); err == nil {
			sc.lastData = ""
		}
	}

	// Очищаем таймер
	if sc.clearTimer != nil {
		sc.clearTimer.Stop()
		sc.clearTimer = nil
	}
}

// Clear clears clipboard
func (sc *SecureClipboard) Clear() error {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	return sc.forceClipboardClear()
}

// forceClipboardClear принудительно очищает clipboard
func (sc *SecureClipboard) forceClipboardClear() error {
	if sc.clearTimer != nil {
		sc.clearTimer.Stop()
		sc.clearTimer = nil
	}

	// Clear clipboard
	if err := sc.copyToClipboard(""); err != nil {
		return fmt.Errorf("failed to clear clipboard: %s", err.Error())
	}

	sc.lastData = ""
	return nil
}

// ReadAll reads data from clipboard
func (sc *SecureClipboard) ReadAll() (string, error) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	return sc.readFromClipboard()
}

// copyToClipboard copies data to clipboard using system commands
func (sc *SecureClipboard) copyToClipboard(data string) error {
	switch runtime.GOOS {
	case "darwin":
		// macOS
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(data)
		return cmd.Run()
	case "linux":
		// Linux (requires xclip or xsel)
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
		// Windows - исправляем проблему с экранированием
		cmd := exec.Command("cmd", "/c", fmt.Sprintf(`echo|set /p="%s"|clip`, strings.ReplaceAll(data, `"`, `""`)))
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// readFromClipboard reads data from clipboard
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
		return strings.TrimSpace(string(output)), nil
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// hasCommand checks if command exists in system
func (sc *SecureClipboard) hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// Global clipboard instance
var globalClipboard *SecureClipboard
var once sync.Once

// GetClipboard returns global SecureClipboard instance
func GetClipboard() *SecureClipboard {
	once.Do(func() {
		globalClipboard = NewSecureClipboard() // Используем конструктор
	})
	return globalClipboard
}

// CopyToClipboard copies data to clipboard with automatic cleanup
func CopyToClipboard(data string) error {
	clipboard := GetClipboard()
	return clipboard.WriteAll(data)
}

// ReadClipboard reads data from clipboard
func ReadClipboard() (string, error) {
	clipboard := GetClipboard()
	return clipboard.ReadAll()
}

// ClearClipboard clears clipboard
func ClearClipboard() error {
	clipboard := GetClipboard()
	return clipboard.Clear()
}
