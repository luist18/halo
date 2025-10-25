package httpexecutor

import (
	"encoding/json"
	"testing"
)

func TestOrderedMap_NewOrderedMap(t *testing.T) {
	om := NewOrderedMap()
	if om == nil {
		t.Fatal("NewOrderedMap() returned nil")
	}
	if om.entries == nil {
		t.Error("NewOrderedMap() entries slice is nil")
	}
	if len(om.entries) != 0 {
		t.Errorf("NewOrderedMap() entries length = %d, want 0", len(om.entries))
	}
}

func TestOrderedMap_Set(t *testing.T) {
	om := NewOrderedMap()
	om.Set("key1", "value1")
	om.Set("key2", 42)
	om.Set("key1", "value3") // Duplicate key

	if len(om.entries) != 3 {
		t.Errorf("Set() entries length = %d, want 3", len(om.entries))
	}

	expected := []entry{
		{key: "key1", value: "value1"},
		{key: "key2", value: 42},
		{key: "key1", value: "value3"},
	}

	for i, e := range om.entries {
		if e.key != expected[i].key {
			t.Errorf("entry[%d].key = %q, want %q", i, e.key, expected[i].key)
		}
		if e.value != expected[i].value {
			t.Errorf("entry[%d].value = %v, want %v", i, e.value, expected[i].value)
		}
	}
}

func TestOrderedMap_MarshalJSON_Empty(t *testing.T) {
	om := NewOrderedMap()
	result, err := om.MarshalJSON()
	if err != nil {
		t.Errorf("MarshalJSON() error = %v, want nil", err)
	}
	expected := "{}"
	if string(result) != expected {
		t.Errorf("MarshalJSON() = %q, want %q", string(result), expected)
	}
}

func TestOrderedMap_MarshalJSON_SingleEntry(t *testing.T) {
	om := NewOrderedMap()
	om.Set("name", "Alice")

	result, err := om.MarshalJSON()
	if err != nil {
		t.Errorf("MarshalJSON() error = %v, want nil", err)
	}

	expected := `{"name":"Alice"}`
	if string(result) != expected {
		t.Errorf("MarshalJSON() = %q, want %q", string(result), expected)
	}
}

func TestOrderedMap_MarshalJSON_MultipleUniqueKeys(t *testing.T) {
	om := NewOrderedMap()
	om.Set("name", "Alice")
	om.Set("age", 30)
	om.Set("active", true)

	result, err := om.MarshalJSON()
	if err != nil {
		t.Errorf("MarshalJSON() error = %v, want nil", err)
	}

	expected := `{"name":"Alice","age":30,"active":true}`
	if string(result) != expected {
		t.Errorf("MarshalJSON() = %q, want %q", string(result), expected)
	}
}

func TestOrderedMap_MarshalJSON_DuplicateKeys(t *testing.T) {
	om := NewOrderedMap()
	om.Set("?column?", 1)
	om.Set("?column?", "Alice")
	om.Set("?column?", 30)

	result, err := om.MarshalJSON()
	if err != nil {
		t.Errorf("MarshalJSON() error = %v, want nil", err)
	}

	// This is the key test - JSON with duplicate keys
	expected := `{"?column?":1,"?column?":"Alice","?column?":30}`
	if string(result) != expected {
		t.Errorf("MarshalJSON() = %q, want %q", string(result), expected)
	}
}

func TestOrderedMap_MarshalJSON_ComplexValues(t *testing.T) {
	om := NewOrderedMap()
	om.Set("name", "Bob")
	om.Set("data", map[string]any{"nested": "value"})
	om.Set("numbers", []int{1, 2, 3})
	om.Set("nil_value", nil)

	result, err := om.MarshalJSON()
	if err != nil {
		t.Errorf("MarshalJSON() error = %v, want nil", err)
	}

	expected := `{"name":"Bob","data":{"nested":"value"},"numbers":[1,2,3],"nil_value":null}`
	if string(result) != expected {
		t.Errorf("MarshalJSON() = %q, want %q", string(result), expected)
	}
}

