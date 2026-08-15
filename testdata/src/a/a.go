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

// PlainPtr aliases a pointer. An alias is IDENTICAL to *Plain in Go — same
// method set, same assignability, same nil — so the parameter is a pointer
// parameter however it is spelled, and the rule decides on the type.
type PlainPtr = *Plain

func takesAliasedPointer(p PlainPtr) { _ = p } // want `pointer parameter`

// namedPtr is a defined type whose underlying is a pointer. The spelling is
// this module's own, so the pointed-to type decides exactly as *Plain does.
type namedPtr *Plain

func takesNamedPointer(p namedPtr) { _ = p } // want `pointer parameter`

// PtrTo is a generic alias; an instantiation names a known pointer type.
type PtrTo[T any] = *T

func takesGenericAliasPointer(p PtrTo[Plain]) { _ = p } // want `pointer parameter`

// LoggerPtr aliases a pointer to a pointer-idiomatic standard-library type:
// the exemptions apply to the aliased spelling exactly as to the star.
type LoggerPtr = *slog.Logger

func takesAliasedLogger(l LoggerPtr) { _ = l }

// variadicAlias is flagged: the alias is unwrapped through the ellipsis too.
func variadicAlias(ps ...PlainPtr) { _ = ps } // want `pointer parameter`

// valueLock declares a nullary Lock on the VALUE receiver, so *valueLock's
// method set holds it. That is the one-method marker, and it is not the one
// go vet's copylocks uses: vet is silent about copying this type, so the
// pointer is not required and the parameter is flagged.
type valueLock struct{ n int }

func (valueLock) Lock() {}

func takesValueLock(v *valueLock) { _ = v } // want `pointer parameter`

// forged holds the one-method marker as a blank field. If the marker counted,
// two lines written once would silence every struct holding it at any depth.
type forged struct {
	_ valueLock
	n int
}

func takesForged(f *forged) { _ = f } // want `pointer parameter`

// nestForged holds forged one level further up.
type nestForged struct{ f forged }

func takesNestForged(n *nestForged) { _ = n } // want `pointer parameter`

// valueLocker's VALUE is a sync.Locker in its own right, so a copy is as
// usable as the original — the clause vet's copylocks states as "a pointer to
// this type is a sync.Locker, but a value is not". Flagged.
type valueLocker struct{ n int }

func (valueLocker) Lock() {}

func (valueLocker) Unlock() {}

func takesValueLocker(v *valueLocker) { _ = v } // want `pointer parameter`

// counterLock is a sync.Locker by pointer but is NOT a struct, and copying it
// copies everything it is. vet's copylocks looks only at structs; so does
// this. Value() keeps the pointer-only-methods exemption out of the decision.
type counterLock int

func (c *counterLock) Lock() {}

func (c *counterLock) Unlock() {}

func (c counterLock) Value() int { return int(c) }

func takesCounterLock(c *counterLock) { _ = c } // want `pointer parameter`

// door is must-not-copy under vet's own marker: *door is a sync.Locker and
// door is not. Allowed.
type door struct{ n int }

func (d *door) Lock() {}

func (d *door) Unlock() {}

// holdsDoor has no methods of its own, so only the copylocks criterion can
// exempt it: the walk reaches the field exactly as vet's does.
type holdsDoor struct {
	d door
	n int
}

func takesHoldsDoor(h *holdsDoor) { _ = h }

// mutexLike carries the marker in its own method set and has a value-receiver
// method besides, so pointer-only-methods cannot answer for it and the
// copylocks criterion must. It has to: go vet reports every copy of this type,
// so reporting the pointer parameter would prescribe a remedy the standard
// toolchain rejects.
type mutexLike struct{ n int }

func (m *mutexLike) Lock() {}

func (m *mutexLike) Unlock() {}

func (m mutexLike) N() int { return m.n }

func takesMutexLike(m *mutexLike) { _ = m }

// derivedBuilder is DEFINED over a type whose pointer is its convention. It
// inherits none of strings.Builder's methods and all of its layout, so it
// carries the same copy hazard while announcing none of it. Allowed, because
// the prescribed remedy on this type does not merely cost — it panics.
type derivedBuilder strings.Builder

func takesDerivedBuilder(b *derivedBuilder) { _ = b }

// derivedTwice is one more definition further along the same chain.
type derivedTwice derivedBuilder

func takesDerivedTwice(b *derivedTwice) { _ = b }

// Config is named for the shape a name-keyed exemption takes: yze/globalvar
// ships one, and an extra disjunct inside an existing condition adds no
// statement, so statement coverage cannot see it. The rule keys on the type
// and never on its spelling, so a type called Config is reported like any
// other and this case is what says so.
type Config struct{ n int }

func takesConfig(c *Config) { _ = c } // want `pointer parameter`

// derivedPlain is defined over a copyable type, so it stays flagged: the walk
// follows the definition, it does not exempt every definition.
type derivedPlain Plain

func takesDerivedPlain(p *derivedPlain) { _ = p } // want `pointer parameter`

// genDerived is the generic form of derivedBuilder: the same definition over
// the same type, so the same layout and the same copy hazard. Keying the
// definition map by the generic ORIGIN and looking it up by the INSTANCE
// missed every instantiation, and the remedy prescribed for one panics.
type genDerived[T any] strings.Builder

func takesGenericDerived(b *genDerived[int]) { _ = b }

// lockConstrained holds a field whose type parameter is constrained to a
// lock. go vet's copylocks resolves the constraint and reports every copy of
// this type; so must this, or the two prescribe opposite things about one
// line. Allowed.
type lockable interface{ sync.Mutex }

type lockConstrained[T lockable] struct{ v T }

func takesLockConstrained[T lockable](c *lockConstrained[T]) { _ = c }

// eitherLock's constraint is a UNION of two lock types, so the walk has to
// enter the terms rather than stop at the element. Allowed.
type eitherLock interface {
	sync.Mutex | sync.RWMutex
}

type unionConstrained[T eitherLock] struct{ v T }

func takesUnionConstrained[T eitherLock](c *unionConstrained[T]) { _ = c }

// countConstrained's constraint is a non-interface type, the shorthand form,
// and int copies freely: flagged.
type countConstrained[T int] struct{ v T }

func takesCountConstrained[T int](c *countConstrained[T]) { _ = c } // want `pointer parameter`

// anyConstrained's constraint admits everything and requires nothing: flagged.
type anyConstrained[T any] struct{ v T }

func takesAnyConstrained[T any](c *anyConstrained[T]) { _ = c } // want `pointer parameter`

// mixedConstrained's constraint is a union of two copyable terms, so the walk
// enters the terms and comes back out again: flagged.
type copyableEither interface {
	int | string
}

type mixedConstrained[T copyableEither] struct{ v T }

func takesMixedConstrained[T copyableEither](c *mixedConstrained[T]) { _ = c } // want `pointer parameter`
