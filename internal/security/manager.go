// internal/security/manager.go
package security

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// --- GracefulShutdownManager Implementation ---

// CleanupResource представляет ресурс, который нуждается в очистке во время завершения работы
type CleanupResource interface {
	Cleanup() error
	Description() string
}

// SecureStringResource оборачивает SecureString для очистки
// Использует interface{} чтобы избежать циклического импорта
type SecureStringResource struct {
	secureStr   interface{} // Должен быть *security.SecureString
	description string
}

func (r *SecureStringResource) Cleanup() error {
	// Используем утверждение типа и рефлексию для вызова метода Clear
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

// TempFileResource представляет временный файл, который содержит чувствительные данные
type TempFileResource struct {
	filePath    string
	description string
}

func (r *TempFileResource) Cleanup() error {
	if _, err := os.Stat(r.filePath); err == nil {
		if err := SecureFileDelete(r.filePath); err != nil {
			return fmt.Errorf("failed to securely delete %s: %v", r.filePath, err)
		}
	}
	return nil
}

func (r *TempFileResource) Description() string {
	return r.description
}

// ClipboardResource обрабатывает очистку буфера обмена
type ClipboardResource struct {
	description string
}

func (r *ClipboardResource) Cleanup() error {
	return ClearClipboard()
}

func (r *ClipboardResource) Description() string {
	return r.description
}

// GracefulShutdownManager обрабатывает корректное завершение работы и очистку ресурсов
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
	// Глобальный экземпляр
	globalManager *GracefulShutdownManager
	managerOnce   sync.Once
)

// GetManager возвращает глобальный экземпляр менеджера завершения работы
func GetManager() *GracefulShutdownManager {
	managerOnce.Do(func() {
		globalManager = newManager()
	})
	return globalManager
}

// newManager создает новый менеджер завершения работы
func newManager() *GracefulShutdownManager {
	ctx, cancel := context.WithCancel(context.Background())

	manager := &GracefulShutdownManager{
		resources: make([]CleanupResource, 0),
		ctx:       ctx,
		cancel:    cancel,
		signals:   make(chan os.Signal, 1),
	}

	// Регистрируем обработчики сигналов
	signal.Notify(manager.signals,
		syscall.SIGINT,  // Ctrl+C
		syscall.SIGTERM, // Запрос на завершение
		syscall.SIGQUIT, // Запрос на выход
	)

	// Запускаем горутину мониторинга сигналов
	go manager.signalHandler()

	return manager
}

// signalHandler обрабатывает входящие сигналы завершения работы
func (m *GracefulShutdownManager) signalHandler() {
	select {
	case sig := <-m.signals:
		fmt.Fprintf(os.Stderr, "\nReceived signal %v, initiating graceful shutdown...\n", sig)
		m.Shutdown()
	case <-m.ctx.Done():
		// Контекст отменен, выходим корректно
		return
	}
}

// RegisterSecureString регистрирует SecureString для очистки
func (m *GracefulShutdownManager) RegisterSecureString(secureStr interface{}, description string) {
	if secureStr == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isShutdown {
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

// RegisterTempFile регистрирует временный файл для безопасной очистки
func (m *GracefulShutdownManager) RegisterTempFile(filePath string, description string) {
	if filePath == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isShutdown {
		SecureFileDelete(filePath)
		return
	}

	resource := &TempFileResource{
		filePath:    filePath,
		description: description,
	}

	m.resources = append(m.resources, resource)
}

// RegisterClipboard регистрирует буфер обмена для очистки
func (m *GracefulShutdownManager) RegisterClipboard(description string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isShutdown {
		ClearClipboard()
		return
	}

	resource := &ClipboardResource{
		description: description,
	}

	m.resources = append(m.resources, resource)
}

// UnregisterSecureString удаляет SecureString из реестра очистки
func (m *GracefulShutdownManager) UnregisterSecureString(secureStr interface{}) {
	if secureStr == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for i, resource := range m.resources {
		if ssResource, ok := resource.(*SecureStringResource); ok {
			if ssResource.secureStr == secureStr {
				m.resources = append(m.resources[:i], m.resources[i+1:]...)
				break
			}
		}
	}
}

// Shutdown выполняет корректное завершение работы и очистку всех ресурсов
func (m *GracefulShutdownManager) Shutdown() {
	m.shutdownOnce.Do(func() {
		m.mu.Lock()
		m.isShutdown = true
		m.mu.Unlock()

		fmt.Fprintln(os.Stderr, "Cleaning up sensitive resources...")
		m.cleanupResources()
		m.cancel()
		fmt.Fprintln(os.Stderr, "Graceful shutdown completed.")
	})
}

// cleanupResources очищает все зарегистрированные ресурсы
func (m *GracefulShutdownManager) cleanupResources() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, resource := range m.resources {
		if err := resource.Cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "Cleanup error for '%s': %v\n", resource.Description(), err)
		}
	}
}

// IsShutdown возвращает true, если было инициировано завершение работы
func (m *GracefulShutdownManager) IsShutdown() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isShutdown
}

// Context возвращает контекст завершения работы
func (m *GracefulShutdownManager) Context() context.Context {
	return m.ctx
}

// GetResourceCount returns the number of registered resources
func (m *GracefulShutdownManager) GetResourceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.resources)
}

// --- Global Helper Functions ---

// RegisterTempFileGlobal регистрирует временный файл для безопасной очистки
func RegisterTempFileGlobal(filePath string, description string) {
	GetManager().RegisterTempFile(filePath, description)
}

// RegisterClipboardGlobal регистрирует буфер обмена для очистки
func RegisterClipboardGlobal(description string) {
	GetManager().RegisterClipboard(description)
}

// IsShuttingDown возвращает true, если было инициировано завершение работы
func IsShuttingDown() bool {
	return GetManager().IsShutdown()
}

// GetResourceCount возвращает количество зарегистрированных ресурсов
func GetResourceCount() int {
	return GetManager().GetResourceCount()
}
