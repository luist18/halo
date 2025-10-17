package secret

import "testing"

func TestNewSecret(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "non-empty string",
			input: "my-secret-connection-string",
		},
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "sensitive data",
			input: "postgres://user:password@localhost:5432/db",
		},
		{
			name:  "special characters",
			input: "secret!@#$%^&*()_+-=[]{}|;:,.<>?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := NewSecret(tt.input)
			if secret == nil {
				t.Fatal("NewSecret returned nil")
			}
			if secret.value != tt.input {
				t.Errorf("NewSecret stored value = %v, want %v", secret.value, tt.input)
			}
		})
	}
}

func TestSecret_String(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "normal string",
			value: "my-secret-value",
		},
		{
			name:  "empty string",
			value: "",
		},
		{
			name:  "sensitive connection string",
			value: "postgres://user:password@localhost:5432/db",
		},
		{
			name:  "special characters",
			value: "!@#$%^&*()_+-=[]{}|;:,.<>?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := NewSecret(tt.value)
			result := secret.String()
			expected := "<redacted>"
			if result != expected {
				t.Errorf("String() = %v, want %v", result, expected)
			}
		})
	}
}

func TestSecret_Unwrap(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "normal string",
			value: "my-secret-value",
		},
		{
			name:  "empty string",
			value: "",
		},
		{
			name:  "connection string with credentials",
			value: "postgres://user:password@localhost:5432/db",
		},
		{
			name:  "special characters",
			value: "secret!@#$%^&*()_+-=[]{}|;:,.<>?",
		},
		{
			name:  "unicode characters",
			value: "secret with 日本語 and émojis 🔒",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := NewSecret(tt.value)
			result := secret.Unwrap()
			if result != tt.value {
				t.Errorf("Unwrap() = %v, want %v", result, tt.value)
			}
		})
	}
}
