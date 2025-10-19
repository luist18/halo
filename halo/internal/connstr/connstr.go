package connstr

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ConnectionConfig holds parsed connection string parameters
type ConnectionConfig struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
	SSLMode  string
	Options  map[string]string
}

// Parse parses a PostgreSQL connection string in either URI or keyword-value format
// Supported formats:
//   - URI: postgresql://user:password@host:port/database?option=value
//   - URI: postgres://user:password@host:port/database?option=value
//   - Keyword-value: host=localhost port=5432 dbname=mydb user=myuser
func Parse(connStr string) (*ConnectionConfig, error) {
	// Trim whitespace first
	connStr = strings.TrimSpace(connStr)

	if connStr == "" {
		return nil, fmt.Errorf("connection string cannot be empty")
	}

	// Determine format and parse accordingly
	if strings.HasPrefix(connStr, "postgresql://") || strings.HasPrefix(connStr, "postgres://") {
		return parseURI(connStr)
	}

	return parseKeywordValue(connStr)
}

// parseURI parses a PostgreSQL URI connection string
func parseURI(connStr string) (*ConnectionConfig, error) {
	// Replace postgres:// with postgresql:// for consistency
	if strings.HasPrefix(connStr, "postgres://") {
		connStr = "postgresql://" + strings.TrimPrefix(connStr, "postgres://")
	}

	u, err := url.Parse(connStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URI format: %w", err)
	}

	if u.Scheme != "postgresql" {
		return nil, fmt.Errorf("invalid scheme: expected postgresql, got %s", u.Scheme)
	}

	config := &ConnectionConfig{
		Host:    u.Hostname(),
		Options: make(map[string]string),
	}

	// Parse port
	if portStr := u.Port(); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
		config.Port = port
	} else {
		config.Port = 5432 // Default PostgreSQL port
	}

	// Parse database
	if u.Path != "" {
		config.Database = strings.TrimPrefix(u.Path, "/")
	}

	// Parse username and password
	if u.User != nil {
		config.User = u.User.Username()
		if password, ok := u.User.Password(); ok {
			config.Password = password
		}
	}

	// Parse query parameters
	for key, values := range u.Query() {
		if len(values) > 0 {
			switch key {
			case "sslmode":
				config.SSLMode = values[0]
			default:
				config.Options[key] = values[0]
			}
		}
	}

	return config, nil
}

// parseKeywordValue parses a keyword-value format connection string
func parseKeywordValue(connStr string) (*ConnectionConfig, error) {
	config := &ConnectionConfig{
		Port:    5432, // Default PostgreSQL port
		Options: make(map[string]string),
	}

	// Split by whitespace, handling quoted values
	pairs, err := splitKeyValuePairs(connStr)
	if err != nil {
		return nil, err
	}

	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid key-value pair format in connection string")
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
			value = value[1 : len(value)-1]
			// Unescape single quotes
			value = strings.ReplaceAll(value, "''", "'")
			// Unescape backslashes
			value = strings.ReplaceAll(value, "\\\\", "\\")
		}

		switch key {
		case "host":
			config.Host = value
		case "port":
			// Skip empty port values
			if value == "" {
				continue
			}
			port, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid port: %w", err)
			}
			config.Port = port
		case "dbname", "database":
			config.Database = value
		case "user":
			config.User = value
		case "password":
			config.Password = value
		case "sslmode":
			config.SSLMode = value
		default:
			config.Options[key] = value
		}
	}

	return config, nil
}

// splitKeyValuePairs splits a connection string into key-value pairs,
// respecting quoted values
func splitKeyValuePairs(s string) ([]string, error) {
	var pairs []string
	var current strings.Builder
	inQuote := false
	escaped := false

	for i, ch := range s {
		switch {
		case escaped:
			current.WriteRune(ch)
			escaped = false
		case ch == '\\':
			current.WriteRune(ch)
			escaped = true
		case ch == '\'':
			current.WriteRune(ch)
			inQuote = !inQuote
		case ch == ' ' && !inQuote:
			if current.Len() > 0 {
				pairs = append(pairs, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}

		// Check for unterminated quote at end of string
		if i == len(s)-1 && inQuote {
			return nil, fmt.Errorf("unterminated quoted value")
		}
	}

	if current.Len() > 0 {
		pairs = append(pairs, current.String())
	}

	return pairs, nil
}

// ToURI converts the ConnectionConfig back to a URI format connection string
func (c *ConnectionConfig) ToURI() string {
	if c == nil {
		return ""
	}

	u := &url.URL{
		Scheme: "postgresql",
		Host:   c.Host,
	}

	if c.Port != 0 && c.Port != 5432 {
		u.Host = fmt.Sprintf("%s:%d", c.Host, c.Port)
	} else if c.Port == 5432 {
		u.Host = fmt.Sprintf("%s:%d", c.Host, c.Port)
	}

	if c.User != "" {
		if c.Password != "" {
			u.User = url.UserPassword(c.User, c.Password)
		} else {
			u.User = url.User(c.User)
		}
	}

	if c.Database != "" {
		u.Path = "/" + c.Database
	}

	// Add query parameters
	q := url.Values{}
	if c.SSLMode != "" {
		q.Set("sslmode", c.SSLMode)
	}
	for key, value := range c.Options {
		q.Set(key, value)
	}

	if len(q) > 0 {
		u.RawQuery = q.Encode()
	}

	return u.String()
}

// ToKeywordValue converts the ConnectionConfig to a keyword-value format connection string
func (c *ConnectionConfig) ToKeywordValue() string {
	if c == nil {
		return ""
	}

	var parts []string

	if c.Host != "" {
		parts = append(parts, fmt.Sprintf("host=%s", escapeKeywordValue(c.Host)))
	}
	if c.Port != 0 {
		parts = append(parts, fmt.Sprintf("port=%d", c.Port))
	}
	if c.Database != "" {
		parts = append(parts, fmt.Sprintf("dbname=%s", escapeKeywordValue(c.Database)))
	}
	if c.User != "" {
		parts = append(parts, fmt.Sprintf("user=%s", escapeKeywordValue(c.User)))
	}
	if c.Password != "" {
		parts = append(parts, fmt.Sprintf("password=%s", escapeKeywordValue(c.Password)))
	}
	if c.SSLMode != "" {
		parts = append(parts, fmt.Sprintf("sslmode=%s", escapeKeywordValue(c.SSLMode)))
	}

	for key, value := range c.Options {
		parts = append(parts, fmt.Sprintf("%s=%s", key, escapeKeywordValue(value)))
	}

	return strings.Join(parts, " ")
}

// escapeKeywordValue escapes a value for use in keyword-value format
func escapeKeywordValue(s string) string {
	// If the value contains spaces, backslashes, or single quotes, quote it
	needsQuoting := strings.ContainsAny(s, " \\'\t\n")

	if !needsQuoting {
		return s
	}

	// Escape single quotes and backslashes
	escaped := strings.ReplaceAll(s, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "'", "''")

	return fmt.Sprintf("'%s'", escaped)
}

// Validate checks if the connection configuration has the minimum required fields
func (c *ConnectionConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("connection config is nil")
	}

	if c.Host == "" {
		return fmt.Errorf("host is required")
	}

	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	return nil
}
