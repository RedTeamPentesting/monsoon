package producer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func makeScript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "script.sh")

	err := os.WriteFile(path, []byte(content), 0755)
	if err != nil {
		t.Fatal(err)
	}

	return path
}

func TestTokenYield(t *testing.T) {
	t.Parallel()

	script := makeScript(t, "#!/bin/sh\necho 300\necho token-value-123\n")
	p := NewToken(script)

	ctx := context.Background()

	value, err := p.Yield(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if value != "token-value-123" {
		t.Errorf("expected 'token-value-123', got %q", value)
	}
}

func TestTokenYieldCachesResult(t *testing.T) {
	t.Parallel()

	scriptContent := `#!/bin/sh
echo 2
echo token-value-123
`
	script := makeScript(t, scriptContent)
	p := NewToken(script)

	ctx := context.Background()

	// First yield
	value1, err := p.Yield(ctx)
	if err != nil {
		t.Fatalf("first yield: expected no error, got %v", err)
	}

	if value1 != "token-value-123" {
		t.Errorf("first yield: expected 'token-value-123', got %q", value1)
	}

	// Second yield should use cached value
	value2, err := p.Yield(ctx)
	if err != nil {
		t.Fatalf("second yield: expected no error, got %v", err)
	}

	if value2 != "token-value-123" {
		t.Errorf("second yield: expected cached 'token-value-123', got %q", value2)
	}
}

func TestTokenYieldRefreshesAfterTTL(t *testing.T) {
	t.Parallel()

	scriptContent := `#!/bin/sh
echo 1
echo token-value-456
`
	script := makeScript(t, scriptContent)
	p := NewToken(script)

	ctx := context.Background()

	// First yield
	value1, err := p.Yield(ctx)
	if err != nil {
		t.Fatalf("first yield: expected no error, got %v", err)
	}

	if value1 != "token-value-456" {
		t.Errorf("first yield: expected 'token-value-456', got %q", value1)
	}

	// Wait for TTL to expire
	time.Sleep(1100 * time.Millisecond)

	// Second yield should refresh
	value2, err := p.Yield(ctx)
	if err != nil {
		t.Fatalf("second yield: expected no error, got %v", err)
	}

	if value2 != "token-value-456" {
		t.Errorf("second yield: expected refreshed 'token-value-456', got %q", value2)
	}
}

func TestTokenInvalidTTL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		script  string
		wantErr bool
	}{
		{
			name:    "empty output",
			script:  "#!/bin/sh\n",
			wantErr: true,
		},
		{
			name:    "non-integer TTL",
			script:  "#!/bin/sh\necho abc\necho token\n",
			wantErr: true,
		},
		{
			name:    "zero TTL",
			script:  "#!/bin/sh\necho 0\necho token\n",
			wantErr: true,
		},
		{
			name:    "negative TTL",
			script:  "#!/bin/sh\necho -5\necho token\n",
			wantErr: true,
		},
		{
			name:    "missing value line",
			script:  "#!/bin/sh\necho 300\n",
			wantErr: true,
		},
		{
			name:    "empty value line",
			script:  "#!/bin/sh\necho 300\necho\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			script := makeScript(t, tt.script)
			p := NewToken(script)

			ctx := context.Background()

			_, err := p.Yield(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("expected error=%v, got err=%v", tt.wantErr, err)
			}
		})
	}
}

func TestTokenExtraOutputLinesIgnored(t *testing.T) {
	t.Parallel()

	script := makeScript(t, "#!/bin/sh\necho 60\necho token-value\necho extra-line-1\necho extra-line-2\n")
	p := NewToken(script)

	ctx := context.Background()

	value, err := p.Yield(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if value != "token-value" {
		t.Errorf("expected 'token-value', got %q", value)
	}
}

func TestCheckToken(t *testing.T) {
	t.Parallel()

	// Test with valid command
	err := CheckToken(context.Background(), "echo -e '123\nhello'")
	if err != nil {
		t.Errorf("expected no error for valid command, got %v", err)
	}

	// Test with non-existent command
	err = CheckToken(context.Background(), "/nonexistent/command arg1 arg2")
	if err == nil {
		t.Errorf("expected error for non-existent command")
	}
}
