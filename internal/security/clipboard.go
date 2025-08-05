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

	// Создаём независимый процесс для очистки clipboard
	switch runtime.GOOS {
	case "darwin":
		return c.scheduleMacOSClipboardClear(timeoutSeconds)
	case "linux":
		return c.scheduleLinuxClipboardClear(timeoutSeconds)
	case "windows":
		return c.scheduleWindowsClipboardClear(timeoutSeconds)
	default:
		// Fallback к горутине для неподдерживаемых платформ
		go func() {
			time.Sleep(time.Duration(timeoutSeconds) * time.Second)
			c.clearClipboard()
		}()
	}

	return nil
}

func (c *Clipboard) scheduleMacOSClipboardClear(timeoutSeconds int) error {
	// Используем nohup для создания независимого процесса
	script := fmt.Sprintf("sleep %d && echo '' | pbcopy", timeoutSeconds)
	cmd := exec.Command("nohup", "sh", "-c", script)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Start() // Start(), не Run() - чтобы не ждать завершения
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
	// Для Windows используем timeout и start /B для фонового процесса
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
		// Пробуем xclip
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd := exec.Command("xclip", "-selection", "clipboard")
			cmd.Stdin = strings.NewReader(data)
			return cmd.Run()
		}
		// Пробуем xsel
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
	// Очищаем clipboard, записывая пустую строку
	if err := c.writeToClipboard(""); err != nil {
		// Если очистка не удалась, попробуем записать пробел
		return c.writeToClipboard(" ")
	}
	return nil
}

// Стандартная функция для совместимости
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

