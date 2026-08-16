package ptrparam

import (
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUncopyableTerminatesOnAConstraintThatNamesItsOwnParameter is the guard
// case: it enters the walk on shapes whose constraint element mentions the
// parameter it constrains, which the language accepts and go vet walks with a
// `seen` map for exactly this reason (copylock.go:282-290, "short-circuit
// infinite recursion due to type cycles"). Every subject here is legal Go that
// `go build` and `go vet` both accept, and without the guard the process does
// not fail a test — it dies with `fatal error: stack overflow`, taking the
// whole run with it.
//
// The verdicts are the point as much as the termination. None of these type
// sets admits a lock, so every one is copyable; a guard that answered "give up,
// call it uncopyable" would terminate and silence the rule. The enumeration is
// bounded and stated: eight spellings, chosen to vary the three things the
// descent keys on — where the cycle closes (struct field, array element, union
// term, generic instantiation), how the constraint is written (inline, named,
// generic alias), and whether one parameter closes the cycle alone or a pair
// closes it between them. It is not a proof that no ninth spelling exists.
func TestUncopyableTerminatesOnAConstraintThatNamesItsOwnParameter(t *testing.T) {
	t.Parallel()
	pkg := checkedPkg(t, `package p

import "sync"

type G[T any] struct{ V T }

type Elem[T any] interface{ ~struct{ X T } }

type AliasElem[T any] = interface{ ~struct{ X T } }

type Inner[T any] interface{ ~struct{ X T } }

type Outer[T any] interface{ Inner[T] }

func Inline[T interface{ ~struct{ N T } }](g *G[T]) {}

func Named[T Elem[T]](g *G[T]) {}

func Mutual[U Elem[V], V Elem[U]](g *G[U]) {}

func ArrayTerm[T interface{ ~struct{ X [1]T } }](g *G[T]) {}

func BareArray[T interface{ ~[1]T }](g *G[T]) {}

func Alias[T AliasElem[T]](g *G[T]) {}

func UnionSecond[T interface{ ~struct{ A int } | ~struct{ N T } }](g *G[T]) {}

func Instantiation[T interface{ ~struct{ X G[T] } }](g *G[T]) {}

func Embedded[T Outer[T]](g *G[T]) {}

func Locked[T interface{ ~struct{ N T; M sync.Mutex } }](g *G[T]) {}

type Holds interface{ sync.Mutex }

type HoldsNested interface{ Holds }

func EmbeddedLock[T HoldsNested](g *G[T]) {}
`)

	for _, name := range []string{
		"Inline", "Named", "Mutual", "ArrayTerm",
		"BareArray", "Alias", "UnionSecond", "Instantiation", "Embedded",
	} {
		assert.False(t, uncopyable(typeParamOf(t, pkg, name)),
			"%s admits no lock, so the walk must come back out of the cycle rather than into it", name)
	}

	assert.True(t, uncopyable(typeParamOf(t, pkg, "Locked")),
		"the cycle guard stops the descent, not the finding: a lock beside the cyclic field is still reached")
	assert.True(t, uncopyable(typeParamOf(t, pkg, "EmbeddedLock")),
		"and a lock one embedding away is the same type set, which go vet resolves and reports every copy of")
}

// TestComponentUncopyableEntersAnInterfaceTypeSetWithoutCycling names the
// claim the interface arm rests on: descending into an interface's type set is
// safe here only because the walk carries a cycle guard. A struct or an array
// cannot close a cycle — Go rejects it — but an interface embedding chain can,
// and the constraint that mentions its own parameter closes one through this
// arm. So the arm and the guard are one decision, and removing either without
// the other either loses the lock or loses the process.
func TestComponentUncopyableEntersAnInterfaceTypeSetWithoutCycling(t *testing.T) {
	t.Parallel()
	pkg := checkedPkg(t, `package p

import "sync"

type Guarded interface{ sync.Mutex }

type Wrapped interface{ Guarded }

type Twice interface{ Wrapped }

type Free interface{ int | string }

type FreeWrapped interface{ Free }
`)

	assert.True(t, componentUncopyable(visited{}, namedOf(t, pkg, "Wrapped")),
		"a lock one embedding away is the same type set")
	assert.True(t, componentUncopyable(visited{}, namedOf(t, pkg, "Twice")),
		"and the arm does not stop after one embedding")
	assert.False(t, componentUncopyable(visited{}, namedOf(t, pkg, "FreeWrapped")),
		"an embedded type set with no lock in it comes back out")
}

// typeParamOf returns the sole type parameter of the package-scope function.
func typeParamOf(t *testing.T, pkg *types.Package, name string) *types.TypeParam {
	t.Helper()
	fn, isFunc := pkg.Scope().Lookup(name).(*types.Func)
	require.True(t, isFunc, "no func %s in the fixture", name)
	return fn.Type().(*types.Signature).TypeParams().At(0)
}
