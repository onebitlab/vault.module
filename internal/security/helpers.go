// internal/security/helpers.go
package security

import (
	"crypto/rand"
	"os"
	mathrand "math/rand"
	"time"
)

// SecureDeleteFile securely deletes a file by overwriting it first
func SecureDeleteFile(filePath string) error {
	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, consider it deleted
		}
		return err
	}
	
	// Open file for writing
	file, err := os.OpenFile(filePath, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	
	// Get file size
	size := fileInfo.Size()
	
	// Overwrite with random data multiple times
	for pass := 0; pass < 3; pass++ {
		// Seek to beginning
		if _, err := file.Seek(0, 0); err != nil {
			break
		}
		
		// Generate random data
		randomData := make([]byte, size)
		if _, err := rand.Read(randomData); err != nil {
			// Fallback to math/rand
			r := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
			for i := range randomData {
				randomData[i] = byte(r.Intn(256))
			}
		}
		
		// Write random data
		if _, err := file.Write(randomData); err != nil {
			break
		}
		
		// Sync to disk
		file.Sync()
		
		// Clear random data from memory
		SecureZero(randomData)
	}
	
	// Close file before deletion
	file.Close()
	
	// Finally remove the file
	return os.Remove(filePath)
}

// CreateTempFileWithAutoCleanup creates a temporary file and registers it for cleanup
func CreateTempFileWithAutoCleanup(pattern string, content []byte, description string) (string, error) {
	filePath, err := SecureCreateTempFile(pattern, content)
	if err != nil {
		return "", err
	}
	
	// Register with resource manager for cleanup
	if manager := GetResourceManager(); manager != nil {
		manager.RegisterTempFile(filePath, description)
	}
	
	return filePath, nil
}

// Integration helper for shutdown package
func InitShutdownIntegration() {
	// This function can be called to set up integration with shutdown package
	// The actual integration is done through dependency injection in shutdown package
}
