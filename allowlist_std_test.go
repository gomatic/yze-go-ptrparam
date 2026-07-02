package ptrparam

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGeneratedStdAllowlistMembership pins representative entries so a
// regeneration that loses a rule (or a toolchain whose API moved) fails
// loudly rather than silently reflagging idiomatic pointers.
func TestGeneratedStdAllowlistMembership(t *testing.T) {
	for _, name := range []string{
		"strings.Builder",        // uncopyable (noCopy)
		"sync.WaitGroup",         // uncopyable
		"testing.T",              // uncopyable
		"os.File",                // pointer-only methods
		"os.Root",                // pointer-only methods
		"net/http.Request",       // pointer-only methods
		"log/slog.Logger",        // pointer-only methods
		"text/template.Template", // pointer-only declared methods (pointer embed)
		"database/sql.DB",        // pointer-only methods
		"flag.Flag",              // stdlib passes it (flag.Visit)
		"runtime.MemStats",       // stdlib passes it (runtime.ReadMemStats)
		"unicode.RangeTable",     // stdlib passes it (unicode.Is)
	} {
		assert.True(t, stdPointerParams[name], name)
	}
}

// TestGeneratedStdAllowlistExclusions pins value-idiomatic types that must
// never be allowlisted; their presence would gut the analyzer.
func TestGeneratedStdAllowlistExclusions(t *testing.T) {
	for _, name := range []string{
		"time.Time",
		"time.Duration",
		"net/netip.Addr",
		"reflect.Value",
		"context.Context",
	} {
		assert.False(t, stdPointerParams[name], name)
	}
	for name := range stdPointerParams {
		assert.False(t, strings.Contains(name, "/internal/"), name)
	}
}
