package httpexecutor

import (
	"context"
	"errors"
	"testing"

	"github.com/luist18/halo/internal/data"
)

func TestExecute_PoolOptInNotImplemented(t *testing.T) {
	tests := []struct {
		name      string
		opts      Options
		wantErr   error
		shouldErr bool
	}{
		{
			name: "pool opt-in enabled returns error",
			opts: Options{
				PoolOptIn:           true,
				BatchIsolationLevel: "ReadCommitted",
				BatchReadOnly:       false,
				BatchDeferrable:     false,
			},
			wantErr:   ErrPoolOptInNotImplemented,
			shouldErr: true,
		},
		{
			name: "pool opt-in disabled should not return error for pool feature",
			opts: Options{
				PoolOptIn:           false,
				BatchIsolationLevel: "ReadCommitted",
				BatchReadOnly:       false,
				BatchDeferrable:     false,
			},
			wantErr:   nil,
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			connStr := data.NewSecret("postgres://user:password@localhost:5432/dbname")
			payload := Payload{
				Query:  "SELECT 1",
				Params: []interface{}{},
			}

			_, err := Execute(ctx, *connStr, payload, tt.opts)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("Execute() expected error but got nil")
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Execute() error = %v, want %v", err, tt.wantErr)
				}
			} else if tt.opts.PoolOptIn {
				// If PoolOptIn is false, we expect other errors (like connection errors)
				// but NOT the ErrPoolOptInNotImplemented
				if errors.Is(err, ErrPoolOptInNotImplemented) {
					t.Errorf("Execute() should not return ErrPoolOptInNotImplemented when PoolOptIn is false, got: %v", err)
				}
			}
		})
	}
}
