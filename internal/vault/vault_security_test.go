package vault

import (
	"strings"
	"testing"
)

// TestSanitizeLogOutput tests the enhanced sanitization function
func TestSanitizeLogOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string // patterns that should NOT be in output
		required []string // patterns that SHOULD be in output
	}{
		{
			name:  "PIN in error message",
			input: "Error: Please enter PIN: 123456\nOperation failed",
			expected: []string{"123456", "PIN"},
			required: []string{"[REDACTED SENSITIVE LINE]", "Operation failed"},
		},
		{
			name:  "YubiKey age identity",
			input: "# created: 2023-01-01\n# public key: age1yubikey1abcd1234...\nage1yubikey1abcd1234efgh5678ijkl9012mnop3456qrst7890uvwx",
			expected: []string{"age1yubikey1"},
			required: []string{"[REDACTED SENSITIVE LINE]"},
		},
		{
			name:  "PEM certificate block",
			input: "Certificate loading...\n-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJAKZ...content...\n-----END CERTIFICATE-----\nCertificate loaded",
			expected: []string{"-----BEGIN", "MIIBkTCB"},
			required: []string{"[REDACTED SENSITIVE LINE]", "Certificate loaded"},
		},
		{
			name:  "Hex key material",
			input: "Processing key: abcd1234567890abcdef1234567890abcdef1234567890abcdef\nKey processed successfully",
			expected: []string{"abcd1234567890abcdef"},
			required: []string{"[REDACTED SENSITIVE LINE]", "Key processed successfully"},
		},
		{
			name:  "Base64 key material",
			input: "Secret data: SGVsbG8gV29ybGQgdGhpcyBpcyBhIGxvbmcgYmFzZTY0IGVuY29kZWQgc3RyaW5nIHRoYXQgbWlnaHQgYmUgc2VjcmV0\nProcessing complete",
			expected: []string{"SGVsbG8gV29ybGQg"},
			required: []string{"[REDACTED SENSITIVE LINE]", "Processing complete"},
		},
		{
			name:  "YubiKey touch prompt",
			input: "Please touch your YubiKey\nOperation timeout\nPlease insert your YubiKey",
			expected: []string{"touch your yubikey", "insert your yubikey"},
			required: []string{"[REDACTED SENSITIVE LINE]", "Operation timeout"},
		},
		{
			name:  "Authentication failure with details",
			input: "Authentication failed: wrong PIN entered\nRetry authentication",
			expected: []string{"wrong PIN"},
			required: []string{"[REDACTED SENSITIVE LINE]", "Retry authentication"},
		},
		{
			name:  "Safe general error",
			input: "Network connection failed\nRetrying in 5 seconds\nOperation completed",
			expected: []string{}, // nothing should be redacted
			required: []string{"Network connection failed", "Retrying in 5 seconds", "Operation completed"},
		},
		{
			name:  "Empty and whitespace lines",
			input: "Line 1\n\n   \nLine 4",
			expected: []string{},
			required: []string{"Line 1", "Line 4"},
		},
		{
			name:  "Mixed sensitive and safe content",
			input: "Starting operation\nPIN verification required\nUser: testuser\ntoken: abc123secret\nOperation completed successfully",
			expected: []string{"PIN verification", "token: abc123secret"},
			required: []string{"Starting operation", "User: testuser", "Operation completed successfully", "[REDACTED SENSITIVE LINE]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeLogOutput(tt.input)
			
			// Check that expected patterns are NOT in the result
			for _, pattern := range tt.expected {
				if strings.Contains(strings.ToLower(result), strings.ToLower(pattern)) {
					t.Errorf("Sanitization failed: sensitive pattern '%s' found in result: %s", pattern, result)
				}
			}
			
			// Check that required patterns ARE in the result
			for _, pattern := range tt.required {
				if !strings.Contains(result, pattern) {
					t.Errorf("Sanitization too aggressive: required pattern '%s' not found in result: %s", pattern, result)
				}
			}
		})
	}
}

