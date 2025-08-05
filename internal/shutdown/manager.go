// internal/shutdown/manager.go
package shutdown

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// CleanupResource represents a resource that needs cleanup during shutdown
type CleanupResource interface {
	Cleanup() error
	Description() string
}

// SecureStringResource wraps SecureString for cleanup
// Uses interface{} to avoid circular import
type SecureStringResource struct {
	secureStr   interface{} // Should be *security.SecureString
	description string
}

func (r *SecureStringResource) Cleanup() error {
	// Use type assertion and reflection to call Clear method
	if r.secureStr != nil {
		if clearable, ok := r.secureStr.(interface{ Clear() }); ok {
			clearable.Clear()
		}
	}
	return nil
}

func (r *SecureStringResource) Description() string {
	return r.description
}

// TempFileResource represents a temporary file that contains sensitive data
type TempFileResource struct {
	filePath    string
	description string
}

func (r *TempFileResource) Cleanup() error {
	if _, err := os.Stat(r.filePath); err == nil {
		// Overwrite file with random data before deletion for security
		// Use dependency injection for secure file deletion
		if err := secureFileDeleteFunc(r.filePath); err != nil {
			return fmt.Errorf("failed to securely delete %s: %v", r.filePath, err)
		}
	}
	return nil
}

func (r *TempFileResource) Description() string {
	return r.description
}

// ClipboardResource handles clipboard cleanup
type ClipboardResource struct {
	description string
}

func (r *ClipboardResource) Cleanup() error {
	// Use dependency injection for clipboard clearing
	return clearClipboardFunc()
}

func (r *ClipboardResource) Description() string {
	return r.description
}

// GracefulShutdownManager handles graceful shutdown and resource cleanup
type GracefulShutdownManager struct {
	resources    []CleanupResource
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	shutdownOnce sync.Once
	isShutdown   bool
	signals      chan os.Signal
}

var (
	// Global instance
	globalManager *GracefulShutdownManager
	managerOnce   sync.Once
)

// GetManager returns the global shutdown manager instance
func GetManager() *GracefulShutdownManager {
	managerOnce.Do(func() {
		globalManager = newManager()
		// Initialize dependency injection for security functions
		initSecurityIntegration()
	})
	return globalManager
}

// newManager creates a new shutdown manager
func newManager() *GracefulShutdownManager {
	ctx, cancel := context.WithCancel(context.Background())

	manager := &GracefulShutdownManager{
		resources: make([]CleanupResource, 0),
		ctx:       ctx,
		cancel:    cancel,
		signals:   make(chan os.Signal, 1),
	}

	// Register signal handlers
	signal.Notify(manager.signals,
		syscall.SIGINT,  // Ctrl+C
		syscall.SIGTERM, // Termination request
		syscall.SIGQUIT, // Quit request
	)

	// Start signal monitoring goroutine
	go manager.signalHandler()

	return manager
}

// signalHandler handles incoming shutdown signals
func (m *GracefulShutdownManager) signalHandler() {
	select {
	case sig := <-m.signals:
		fmt.Fprintf(os.Stderr, "\nReceived signal %v, initiating graceful shutdown...\n", sig)
		m.Shutdown()
	case <-m.ctx.Done():
		// Context cancelled, exit gracefully
		return
	}
}

// RegisterSecureString registers a SecureString for cleanup
// Uses interface{} to avoid circular import - expects *security.SecureString
func (m *GracefulShutdownManager) RegisterSecureString(secureStr interface{}, description string) {
	if secureStr == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isShutdown {
		// Already shutting down, clean immediately
		if clearable, ok := secureStr.(interface{ Clear() }); ok {
			clearable.Clear()
		}
		return
	}

	resource := &SecureStringResource{
		secureStr:   secureStr,
		description: description,
	}

	m.resources = append(m.resources, resource)
}

// RegisterTempFile registers a temporary file for secure cleanup
func (m *GracefulShutdownManager) RegisterTempFile(filePath string, description string) {
	if filePath == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isShutdown {
		// Already shutting down, clean immediately
		secureFileDeleteFunc(filePath)
		return
	}

	resource := &TempFileResource{
		filePath:    filePath,
		description: description,
	}

	m.resources = append(m.resources, resource)
}

// RegisterClipboard registers clipboard for cleanup
func (m *GracefulShutdownManager) RegisterClipboard(description string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isShutdown {
		// Already shutting down, clean immediately
		clearClipboardFunc()
		return
	}

	resource := &ClipboardResource{
		description: description,
	}

	m.resources = append(m.resources, resource)
}

