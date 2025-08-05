// internal/shutdown/helpers.go
package shutdown

// RegisterTempFileGlobal registers a temporary file for secure cleanup
func RegisterTempFileGlobal(filePath string, description string) {
	GetManager().RegisterTempFile(filePath, description)
}

// RegisterClipboardGlobal registers clipboard for cleanup
func RegisterClipboardGlobal(description string) {
	GetManager().RegisterClipboard(description)
}

// CreateTempFileWithAutoCleanup creates a temporary file and registers it for cleanup
// This version uses dependency injection
func CreateTempFileWithAutoCleanup(pattern string, content []byte, description string, createTempFile func(string, []byte) (string, error)) (string, error) {
	filePath, err := createTempFile(pattern, content)
	if err != nil {
		return "", err
	}

	RegisterTempFileGlobal(filePath, description)
	return filePath, nil
}

// IsShuttingDown returns true if shutdown has been initiated
func IsShuttingDown() bool {
	return GetManager().IsShutdown()
}

// GetResourceCount returns the number of registered resources
func GetResourceCount() int {
	return GetManager().GetResourceCount()
}
