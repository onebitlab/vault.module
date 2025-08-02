//go:build windows
// +build windows

// internal/security/secure_string_windows.go
package security

import (
	"crypto/rand"
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procVirtualLock  = kernel32.NewProc("VirtualLock")
	procVirtualUnlock = kernel32.NewProc("VirtualUnlock")
	procGetSystemInfo = kernel32.NewProc("GetSystemInfo")
)

type systemInfo struct {
	ProcessorArchitecture     uint16
	Reserved                  uint16
	PageSize                  uint32
	MinimumApplicationAddress uintptr
	MaximumApplicationAddress uintptr
	ActiveProcessorMask       uintptr
	NumberOfProcessors        uint32
	ProcessorType             uint32
	AllocationGranularity     uint32
	ProcessorLevel            uint16
	ProcessorRevision         uint16
}

// Platform-specific memory locking implementation for Windows
func (s *SecureString) lockMemory() error {
	if len(s.data) == 0 {
		return nil
	}
	
	// Lock data pages in memory using VirtualLock
	ret, _, err := procVirtualLock.Call(
		uintptr(unsafe.Pointer(&s.data[0])),
		uintptr(len(s.data)),
	)
	if ret == 0 {
		return err
	}
	
	if len(s.pad) > 0 {
		ret, _, err := procVirtualLock.Call(
			uintptr(unsafe.Pointer(&s.pad[0])),
			uintptr(len(s.pad)),
		)
		if ret == 0 {
			// If locking pad fails, unlock data and return error
			procVirtualUnlock.Call(
				uintptr(unsafe.Pointer(&s.data[0])),
				uintptr(len(s.data)),
			)
			return err
		}
	}
	
	s.locked = true
	return nil
}

func (s *SecureString) unlockMemory() error {
	if !s.locked {
		return nil
	}
	
	var unlockErr error
	
	if len(s.data) > 0 {
		ret, _, err := procVirtualUnlock.Call(
			uintptr(unsafe.Pointer(&s.data[0])),
			uintptr(len(s.data)),
		)
		if ret == 0 {
			unlockErr = err
		}
	}
	
	if len(s.pad) > 0 {
		ret, _, err := procVirtualUnlock.Call(
			uintptr(unsafe.Pointer(&s.pad[0])),
			uintptr(len(s.pad)),
		)
		if ret == 0 && unlockErr == nil {
			unlockErr = err
		}
	}
	
	s.locked = false
	return unlockErr
}

// SecureClearBytes securely clears sensitive data from a byte slice using multiple pass overwriting
func SecureClearBytes(data []byte) {
	secureZero(data)
}

// secureZero overwrites memory with zeros multiple times for enhanced security
func secureZero(data []byte) {
	if len(data) == 0 {
		return
	}
	
	// Multiple pass overwriting for enhanced security
	// Pass 1: Random data
	rand.Read(data)
	
	// Pass 2: All ones
	for i := range data {
		data[i] = 0xFF
	}
	
	// Pass 3: All zeros
	for i := range data {
		data[i] = 0x00
	}
	
	// Pass 4: Random data again
	rand.Read(data)
	
	// Pass 5: Final zero
	for i := range data {
		data[i] = 0x00
	}
}

// getPageSize returns the system page size for memory alignment
func getPageSize() int {
	var si systemInfo
	procGetSystemInfo.Call(uintptr(unsafe.Pointer(&si)))
	return int(si.PageSize)
}


