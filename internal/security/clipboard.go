// internal/security/clipboard.go
package security

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
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

	// Запуск горутины для очистки через указанное время
	go func() {
		time.Sleep(time.Duration(timeoutSeconds) * time.Second)
		c.clearClipboard()
	}()

	return nil
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
	return c.writeToClipboard("")
}

// Стандартная функция для совместимости
func CopyToClipboard(data string) error {
	return GetClipboard().WriteAllWithCustomTimeout(data, 30)
}
