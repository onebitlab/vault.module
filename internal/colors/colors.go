package colors

import (
	"os"
)

// Цветовые коды ANSI
const (
	ResetCode  = "\033[0m"
	RedCode    = "\033[31m"
	GreenCode  = "\033[32m"
	YellowCode = "\033[33m"
	BlueCode   = "\033[34m"
	CyanCode   = "\033[36m"
	BoldCode   = "\033[1m"
	DimCode    = "\033[2m"
)

// Основные цвета для сообщений
func Error(text string) string {
	return RedCode + "❌ " + text + ResetCode
}

func Success(text string) string {
	return GreenCode + "✅ " + text + ResetCode
}

func Warning(text string) string {
	return YellowCode + "⚠️ " + text + ResetCode
}

func Info(text string) string {
	return BlueCode + "ℹ️ " + text + ResetCode
}

// Дополнительные цвета для элементов
func Cyan(text string) string {
	return CyanCode + text + ResetCode
}

func Yellow(text string) string {
	return YellowCode + text + ResetCode
}

func Dim(text string) string {
	return DimCode + text + ResetCode
}

func Bold(text string) string {
	return BoldCode + text + ResetCode
}

// Проверка, поддерживает ли терминал цвета
func SupportsColors() bool {
	// Проверяем переменную окружения NO_COLOR
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Проверяем, что stdout подключен к терминалу
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// Безопасный вывод с цветами (отключает цвета если не поддерживаются)
func SafeColor(text string, colorFunc func(string) string) string {
	if SupportsColors() {
		return colorFunc(text)
	}
	return text
}
