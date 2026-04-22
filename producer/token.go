package producer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Token runs a shell script and produces a single cached value.
// The script's first line is the TTL in seconds, the second line is the value.
// The token is cached until the TTL expires, then the script is re-executed.
type Token struct {
	cmd string

	// Cached state
	m           sync.Mutex
	cachedValue string
	cachedAt    time.Time
	ttl         time.Duration
}

// NewToken creates a new Token producer that runs the given shell command.
func NewToken(cmd string) *Token {
	return &Token{
		cmd: cmd,
	}
}

// Yield executes the script (if not cached or TTL expired), parses the
// output. Yield can be called concurrently.
func (t *Token) Yield(ctx context.Context) (string, error) {
	t.m.Lock()
	defer t.m.Unlock()

	// Check if cache is still valid
	if t.cachedValue != "" && time.Since(t.cachedAt) < t.ttl {
		return t.cachedValue, nil
	}

	// Execute the script and parse output
	ttl, value, err := executeTokenScript(ctx, t.cmd)
	if err != nil {
		return "", err
	}

	t.ttl = ttl
	t.cachedValue = value
	t.cachedAt = time.Now()

	return value, nil
}

// executeTokenScript runs the script and parses the first two lines.
func executeTokenScript(ctx context.Context, command string) (ttl time.Duration, value string, err error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return 0, "", fmt.Errorf("script %q exited with code %d: %s", command, exitErr.ExitCode(), string(exitErr.Stderr))
		}

		return 0, "", fmt.Errorf("script %q failed: %w", command, err)
	}

	return parseTokenOutput(string(output))
}

// parseTokenOutput parses the script output. The first line must be an integer
// representing the TTL in seconds. The second line is the value.
func parseTokenOutput(output string) (ttl time.Duration, value string, err error) {
	scanner := bufio.NewScanner(strings.NewReader(output))

	// Read TTL line
	if !scanner.Scan() {
		return 0, "", fmt.Errorf("script output is empty, expected TTL on first line")
	}

	ttlLine := strings.TrimSpace(scanner.Text())

	ttlSeconds, err := strconv.Atoi(ttlLine)
	if err != nil {
		return 0, "", fmt.Errorf("invalid TTL on first line %q: must be an integer", ttlLine)
	}

	if ttlSeconds <= 0 {
		return 0, "", fmt.Errorf("TTL must be positive, got %d", ttlSeconds)
	}

	ttl = time.Duration(ttlSeconds) * time.Second

	// Read value line
	if !scanner.Scan() {
		return 0, "", fmt.Errorf("script output missing value on second line")
	}

	value = strings.TrimSpace(scanner.Text())

	if value == "" {
		return 0, "", fmt.Errorf("script output value on second line is empty")
	}

	// Ignore any additional lines
	err = scanner.Err()
	if err != nil {
		return 0, "", fmt.Errorf("error reading script output: %w", err)
	}

	return ttl, value, nil
}

const TokenCheckTimeout = 30 * time.Second

// allow testing an exec command early before setting up the producer.
func CheckToken(ctx context.Context, cmd string) error {
	ctx, cancel := context.WithTimeout(ctx, TokenCheckTimeout)
	defer cancel()

	_, _, err := executeTokenScript(ctx, cmd)
	if err != nil {
		return fmt.Errorf("run token script: %w", err)
	}

	return nil
}
