package a

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"sync"

	cli "github.com/urfave/cli/v3"
)

type Plain struct{ x int }

// BufAlias is an alias of an allow-listed standard-library type.
type BufAlias = bytes.Buffer

// takesLocal is flagged: pointer to a local type.
func takesLocal(p *Plain) { _ = p } // want `pointer parameter`

// takesInt is flagged: pointer to a basic type.
func takesInt(n *int) { _ = n } // want `pointer parameter`

// takesErr is flagged: pointer to error (named, no package).
func takesErr(e *error) { _ = e } // want `pointer parameter`

// takesLogger is allowed: a standard-library type where a pointer is idiomatic.
func takesLogger(l *slog.Logger) { _ = l }

// takesValue is fine: a value parameter.
func takesValue(p Plain) { _ = p }

// aliasOK is allowed: a pointer to an alias of an allow-listed stdlib type.
func aliasOK(b *BufAlias) { _ = b }

// variadicPtr is flagged: a variadic pointer parameter.
func variadicPtr(xs ...*int) { _ = xs } // want `pointer parameter`

// takesBuilder is allowed: strings.Builder is only usable by pointer.
func takesBuilder(b *strings.Builder) { _ = b }

// takesRoot is allowed: os.Root wraps a file descriptor, only usable by pointer.
func takesRoot(r *os.Root) { _ = r }

// takesCommand is allowed: the sanctioned CLI framework imposes *cli.Command
// in every callback signature.
func takesCommand(c *cli.Command) { _ = c }

// generic is allowed: a pointer to a type parameter is a generic seam the
// analyzer cannot judge.
func generic[T any](cfg *T) { _ = cfg }

// Engine declares only pointer-receiver methods: a value is unusable, so the
// pointer is its idiomatic convention — allowed semantically, no allowlist.
type Engine struct{ n int }

func (e *Engine) Run() { e.n++ }

func takesEngine(e *Engine) { _ = e }

// Mixed has a value-receiver method, so values are usable and the pointer
// parameter is flagged.
type Mixed struct{ n int }

func (m Mixed) Get() int { return m.n }

func (m *Mixed) Set(n int) { m.n = n }

func takesMixed(m *Mixed) { _ = m } // want `pointer parameter`

// guarded transitively holds a lock (vet copylocks criterion): uncopyable,
// so the pointer is required — allowed semantically.
type guarded struct {
	mu sync.Mutex
	n  int
}

func takesGuarded(g *guarded) { _ = g }

// unexportedMix skips unexported methods: its exported methods are all
// pointer-receiver, so it is pointer-only and allowed.
type unexportedMix struct{ n int }

func (u *unexportedMix) Bump() { u.n++ }

func (u unexportedMix) peek() int { return u.n }

func takesUnexportedMix(u *unexportedMix) { _ = u }

// twice holds two fields of one lock-free type, exercising the copy-walk's
// seen-set; still copyable and methodless, so flagged.
type point struct{ x, y int }

type twice struct{ a, b point }

func takesTwice(tw *twice) { _ = tw } // want `pointer parameter`

// arrayGuard holds locks inside an array element: uncopyable, allowed.
type arrayGuard struct{ cells [2]guarded }

func takesArrayGuard(a *arrayGuard) { _ = a }

// ptrField holds a lock behind a pointer, which copies safely: flagged.
type ptrField struct{ mu *sync.Mutex }

func takesPtrField(p *ptrField) { _ = p } // want `pointer parameter`
