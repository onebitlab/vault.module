// internal/integration.go
package internal

import (
	"vault.module/internal/security"
	"vault.module/internal/shutdown"
)

// ShutdownManagerAdapter adapts the shutdown manager to the security ResourceManager interface
type ShutdownManagerAdapter struct {
	manager *shutdown.GracefulShutdownManager
}

// NewShutdownManagerAdapter creates a new adapter
func NewShutdownManagerAdapter(manager *shutdown.GracefulShutdownManager) *ShutdownManagerAdapter {
	return &ShutdownManagerAdapter{manager: manager}
}

// RegisterSecureString adapts the interface by converting *SecureString to interface{}
func (a *ShutdownManagerAdapter) RegisterSecureString(secureStr *security.SecureString, description string) {
	a.manager.RegisterSecureString(interface{}(secureStr), description)
}

// UnregisterSecureString adapts the interface by converting *SecureString to interface{}
func (a *ShutdownManagerAdapter) UnregisterSecureString(secureStr *security.SecureString) {
	a.manager.UnregisterSecureString(interface{}(secureStr))
}

// RegisterTempFile delegates to the shutdown manager
func (a *ShutdownManagerAdapter) RegisterTempFile(filePath string, description string) {
	a.manager.RegisterTempFile(filePath, description)
}

// RegisterClipboard delegates to the shutdown manager
func (a *ShutdownManagerAdapter) RegisterClipboard(description string) {
	a.manager.RegisterClipboard(description)
}

// IsShutdown delegates to the shutdown manager
func (a *ShutdownManagerAdapter) IsShutdown() bool {
	return a.manager.IsShutdown()
}

// GetResourceCount delegates to the shutdown manager
func (a *ShutdownManagerAdapter) GetResourceCount() int {
	return a.manager.GetResourceCount()
}

// InitializeIntegration sets up cross-package integrations
func InitializeIntegration() {
	// Set up security <-> shutdown integration
	// This avoids circular imports by doing the integration in a separate package
	
	// Set up dependency injection for shutdown manager
	shutdown.SetSecurityFunctions(
		security.SecureDeleteFile,  // Function to securely delete files
		security.ClearClipboard,    // Function to clear clipboard
	)
	
	// Set up resource manager integration using the adapter
	adapter := NewShutdownManagerAdapter(shutdown.GetManager())
	security.SetResourceManager(adapter)
}