// TestIsHexString tests hex string detection
func TestIsHexString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid long hex", "abcd1234567890abcdef1234567890abcdef1234567890", true},
		{"Valid hex with spaces", "ab cd 12 34 56 78 90 ab cd ef 12 34 56 78 90 ab cd ef 12 34 56 78 90", true},
		{"Valid hex with colons", "ab:cd:12:34:56:78:90:ab:cd:ef:12:34:56:78:90:ab:cd:ef:12:34:56:78:90", true},
		{"Too short hex", "abcd1234", false},
		{"Invalid chars", "abcd1234567890ghij1234567890abcdef1234567890", false},
		{"Not hex at all", "Hello World this is not hex", false},
		{"Empty string", "", false},
		{"Mixed case valid", "ABcd1234567890abCDef1234567890ABcdef1234567890", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHexString(tt.input)
			if result != tt.expected {
				t.Errorf("isHexString(%s) = %v; expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIsBase64Like tests base64 detection
func TestIsBase64Like(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid base64", "SGVsbG8gV29ybGQgdGhpcyBpcyBhIGxvbmcgYmFzZTY0IGVuY29kZWQgc3RyaW5nIHRoYXQgbWlnaHQgYmUgc2VjcmV0", true},
		{"Base64 with padding", "SGVsbG8gV29ybGQ=", false}, // too short
		{"Long base64 with padding", "SGVsbG8gV29ybGQgdGhpcyBpcyBhIGxvbmcgYmFzZTY0IGVuY29kZWQgc3RyaW5nIHRoYXQ=", true},
		{"Too short", "SGVsbG8=", false},
		{"Not base64", "Hello World this is not base64 encoded content here", false},
		{"Mixed with invalid chars", "SGVsbG8gV29ybGQgdGhpcyBpcyBhIGxvbmcgYmFzZTY0IGVuY29kZWQgc3RyaW5nIHRoYXQg@#$", false},
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBase64Like(tt.input)
			if result != tt.expected {
				t.Errorf("isBase64Like(%s) = %v; expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestSanitizeLogOutputEdgeCases tests edge cases and potential bypasses
func TestSanitizeLogOutputEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		desc  string
	}{
		{
			name:  "Case sensitivity bypass attempt",
			input: "User PIN: 123456\nUSER pin: 789012\nPin Entry: 345678",
			desc:  "Should catch PIN in any case combination",
		},
		{
			name:  "Obfuscation with special chars",
			input: "P-I-N: 123456\np.i.n: 789012\nP_I_N: 345678",
			desc:  "Patterns with separators might still be sensitive",
		},
		{
			name:  "Age key variants",
			input: "age1yubikey123456\nAGE1YUBIKEY789012\nage-1-yubikey-345678",
			desc:  "Various age key formats should be detected",
		},
		{
			name:  "Partial key leakage",
			input: "Key material (partial): ...ef1234567890abcdef1234567890abcdef1234567890abcdef\nKey end detected",
			desc:  "Even partial key material should be redacted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeLogOutput(tt.input)
			
			// Should contain redaction markers
			if !strings.Contains(result, "[REDACTED SENSITIVE LINE]") {
				t.Errorf("Test '%s' failed: no redaction marker found. Description: %s\nInput: %s\nOutput: %s", 
					tt.name, tt.desc, tt.input, result)
			}
			
			// Should not contain obvious sensitive patterns
			lowerResult := strings.ToLower(result)
			sensitivePatterns := []string{"pin", "123456", "789012", "345678", "age1yubikey", "ef1234567890ab"}
			
			for _, pattern := range sensitivePatterns {
				if strings.Contains(lowerResult, pattern) {
					t.Errorf("Test '%s' failed: sensitive pattern '%s' not redacted. Description: %s\nResult: %s", 
						tt.name, pattern, tt.desc, result)
				}
			}
		})
	}
}

// BenchmarkSanitizeLogOutput benchmarks the sanitization function
func BenchmarkSanitizeLogOutput(b *testing.B) {
	testInput := `Operation starting
User authentication required
Please enter PIN: 123456
YubiKey detected: age1yubikey1abcd1234efgh5678ijkl9012mnop3456qrst7890uvwx
Certificate: -----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKZqJ5J5J5J5J5J5J5J5J5J5J5J5J5J5J5J5J5J5J5J5J5J5J5J5
-----END CERTIFICATE-----
Hex key material: abcd1234567890abcdef1234567890abcdef1234567890abcdef
Base64 data: SGVsbG8gV29ybGQgdGhpcyBpcyBhIGxvbmcgYmFzZTY0IGVuY29kZWQgc3RyaW5nIHRoYXQgbWlnaHQgYmUgc2VjcmV0
Operation completed successfully`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sanitizeLogOutput(testInput)
	}
}

// TestSanitizeLogOutputConcurrency tests thread safety
func TestSanitizeLogOutputConcurrency(t *testing.T) {
	testInput := "Please enter PIN: 123456\nage1yubikey1abcd1234\nOperation completed"
	
	// Run sanitization concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			result := sanitizeLogOutput(testInput)
			if strings.Contains(result, "123456") || strings.Contains(result, "age1yubikey1") {
				t.Errorf("Concurrent sanitization failed: sensitive data leaked")
			}
			done <- true
		}()
	}
	
	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
