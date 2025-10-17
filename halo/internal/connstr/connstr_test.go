package connstr

import (
	"testing"
)

func TestParse_URI(t *testing.T) {
	tests := []struct {
		name    string
		connStr string
		want    *ConnectionConfig
		wantErr bool
	}{
		{
			name:    "basic URI with postgresql scheme",
			connStr: "postgresql://user:pass@localhost:5432/mydb",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "pass",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "basic URI with postgres scheme",
			connStr: "postgres://user:pass@localhost:5432/mydb",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "pass",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "URI without port uses default",
			connStr: "postgresql://user:pass@localhost/mydb",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "pass",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "URI without password",
			connStr: "postgresql://user@localhost:5432/mydb",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "URI without database",
			connStr: "postgresql://user:pass@localhost:5432",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "",
				User:     "user",
				Password: "pass",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "URI with sslmode query parameter",
			connStr: "postgresql://user:pass@localhost:5432/mydb?sslmode=require",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "pass",
				SSLMode:  "require",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "URI with multiple query parameters",
			connStr: "postgresql://user:pass@localhost:5432/mydb?sslmode=require&connect_timeout=10&application_name=myapp",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "pass",
				SSLMode:  "require",
				Options: map[string]string{
					"connect_timeout":  "10",
					"application_name": "myapp",
				},
			},
			wantErr: false,
		},
		{
			name:    "URI with encoded password",
			connStr: "postgresql://user:p%40ss%23word@localhost:5432/mydb",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "p@ss#word",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "URI with custom port",
			connStr: "postgresql://user:pass@localhost:5433/mydb",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5433,
				Database: "mydb",
				User:     "user",
				Password: "pass",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "URI with IPv4 address",
			connStr: "postgresql://user:pass@192.168.1.1:5432/mydb",
			want: &ConnectionConfig{
				Host:     "192.168.1.1",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "pass",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "URI with IPv6 address",
			connStr: "postgresql://user:pass@[::1]:5432/mydb",
			want: &ConnectionConfig{
				Host:     "::1",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "pass",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "invalid port in URI",
			connStr: "postgresql://user:pass@localhost:invalid/mydb",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid URI format",
			connStr: "postgresql://[invalid",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.connStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !compareConnectionConfigs(got, tt.want) {
				t.Errorf("Parse() got = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParse_KeywordValue(t *testing.T) {
	tests := []struct {
		name    string
		connStr string
		want    *ConnectionConfig
		wantErr bool
	}{
		{
			name:    "basic keyword-value format",
			connStr: "host=localhost port=5432 dbname=mydb user=myuser password=mypass",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "myuser",
				Password: "mypass",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "keyword-value with sslmode",
			connStr: "host=localhost port=5432 dbname=mydb user=myuser sslmode=require",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "myuser",
				SSLMode:  "require",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "keyword-value with database instead of dbname",
			connStr: "host=localhost port=5432 database=mydb user=myuser",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "myuser",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "keyword-value with quoted value containing spaces",
			connStr: "host=localhost dbname=mydb user='my user'",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "my user",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "keyword-value with escaped quotes",
			connStr: "host=localhost dbname=mydb password='my''password'",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				Password: "my'password",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "keyword-value with escaped backslash",
			connStr: "host=localhost dbname=mydb password='my\\\\password'",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				Password: "my\\password",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "keyword-value without port uses default",
			connStr: "host=localhost dbname=mydb user=myuser",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "myuser",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "keyword-value with custom options",
			connStr: "host=localhost dbname=mydb user=myuser connect_timeout=10 application_name=myapp",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "myuser",
				Options: map[string]string{
					"connect_timeout":  "10",
					"application_name": "myapp",
				},
			},
			wantErr: false,
		},
		{
			name:    "keyword-value with extra spaces",
			connStr: "  host=localhost   port=5432   dbname=mydb  ",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "invalid keyword-value format (missing value)",
			connStr: "host=localhost port= dbname=mydb",
			want: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				Options:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name:    "invalid keyword-value format (no equals)",
			connStr: "host=localhost invalid dbname=mydb",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid port value",
			connStr: "host=localhost port=invalid dbname=mydb",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "unterminated quote",
			connStr: "host=localhost user='unterminated",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.connStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !compareConnectionConfigs(got, tt.want) {
				t.Errorf("Parse() got = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParse_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		connStr string
		wantErr bool
	}{
		{
			name:    "empty string",
			connStr: "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			connStr: "   ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.connStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConnectionConfig_ToURI(t *testing.T) {
	tests := []struct {
		name   string
		config *ConnectionConfig
		want   string
	}{
		{
			name: "basic config",
			config: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "pass",
				Options:  map[string]string{},
			},
			want: "postgresql://user:pass@localhost:5432/mydb",
		},
		{
			name: "config with sslmode",
			config: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "pass",
				SSLMode:  "require",
				Options:  map[string]string{},
			},
			want: "postgresql://user:pass@localhost:5432/mydb?sslmode=require",
		},
		{
			name: "config without password",
			config: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Options:  map[string]string{},
			},
			want: "postgresql://user@localhost:5432/mydb",
		},
		{
			name: "config without database",
			config: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "pass",
				Options:  map[string]string{},
			},
			want: "postgresql://user:pass@localhost:5432",
		},
		{
			name:   "nil config",
			config: nil,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ToURI()
			if got != tt.want {
				t.Errorf("ToURI() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConnectionConfig_ToKeywordValue(t *testing.T) {
	tests := []struct {
		name   string
		config *ConnectionConfig
	}{
		{
			name: "basic config",
			config: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "pass",
				Options:  map[string]string{},
			},
		},
		{
			name: "config with sslmode",
			config: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				SSLMode:  "require",
				Options:  map[string]string{},
			},
		},
		{
			name: "config with spaces in password",
			config: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "my pass",
				Options:  map[string]string{},
			},
		},
		{
			name: "config with special characters",
			config: &ConnectionConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "mydb",
				User:     "user",
				Password: "my'pass\\word",
				Options:  map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to keyword-value format
			kvStr := tt.config.ToKeywordValue()

			// Parse it back
			parsed, err := Parse(kvStr)
			if err != nil {
				t.Errorf("Failed to parse converted keyword-value string: %v", err)
				return
			}

			// Compare the configs
			if !compareConnectionConfigs(parsed, tt.config) {
				t.Errorf("Roundtrip failed: got %+v, want %+v", parsed, tt.config)
			}
		})
	}
}

func TestConnectionConfig_ToKeywordValue_Nil(t *testing.T) {
	var config *ConnectionConfig
	got := config.ToKeywordValue()
	if got != "" {
		t.Errorf("ToKeywordValue() for nil config = %v, want empty string", got)
	}
}

func TestConnectionConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *ConnectionConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &ConnectionConfig{
				Host:    "localhost",
				Port:    5432,
				Options: map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "missing host",
			config: &ConnectionConfig{
				Port:    5432,
				Options: map[string]string{},
			},
			wantErr: true,
		},
		{
			name: "invalid port (zero)",
			config: &ConnectionConfig{
				Host:    "localhost",
				Port:    0,
				Options: map[string]string{},
			},
			wantErr: true,
		},
		{
			name: "invalid port (negative)",
			config: &ConnectionConfig{
				Host:    "localhost",
				Port:    -1,
				Options: map[string]string{},
			},
			wantErr: true,
		},
		{
			name: "invalid port (too large)",
			config: &ConnectionConfig{
				Host:    "localhost",
				Port:    70000,
				Options: map[string]string{},
			},
			wantErr: true,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRoundtrip_URIToKeywordValue(t *testing.T) {
	tests := []string{
		"postgresql://user:pass@localhost:5432/mydb",
		"postgresql://user@localhost:5432/mydb",
		"postgresql://user:pass@localhost/mydb",
		"postgresql://user:pass@localhost:5432/mydb?sslmode=require",
		"postgresql://user:pass@192.168.1.1:5432/mydb",
	}

	for _, connStr := range tests {
		t.Run(connStr, func(t *testing.T) {
			// Parse the URI
			config, err := Parse(connStr)
			if err != nil {
				t.Fatalf("Failed to parse URI: %v", err)
			}

			// Convert to keyword-value
			kvStr := config.ToKeywordValue()

			// Parse the keyword-value string
			parsed, err := Parse(kvStr)
			if err != nil {
				t.Fatalf("Failed to parse keyword-value string: %v", err)
			}

			// Compare configs
			if !compareConnectionConfigs(config, parsed) {
				t.Errorf("Roundtrip failed: original %+v, parsed %+v", config, parsed)
			}
		})
	}
}

func TestRoundtrip_KeywordValueToURI(t *testing.T) {
	tests := []string{
		"host=localhost port=5432 dbname=mydb user=user password=pass",
		"host=localhost port=5432 dbname=mydb user=user",
		"host=localhost dbname=mydb user=user password=pass",
		"host=localhost port=5432 dbname=mydb user=user sslmode=require",
	}

	for _, connStr := range tests {
		t.Run(connStr, func(t *testing.T) {
			// Parse the keyword-value string
			config, err := Parse(connStr)
			if err != nil {
				t.Fatalf("Failed to parse keyword-value: %v", err)
			}

			// Convert to URI
			uriStr := config.ToURI()

			// Parse the URI
			parsed, err := Parse(uriStr)
			if err != nil {
				t.Fatalf("Failed to parse URI string: %v", err)
			}

			// Compare configs
			if !compareConnectionConfigs(config, parsed) {
				t.Errorf("Roundtrip failed: original %+v, parsed %+v", config, parsed)
			}
		})
	}
}

// Helper function to compare ConnectionConfig structs
func compareConnectionConfigs(a, b *ConnectionConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	if a.Host != b.Host || a.Port != b.Port || a.Database != b.Database ||
		a.User != b.User || a.Password != b.Password || a.SSLMode != b.SSLMode {
		return false
	}

	if len(a.Options) != len(b.Options) {
		return false
	}

	for k, v := range a.Options {
		if b.Options[k] != v {
			return false
		}
	}

	return true
}
