package colors

import (
	"os"
)

// ANSI color codes
const (
	ResetCode  = "\033[0m"
	RedCode    = "\033[31m"
	GreenCode  = "\033[32m"
	YellowCode = "\033[33m"
	BlueCode   = "\033[34m"
	CyanCode   = "\033[36m"
	WhiteCode  = "\033[37m"
	BoldCode   = "\033[1m"
	DimCode    = "\033[2m"
)

// Main colors for messages
func Error(text string) string {
	return RedCode + text + ResetCode
}

func Success(text string) string {
	return GreenCode + text + ResetCode
}

func Warning(text string) string {
	return YellowCode + text + ResetCode
}

func Info(text string) string {
	return BlueCode + text + ResetCode
}

// Additional colors for elements
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

func White(text string) string {
	return WhiteCode + text + ResetCode
}

// Check if terminal supports colors
func SupportsColors() bool {
	// Check NO_COLOR environment variable
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check if stdout is connected to terminal
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// Safe color output (disables colors if not supported)
func SafeColor(text string, colorFunc func(string) string) string {
	if SupportsColors() {
		return colorFunc(text)
	}
	return text
}
