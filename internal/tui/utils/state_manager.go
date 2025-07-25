// internal/tui/utils/state_manager.go
package utils

import (
	"sync"
	"vault.module/internal/vault"
)

// AppState представляет глобальное состояние приложения
type AppState struct {
	mu                sync.RWMutex
	currentVault      string
	vault             vault.Vault
	isAuthenticated   bool
	sessionTimeout    int64
	lastActivity      int64
	programmaticMode  bool
	terminalWidth     int
	terminalHeight    int
	theme             *Theme
}

// StateManager управляет глобальным состоянием приложения
type StateManager struct {
	state *AppState
}

var (
	stateManager *StateManager
	once         sync.Once
)

// GetStateManager возвращает singleton экземпляр StateManager
func GetStateManager() *StateManager {
	once.Do(func() {
		stateManager = &StateManager{
			state: &AppState{
				isAuthenticated:  false,
				sessionTimeout:   3600, // 1 час по умолчанию
				programmaticMode: false,
				theme:           GetDefaultTheme(),
			},
		}
	})
	return stateManager
}

// SetCurrentVault устанавливает текущий vault
func (sm *StateManager) SetCurrentVault(name string, v vault.Vault) {
	sm.state.mu.Lock()
	defer sm.state.mu.Unlock()
	
	sm.state.currentVault = name
	sm.state.vault = v
	sm.updateLastActivity()
}

// GetCurrentVault возвращает текущий vault
func (sm *StateManager) GetCurrentVault() (string, vault.Vault) {
	sm.state.mu.RLock()
	defer sm.state.mu.RUnlock()
	
	return sm.state.currentVault, sm.state.vault
}

// SetAuthenticated устанавливает статус аутентификации
func (sm *StateManager) SetAuthenticated(authenticated bool) {
	sm.state.mu.Lock()
	defer sm.state.mu.Unlock()
	
	sm.state.isAuthenticated = authenticated
	if authenticated {
		sm.updateLastActivity()
	}
}

// IsAuthenticated проверяет статус аутентификации
func (sm *StateManager) IsAuthenticated() bool {
	sm.state.mu.RLock()
	defer sm.state.mu.RUnlock()
	
	return sm.state.isAuthenticated
}

// SetTerminalSize устанавливает размер терминала
func (sm *StateManager) SetTerminalSize(width, height int) {
	sm.state.mu.Lock()
	defer sm.state.mu.Unlock()
	
	sm.state.terminalWidth = width
	sm.state.terminalHeight = height
}

// GetTerminalSize возвращает размер терминала
func (sm *StateManager) GetTerminalSize() (int, int) {
	sm.state.mu.RLock()
	defer sm.state.mu.RUnlock()
	
	return sm.state.terminalWidth, sm.state.terminalHeight
}

// SetTheme устанавливает тему
func (sm *StateManager) SetTheme(theme *Theme) {
	sm.state.mu.Lock()
	defer sm.state.mu.Unlock()
	
	sm.state.theme = theme
}

// GetTheme возвращает текущую тему
func (sm *StateManager) GetTheme() *Theme {
	sm.state.mu.RLock()
	defer sm.state.mu.RUnlock()
	
	return sm.state.theme
}

// UpdateLastActivity обновляет время последней активности
func (sm *StateManager) UpdateLastActivity() {
	sm.state.mu.Lock()
	defer sm.state.mu.Unlock()
	
	sm.updateLastActivity()
}

// updateLastActivity внутренний метод для обновления активности (без блокировки)
func (sm *StateManager) updateLastActivity() {
	// Здесь можно использовать time.Now().Unix(), но для простоты пока оставим 0
	sm.state.lastActivity = 0 // TODO: implement proper timestamp
}

// IsSessionExpired проверяет, истекла ли сессия
func (sm *StateManager) IsSessionExpired() bool {
	sm.state.mu.RLock()
	defer sm.state.mu.RUnlock()
	
	// TODO: implement proper session timeout logic
	return false
}

// ClearSession очищает сессию
func (sm *StateManager) ClearSession() {
	sm.state.mu.Lock()
	defer sm.state.mu.Unlock()
	
	sm.state.isAuthenticated = false
	sm.state.currentVault = ""
	sm.state.vault = nil
	sm.state.lastActivity = 0
}
