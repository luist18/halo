package httpexecutor

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OrderedMap represents a map that preserves insertion order and allows duplicate keys.
// This is used to serialize SQL result rows with duplicate column names (e.g., multiple ?column?)
// as JSON objects with duplicate keys, matching the Neon serverless driver API behavior.
type OrderedMap struct {
	entries []entry
}

type entry struct {
	key   string
	value any
}

// NewOrderedMap creates a new OrderedMap instance
func NewOrderedMap() *OrderedMap {
	return &OrderedMap{
		entries: make([]entry, 0),
	}
}

// Set adds a key-value pair to the ordered map.
// Unlike regular maps, this allows duplicate keys.
func (om *OrderedMap) Set(key string, value any) {
	om.entries = append(om.entries, entry{key: key, value: value})
}

// MarshalJSON implements the json.Marshaler interface to produce JSON with duplicate keys.
// This manually constructs JSON to allow duplicate keys, which is technically non-standard
// but required for compatibility with certain APIs (like Neon's serverless driver).
func (om *OrderedMap) MarshalJSON() ([]byte, error) {
	if len(om.entries) == 0 {
		return []byte("{}"), nil
	}

	var sb strings.Builder
	sb.WriteString("{")

	for i, e := range om.entries {
		if i > 0 {
			sb.WriteString(",")
		}

		// Marshal the key
		keyJSON, err := json.Marshal(e.key)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal key %q: %w", e.key, err)
		}
		sb.Write(keyJSON)
		sb.WriteString(":")

		// Marshal the value
		valueJSON, err := json.Marshal(e.value)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal value for key %q: %w", e.key, err)
		}
		sb.Write(valueJSON)
	}

	sb.WriteString("}")
	return []byte(sb.String()), nil
}

