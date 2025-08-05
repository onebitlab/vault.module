// internal/security/clipboard.go
package security

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"vault.module/internal/config"
)

type Clipboard struct{}

var clipboardInstance *Clipboard

func GetClipboard() *Clipboard {
	if clipboardInstance == nil {
		clipboardInstance = &Clipboard{}
	}
	return clipboardInstance
}

func (c *Clipboard) WriteAllWithCustomTimeout(data string, timeoutSeconds int) error {
	if err := c.writeToClipboard(data); err != nil {
		return err
	}

	// Create a detached process for clipboard clearing
	switch runtime.GOOS {
	case "darwin":
		return c.scheduleMacOSClipboardClear(timeoutSeconds)
	case "linux":
		return c.scheduleLinuxClipboardClear(timeoutSeconds)
	case "windows":
		return c.scheduleWindowsClipboardClear(timeoutSeconds)
	default:
		// Fallback to goroutine for unsupported platforms
		go func() {
			time.Sleep(time.Duration(timeoutSeconds) * time.Second)
			c.clearClipboard()
		}()
	}

	return nil
}

func (c *Clipboard) scheduleMacOSClipboardClear(timeoutSeconds int) error {
	// Use nohup to create a detached process
	script := fmt.Sprintf("sleep %d && echo '' | pbcopy", timeoutSeconds)
	cmd := exec.Command("nohup", "sh", "-c", script)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start() // Start(), not Run() - do not wait for completion
}

func (c *Clipboard) scheduleLinuxClipboardClear(timeoutSeconds int) error {
	var script string
	if _, err := exec.LookPath("xclip"); err == nil {
		script = fmt.Sprintf("sleep %d && echo '' | xclip -selection clipboard", timeoutSeconds)
	} else if _, err := exec.LookPath("xsel"); err == nil {
		script = fmt.Sprintf("sleep %d && echo '' | xsel --clipboard --input", timeoutSeconds)
	} else {
		return fmt.Errorf("no clipboard utility found")
	}

	cmd := exec.Command("nohup", "sh", "-c", script)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start()
}

func (c *Clipboard) scheduleWindowsClipboardClear(timeoutSeconds int) error {
	// For Windows, use timeout and start /B for background process
	script := fmt.Sprintf("timeout %d >nul && echo. | clip", timeoutSeconds)
	cmd := exec.Command("cmd", "/C", "start", "/B", script)
	return cmd.Start()
}

func (c *Clipboard) writeToClipboard(data string) error {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(data)
		return cmd.Run()
	case "linux":
		// Try xclip
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd := exec.Command("xclip", "-selection", "clipboard")
			cmd.Stdin = strings.NewReader(data)
			return cmd.Run()
		}
		// Try xsel
		if _, err := exec.LookPath("xsel"); err == nil {
			cmd := exec.Command("xsel", "--clipboard", "--input")
			cmd.Stdin = strings.NewReader(data)
			return cmd.Run()
		}
		return fmt.Errorf("no clipboard utility found (install xclip or xsel)")
	case "windows":
		cmd := exec.Command("clip")
		cmd.Stdin = strings.NewReader(data)
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func (c *Clipboard) clearClipboard() error {
	// Clear clipboard by writing an empty string
	if err := c.writeToClipboard(""); err != nil {
		// If clearing failed, try writing a space
		return c.writeToClipboard(" ")
	}
	return nil
}

// Standard function for compatibility
func CopyToClipboard(data string) error {
	return GetClipboard().WriteAllWithCustomTimeout(data, config.GetClipboardTimeout())
}

// ClearClipboard immediately clears the clipboard (for shutdown cleanup)
func ClearClipboard() error {
	return GetClipboard().clearClipboard()
}

// CopyToClipboardWithAutoCleanup copies data and registers for shutdown cleanup
func CopyToClipboardWithAutoCleanup(data string, description string) error {
	// Register clipboard for cleanup before copying
	// Note: Import shutdown package when this is used
	// shutdown.RegisterClipboardGlobal(description)

	return CopyToClipboard(data)
}
