// internal/security/resource_manager.go
package security

// ResourceManager defines the interface for managing cleanup of sensitive resources
type ResourceManager interface {
	// RegisterSecureString registers a SecureString for cleanup during shutdown
	RegisterSecureString(secureStr *SecureString, description string)
	
	// UnregisterSecureString removes a SecureString from cleanup registry
	UnregisterSecureString(secureStr *SecureString)
	
	// RegisterTempFile registers a temporary file for secure cleanup
	RegisterTempFile(filePath string, description string)
	
	// RegisterClipboard registers clipboard for cleanup
	RegisterClipboard(description string)
	
	// IsShutdown returns true if shutdown has been initiated
	IsShutdown() bool
	
	// GetResourceCount returns the number of registered resources
	GetResourceCount() int
}

// Global resource manager instance
var globalResourceManager ResourceManager

// SetResourceManager sets the global resource manager instance
func SetResourceManager(manager ResourceManager) {
	globalResourceManager = manager
}

// GetResourceManager returns the global resource manager instance
func GetResourceManager() ResourceManager {
	return globalResourceManager
}