func TestOrderedMap_MarshalJSON_SpecialCharactersInKey(t *testing.T) {
	om := NewOrderedMap()
	om.Set("key with spaces", "value1")
	om.Set("key\"with\"quotes", "value2")
	om.Set("key\nwith\nnewlines", "value3")

	result, err := om.MarshalJSON()
	if err != nil {
		t.Errorf("MarshalJSON() error = %v, want nil", err)
	}

	// Verify it's valid JSON by unmarshaling (will only parse the first key due to duplicates)
	var temp map[string]any
	if err := json.Unmarshal(result, &temp); err != nil {
		t.Errorf("Result is not valid JSON: %v", err)
	}
}

func TestOrderedMap_MarshalJSON_SpecialCharactersInValue(t *testing.T) {
	om := NewOrderedMap()
	om.Set("text", "value with \"quotes\" and \n newlines")
	om.Set("unicode", "Hello 世界 🌍")

	result, err := om.MarshalJSON()
	if err != nil {
		t.Errorf("MarshalJSON() error = %v, want nil", err)
	}

	// Verify proper escaping
	expected := `{"text":"value with \"quotes\" and \n newlines","unicode":"Hello 世界 🌍"}`
	if string(result) != expected {
		t.Errorf("MarshalJSON() = %q, want %q", string(result), expected)
	}
}

func TestOrderedMap_MarshalJSON_PreservesOrder(t *testing.T) {
	om := NewOrderedMap()
	om.Set("z", 1)
	om.Set("a", 2)
	om.Set("m", 3)

	result, err := om.MarshalJSON()
	if err != nil {
		t.Errorf("MarshalJSON() error = %v, want nil", err)
	}

	// Order should be preserved (z, a, m) not alphabetical
	expected := `{"z":1,"a":2,"m":3}`
	if string(result) != expected {
		t.Errorf("MarshalJSON() = %q, want %q (order not preserved)", string(result), expected)
	}
}

func TestOrderedMap_JSONMarshal_Integration(t *testing.T) {
	// Test that json.Marshal works with OrderedMap
	om := NewOrderedMap()
	om.Set("?column?", 1)
	om.Set("?column?", "Alice")
	om.Set("?column?", 30)

	result, err := json.Marshal(om)
	if err != nil {
		t.Errorf("json.Marshal() error = %v, want nil", err)
	}

	expected := `{"?column?":1,"?column?":"Alice","?column?":30}`
	if string(result) != expected {
		t.Errorf("json.Marshal() = %q, want %q", string(result), expected)
	}
}

func TestOrderedMap_JSONMarshal_InStruct(t *testing.T) {
	// Test OrderedMap when used in a struct (like ExecutorResponse)
	type TestResponse struct {
		Row *OrderedMap `json:"row"`
	}

	om := NewOrderedMap()
	om.Set("id", 1)
	om.Set("name", "test")

	resp := TestResponse{Row: om}
	result, err := json.Marshal(resp)
	if err != nil {
		t.Errorf("json.Marshal() error = %v, want nil", err)
	}

	expected := `{"row":{"id":1,"name":"test"}}`
	if string(result) != expected {
		t.Errorf("json.Marshal() = %q, want %q", string(result), expected)
	}
}

func TestOrderedMap_JSONMarshal_InSlice(t *testing.T) {
	// Test OrderedMap when used in a slice (like ExecutorResponse.Rows)
	om1 := NewOrderedMap()
	om1.Set("?column?", 1)
	om1.Set("?column?", "Alice")

	om2 := NewOrderedMap()
	om2.Set("?column?", 2)
	om2.Set("?column?", "Bob")

	rows := []any{om1, om2}
	result, err := json.Marshal(rows)
	if err != nil {
		t.Errorf("json.Marshal() error = %v, want nil", err)
	}

	expected := `[{"?column?":1,"?column?":"Alice"},{"?column?":2,"?column?":"Bob"}]`
	if string(result) != expected {
		t.Errorf("json.Marshal() = %q, want %q", string(result), expected)
	}
}
