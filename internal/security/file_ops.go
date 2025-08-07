// internal/security/file_ops.go
package security

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
)

// SecureFileDelete securely deletes a file by overwriting it multiple times
func SecureFileDelete(filePath string) error {
	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, nothing to delete
		}
		return fmt.Errorf("failed to stat file %s: %v", filePath, err)
	}

	fileSize := fileInfo.Size()
	if fileSize == 0 {
		// Empty file, just delete it
		return os.Remove(filePath)
	}

	// Open file for writing
	file, err := os.OpenFile(filePath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open file %s for overwriting: %v", filePath, err)
	}
	defer file.Close()

	// Overwrite file with random data multiple times
	const overwritePasses = 3
	buffer := make([]byte, min(4096, int(fileSize))) // Use 4KB buffer or file size, whichever is smaller

	for pass := 0; pass < overwritePasses; pass++ {
		// Seek to beginning of file
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek to beginning of file %s: %v", filePath, err)
		}

		// Overwrite entire file
		remaining := fileSize
		for remaining > 0 {
			// Generate random data for this chunk
			chunkSize := int64(len(buffer))
			if remaining < chunkSize {
				chunkSize = remaining
			}

			randomChunk := buffer[:chunkSize]
			if _, err := rand.Read(randomChunk); err != nil {
				panic(fmt.Sprintf("CRITICAL: could not get cryptographic random data to securely delete file %s: %v", filePath, err))
			}

			// Write the random chunk
			if _, err := file.Write(randomChunk); err != nil {
				return fmt.Errorf("failed to overwrite file %s: %v", filePath, err)
			}

			remaining -= chunkSize
		}

		// Sync to ensure data is written to disk
		if err := file.Sync(); err != nil {
			return fmt.Errorf("failed to sync file %s: %v", filePath, err)
		}
	}

	// Close the file before deletion
	file.Close()

	// Finally, delete the file
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file %s: %v", filePath, err)
	}

	return nil
}

// SecureCreateTempFile creates a temporary file with secure permissions and registers it for cleanup
func SecureCreateTempFile(pattern string, content []byte) (string, error) {
	// Create temporary file
	tempFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}

	filePath := tempFile.Name()

	// Immediately set secure permissions (0600) for sensitive data
	if err := tempFile.Chmod(0600); err != nil {
		tempFile.Close()
		os.Remove(filePath) // Clean up on error
		return "", fmt.Errorf("failed to set secure permissions on temp file: %v", err)
	}

	// Write content if provided
	if len(content) > 0 {
		if _, err := tempFile.Write(content); err != nil {
			tempFile.Close()
			os.Remove(filePath) // Clean up on error
			return "", fmt.Errorf("failed to write to temp file: %v", err)
		}
	}

	// Close the file
	if err := tempFile.Close(); err != nil {
		os.Remove(filePath) // Clean up on error
		return "", fmt.Errorf("failed to close temp file: %v", err)
	}

	return filePath, nil
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
