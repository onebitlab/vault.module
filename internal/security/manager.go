// internal/security/manager.go
package security

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
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

// cleanupResourceWithTimeout выполняет очистку отдельного ресурса с таймаутом
func (m *GracefulShutdownManager) cleanupResourceWithTimeout(ctx context.Context, resource CleanupResource, resultsCh chan<- struct {
	resource CleanupResource
	err      error
}) {
	// Создаём канал для результата операции очистки
	cleanupDone := make(chan error, 1)

	// Запускаем очистку в отдельной горутине
	go func() {
		cleanupDone <- resource.Cleanup()
	}()

	// Ждём завершения или таймаута
	select {
	case err := <-cleanupDone:
		resultsCh <- struct {
			resource CleanupResource
			err      error
		}{resource: resource, err: err}
	case <-ctx.Done():
		resultsCh <- struct {
			resource CleanupResource
			err      error
		}{resource: resource, err: fmt.Errorf("cleanup timeout for resource '%s'", resource.Description())}
	}
}

// RegisterSecureString регистрирует SecureString для очистки с таймаутом
func (m *GracefulShutdownManager) RegisterSecureString(secureStr interface{}, description string) {
	if secureStr == nil {
		return
	}

	// Попытка получить блокировку с таймаутом
	lockAcquired := make(chan struct{}, 1)
	go func() {
		m.mu.Lock()
		lockAcquired <- struct{}{}
	}()

	select {
	case <-lockAcquired:
		defer m.mu.Unlock()
	case <-time.After(2 * time.Second):
		// Таймаут получения блокировки - немедленная очистка
		fmt.Fprintf(os.Stderr, "WARNING: registration timeout, cleaning resource immediately\n")
		if clearable, ok := secureStr.(interface{ Clear() }); ok {
			clearable.Clear()
		}
		return
	}

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

// UnregisterSecureString удаляет SecureString из реестра очистки с таймаутом
func (m *GracefulShutdownManager) UnregisterSecureString(secureStr interface{}) {
	if secureStr == nil {
		return
	}

	// Попытка получить блокировку с таймаутом
	lockAcquired := make(chan struct{}, 1)
	go func() {
		m.mu.Lock()
		lockAcquired <- struct{}{}
	}()

	select {
	case <-lockAcquired:
		defer m.mu.Unlock()
		// Продолжаем с удалением
	case <-time.After(2 * time.Second):
		// Таймаут - просто выходим, ресурс останется зарегистрированным
		fmt.Fprintf(os.Stderr, "WARNING: unregistration timeout, resource remains registered\n")
		return
	}

	for i, resource := range m.resources {
		if ssResource, ok := resource.(*SecureStringResource); ok {
			if ssResource.secureStr == secureStr {
				m.resources = append(m.resources[:i], m.resources[i+1:]...)
				break
			}
		}
	}
}

// Shutdown выполняет корректное завершение работы и очистку всех ресурсов с улучшенной обработкой ошибок
func (m *GracefulShutdownManager) Shutdown() {
	m.shutdownOnce.Do(func() {
		// Попытка установить флаг shutdown с таймаутом
		shutdownFlagSet := make(chan struct{}, 1)
		go func() {
			m.mu.Lock()
			m.isShutdown = true
			m.mu.Unlock()
			shutdownFlagSet <- struct{}{}
		}()

		select {
		case <-shutdownFlagSet:
			// Флаг успешно установлен
		case <-time.After(5 * time.Second):
			fmt.Fprintf(os.Stderr, "WARNING: failed to set shutdown flag, continuing with cleanup\n")
		}

		fmt.Fprintln(os.Stderr, "Cleaning up sensitive resources...")
		
		// Создаём контекст с таймаутом для всей операции shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer shutdownCancel()

		// Запускаем очистку в отдельной горутине
		cleanupDone := make(chan struct{}, 1)
		go func() {
			m.cleanupResources()
			cleanupDone <- struct{}{}
		}()

		// Ждём завершения очистки или таймаута
		select {
		case <-cleanupDone:
			fmt.Fprintln(os.Stderr, "Resource cleanup completed successfully.")
		case <-shutdownCtx.Done():
			fmt.Fprintln(os.Stderr, "WARNING: cleanup operation timed out, forcing shutdown.")
		}

		m.cancel()
		fmt.Fprintln(os.Stderr, "Graceful shutdown completed.")
	})
}

// cleanupResources очищает все зарегистрированные ресурсы с таймаутами
func (m *GracefulShutdownManager) cleanupResources() {
	m.mu.RLock()
	resources := make([]CleanupResource, len(m.resources))
	copy(resources, m.resources)
	m.mu.RUnlock()

	// Создаём контекст с таймаутом для общей операции очистки
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Используем каналы для параллельной очистки с таймаутом
	resultsCh := make(chan struct{
		resource CleanupResource
		err      error
	}, len(resources))

	// Запускаем очистку ресурсов параллельно
	for _, resource := range resources {
		go m.cleanupResourceWithTimeout(ctx, resource, resultsCh)
	}

	// Собираем результаты
	for i := 0; i < len(resources); i++ {
		select {
		case result := <-resultsCh:
			if result.err != nil {
				fmt.Fprintf(os.Stderr, "Cleanup error for '%s': %v\n", result.resource.Description(), result.err)
			}
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "Cleanup operation timed out\n")
			return
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
