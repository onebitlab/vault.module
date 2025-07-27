package security

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// SecureClipboard provides secure clipboard operations
type SecureClipboard struct {
	lastData   string
	clearTimer *time.Timer
}

// NewSecureClipboard creates a new SecureClipboard instance
func NewSecureClipboard() *SecureClipboard {
	return &SecureClipboard{}
}

// WriteAll safely copies data to clipboard with automatic cleanup
func (sc *SecureClipboard) WriteAll(data string) error {
	// Clear previous timer if exists
	if sc.clearTimer != nil {
		sc.clearTimer.Stop()
	}

	// Copy data to clipboard
	if err := sc.copyToClipboard(data); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %s", err.Error())
	}

	sc.lastData = data

	// Set timer to clear after 30 seconds
	sc.clearTimer = time.AfterFunc(30*time.Second, func() {
		sc.Clear()
	})

	return nil
}

// Clear clears clipboard
func (sc *SecureClipboard) Clear() error {
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
		// Windows
		cmd := exec.Command("powershell", "-command", fmt.Sprintf("Set-Clipboard -Value '%s'", data))
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
		return string(output), nil
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

// GetClipboard returns global SecureClipboard instance
func GetClipboard() *SecureClipboard {
	if globalClipboard == nil {
		globalClipboard = &SecureClipboard{}
	}
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