// RegisterCustomResource registers a custom cleanup resource
func (m *GracefulShutdownManager) RegisterCustomResource(resource CleanupResource) {
	if resource == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isShutdown {
		// Already shutting down, clean immediately
		resource.Cleanup()
		return
	}

	m.resources = append(m.resources, resource)
}

// UnregisterSecureString removes a SecureString from cleanup registry
// Uses interface{} to avoid circular import - expects *security.SecureString
func (m *GracefulShutdownManager) UnregisterSecureString(secureStr interface{}) {
	if secureStr == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Find and remove the resource
	for i, resource := range m.resources {
		if ssResource, ok := resource.(*SecureStringResource); ok {
			if ssResource.secureStr == secureStr {
				// Remove from slice
				m.resources = append(m.resources[:i], m.resources[i+1:]...)
				break
			}
		}
	}
}

// Shutdown performs graceful shutdown and cleanup of all resources
func (m *GracefulShutdownManager) Shutdown() {
	m.shutdownOnce.Do(func() {
		m.mu.Lock()
		m.isShutdown = true
		m.mu.Unlock()

		fmt.Fprintln(os.Stderr, "Cleaning up sensitive resources...")

		// Create a timeout context for cleanup
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()

		// Cleanup all resources concurrently with timeout
		m.cleanupResources(cleanupCtx)

		// Cancel the main context
		m.cancel()

		fmt.Fprintln(os.Stderr, "Graceful shutdown completed.")
	})
}

// cleanupResources cleans up all registered resources
func (m *GracefulShutdownManager) cleanupResources(ctx context.Context) {
	m.mu.RLock()
	resources := make([]CleanupResource, len(m.resources))
	copy(resources, m.resources)
	m.mu.RUnlock()

	if len(resources) == 0 {
		return
	}

	// Create a worker pool for concurrent cleanup
	const maxWorkers = 10
	workers := len(resources)
	if workers > maxWorkers {
		workers = maxWorkers
	}

	resourceChan := make(chan CleanupResource, len(resources))
	resultChan := make(chan error, len(resources))

	// Start workers
	for i := 0; i < workers; i++ {
		go func() {
			for resource := range resourceChan {
				select {
				case <-ctx.Done():
					resultChan <- fmt.Errorf("cleanup timeout for %s", resource.Description())
					return
				default:
					if err := resource.Cleanup(); err != nil {
						resultChan <- fmt.Errorf("failed to cleanup %s: %v", resource.Description(), err)
					} else {
						resultChan <- nil
					}
				}
			}
		}()
	}

	// Send resources to workers
	go func() {
		defer close(resourceChan)
		for _, resource := range resources {
			select {
			case resourceChan <- resource:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Collect results
	cleanupErrors := make([]error, 0)
	for i := 0; i < len(resources); i++ {
		select {
		case err := <-resultChan:
			if err != nil {
				cleanupErrors = append(cleanupErrors, err)
				fmt.Fprintf(os.Stderr, "Cleanup error: %v\n", err)
			}
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "Cleanup timeout reached, forcing exit\n")
			return
		}
	}

	if len(cleanupErrors) > 0 {
		fmt.Fprintf(os.Stderr, "Completed cleanup with %d errors\n", len(cleanupErrors))
	} else {
		fmt.Fprintf(os.Stderr, "All resources cleaned successfully\n")
	}

	// Clear the resources list
	m.mu.Lock()
	m.resources = m.resources[:0]
	m.mu.Unlock()
}

// GetResourceCount returns the number of registered resources
func (m *GracefulShutdownManager) GetResourceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.resources)
}

// IsShutdown returns true if shutdown has been initiated
func (m *GracefulShutdownManager) IsShutdown() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isShutdown
}

// Context returns the shutdown context
func (m *GracefulShutdownManager) Context() context.Context {
	return m.ctx
}

// ForceShutdown performs immediate shutdown without cleanup (emergency use only)
func (m *GracefulShutdownManager) ForceShutdown() {
	m.cancel()
	os.Exit(1)
}

// --- Dependency Injection for Security Functions ---

var (
	secureFileDeleteFunc func(string) error
	clearClipboardFunc   func() error
)

// SetSecurityFunctions sets the dependency injection functions for security operations
func SetSecurityFunctions(
	secureFileDelete func(string) error,
	clearClipboard func() error,
) {
	secureFileDeleteFunc = secureFileDelete
	clearClipboardFunc = clearClipboard
}

// initSecurityIntegration initializes the integration with security package
func initSecurityIntegration() {
	// Set default no-op functions to prevent panics
	if secureFileDeleteFunc == nil {
		secureFileDeleteFunc = func(path string) error {
			return os.Remove(path) // Fallback to simple deletion
		}
	}
	if clearClipboardFunc == nil {
		clearClipboardFunc = func() error {
			return nil // No-op fallback
		}
	}
}
