// internal/shutdown/helpers.go
package shutdown

// RegisterSecureStringGlobal is a helper function to register SecureString globally
// Note: This function is now deprecated. Use ResourceManager interface through dependency injection instead.
// func RegisterSecureStringGlobal(secureStr *security.SecureString, description string) {
//	GetManager().RegisterSecureString(secureStr, description)
// }

// RegisterTempFileGlobal registers a temporary file for secure cleanup
func RegisterTempFileGlobal(filePath string, description string) {
	GetManager().RegisterTempFile(filePath, description)
}

// RegisterClipboardGlobal registers clipboard for cleanup
func RegisterClipboardGlobal(description string) {
	GetManager().RegisterClipboard(description)
}

// UnregisterSecureStringGlobal removes a SecureString from cleanup registry
// Note: This function is now deprecated. Use ResourceManager interface through dependency injection instead.
// func UnregisterSecureStringGlobal(secureStr *security.SecureString) {
//	GetManager().UnregisterSecureString(secureStr)
// }

// CreateSecureStringWithAutoCleanup creates and registers a SecureString
// Note: This function is now deprecated. Use security.NewSecureStringWithRegistration instead.
// func CreateSecureStringWithAutoCleanup(value string, description string) *security.SecureString {
//	secureStr := security.NewSecureString(value)
//	if secureStr != nil && !secureStr.IsEmpty() {
//		RegisterSecureStringGlobal(secureStr, description)
//	}
//	return secureStr
// }

// CreateTempFileWithAutoCleanup creates a temporary file and registers it for cleanup
// Note: This function needs to be updated to use dependency injection or moved to security package
// func CreateTempFileWithAutoCleanup(pattern string, content []byte, description string) (string, error) {
//	filePath, err := security.SecureCreateTempFile(pattern, content)
//	if err != nil {
//		return "", err
//	}
//	
//	RegisterTempFileGlobal(filePath, description)
//	return filePath, nil
// }

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
