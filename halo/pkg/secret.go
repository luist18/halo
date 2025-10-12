package pkg

// Secret is a wrapper around a string that redacts the value in the String() method.
// It is used to store sensitive information like connection strings.
type Secret struct {
	// value is the underlying string value.
	value string
}

// NewSecret creates a new Secret wrapper around the given string.
func NewSecret(val string) *Secret {
	return &Secret{
		value: val,
	}
}

// String returns a redacted string representation of the Secret.
func (s Secret) String() string {
	return "<redacted>"
}

// Unwrap returns the underlying string value of the Secret.
func (s Secret) Unwrap() string {
	return s.value
}
