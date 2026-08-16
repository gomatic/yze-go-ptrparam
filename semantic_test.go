package ptrparam

import (
	"go/importer"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mkNamed builds a named struct type in a throwaway package.
func mkNamed(pkg *types.Package, name string, fields ...*types.Var) *types.Named {
	st := types.NewStruct(fields, nil)
	return types.NewNamed(types.NewTypeName(token.NoPos, pkg, name, nil), st, nil)
}

// mkNamedOver builds a named type over an arbitrary underlying type.
func mkNamedOver(pkg *types.Package, name string, under types.Type) *types.Named {
	return types.NewNamed(types.NewTypeName(token.NoPos, pkg, name, nil), under, nil)
}

// addMethod attaches a method with the given receiver pointer-ness and shape.
func addMethod(pkg *types.Package, n *types.Named, name string, onPointer, nullary bool) {
	var recvType types.Type = n
	if onPointer {
		recvType = types.NewPointer(n)
	}
	recv := types.NewVar(token.NoPos, pkg, "r", recvType)
	params := types.NewTuple()
	if !nullary {
		params = types.NewTuple(types.NewVar(token.NoPos, pkg, "n", types.Typ[types.Int]))
	}
	sig := types.NewSignatureType(recv, nil, nil, params, types.NewTuple(), false)
	n.AddMethod(types.NewFunc(token.NoPos, pkg, name, sig))
}

// addLocker gives n the sync.Locker method pair on the chosen receiver.
func addLocker(pkg *types.Package, n *types.Named, onPointer bool) {
	addMethod(pkg, n, "Lock", onPointer, true)
	addMethod(pkg, n, "Unlock", onPointer, true)
}

// TestLockerTypeIsSyncLocker pins the marker itself against the standard
// library rather than against this file's idea of it. mustNotCopy's whole
// claim is that this package and go vet's copylocks agree by construction, and
// that claim is only worth anything if the interface built here IS sync.Locker
// — a dropped method or a misspelt name would silently widen the exemption
// back to the one-method marker this replaced.
func TestLockerTypeIsSyncLocker(t *testing.T) {
	t.Parallel()
	sync, err := importer.Default().Import("sync")
	require.NoError(t, err)
	locker, ok := sync.Scope().Lookup("Locker").(*types.TypeName)
	require.True(t, ok)
	assert.True(t, types.Identical(lockerType, locker.Type().Underlying()),
		"the marker must be sync.Locker exactly, not something shaped like it")
}

func TestPointerOnlyMethods(t *testing.T) {
	pkg := types.NewPackage("x", "x")

	ptrOnly := mkNamed(pkg, "PtrOnly")
	addMethod(pkg, ptrOnly, "Run", true, true)
	assert.True(t, pointerOnlyMethods(ptrOnly))

	mixed := mkNamed(pkg, "Mixed")
	addMethod(pkg, mixed, "Get", false, true)
	addMethod(pkg, mixed, "Set", true, false)
	assert.False(t, pointerOnlyMethods(mixed), "a value receiver makes values usable")

	bare := mkNamed(pkg, "Bare")
	assert.False(t, pointerOnlyMethods(bare), "no exported methods, no convention")

	shy := mkNamed(pkg, "Shy")
	addMethod(pkg, shy, "hidden", false, true)
	addMethod(pkg, shy, "Bump", true, true)
	assert.True(t, pointerOnlyMethods(shy), "unexported methods are skipped")
}

// TestUncopyableAppliesVetsCopylocksCriterion holds uncopyable to the standard
// semantic.go cites, in both directions. The criterion is sync.Locker on the
// POINTER and not on the value, over a struct — not "some method called Lock",
// which is strictly wider than go vet and is two lines to forge.
func TestUncopyableAppliesVetsCopylocksCriterion(t *testing.T) {
	pkg := types.NewPackage("x", "x")

	locker := mkNamed(pkg, "Locky")
	addLocker(pkg, locker, true)
	assert.True(t, uncopyable(locker), "*T is a sync.Locker and T is not: vet's must-not-copy marker")

	oneMethod := mkNamed(pkg, "OneMethod")
	addMethod(pkg, oneMethod, "Lock", true, true)
	assert.False(t, uncopyable(oneMethod), "Lock alone is not sync.Locker, and go vet is silent about copying it")

	onValue := mkNamed(pkg, "OnValue")
	addLocker(pkg, onValue, false)
	assert.False(t, uncopyable(onValue), "a value that is itself a Locker copies as usable as the original")

	withArgs := mkNamed(pkg, "WithArgs")
	addMethod(pkg, withArgs, "Lock", true, false)
	addMethod(pkg, withArgs, "Unlock", true, true)
	assert.False(t, uncopyable(withArgs), "Lock with parameters is not the copylocks marker")

	counter := mkNamedOver(pkg, "Counter", types.Typ[types.Int])
	addLocker(pkg, counter, true)
	assert.False(t, uncopyable(counter), "vet's copylocks looks only at structs; copying an int copies all of it")

	holder := mkNamed(pkg, "Holder", types.NewVar(token.NoPos, pkg, "mu", locker))
	assert.True(t, uncopyable(holder), "a lock field poisons the struct")

	viaPtr := mkNamed(pkg, "ViaPtr", types.NewVar(token.NoPos, pkg, "mu", types.NewPointer(locker)))
	assert.False(t, uncopyable(viaPtr), "a lock behind a pointer copies safely")

	arr := mkNamed(pkg, "Arr", types.NewVar(token.NoPos, pkg, "cells", types.NewArray(locker, 2)))
	assert.True(t, uncopyable(arr), "array elements are walked")

	plain := types.NewVar(token.NoPos, pkg, "a", types.Typ[types.Int])
	twice := mkNamed(
		pkg, "Twice",
		types.NewVar(token.NoPos, pkg, "a", plain.Type()),
		types.NewVar(token.NoPos, pkg, "b", plain.Type()),
	)
	assert.False(t, uncopyable(twice), "plain fields stay copyable however many of them there are")
	assert.False(t, uncopyable(types.Typ[types.Int]), "basics are copyable")
}

// TestConstraintUncopyableReadsEveryConstraintFormGoTypesProduces names the
// assertion constraintUncopyable rests on: a constraint's underlying is an
// interface in every form the language admits, because go/types wraps a
// non-interface one implicitly. The shorthand `[T int]` is the case that
// looks like a counter-example and is not, and it is asserted here rather
// than assumed, because the alternative to asserting it is a branch nothing
// can reach and a coverage gate that says so.
//
// The rest is the reason the walk exists at all: go vet's copylocks resolves
// a type parameter through its constraint, and a diagnostic that skipped that
// step prescribed a value parameter which vet then rejected.
func TestConstraintUncopyableReadsEveryConstraintFormGoTypesProduces(t *testing.T) {
	t.Parallel()
	pkg := checkedPkg(t, `package p

import "sync"

type Locked interface{ sync.Mutex }

type Either interface {
	sync.Mutex | sync.RWMutex
}

type Copyable interface {
	int | string
}

func Locks[T Locked](v T) {}

func Union[T Either](v T) {}

func Copies[T Copyable](v T) {}

func Counts[T int](v T) {}

func Anything[T any](v T) {}
`)

	counts := typeParamOf(t, pkg, "Counts")
	_, wrapped := counts.Constraint().Underlying().(*types.Interface)
	assert.True(t, wrapped, "go/types wraps a non-interface constraint in an implicit interface")

	assert.True(t, uncopyable(typeParamOf(t, pkg, "Locks")), "a lock-constrained parameter must not be copied")
	assert.True(t, uncopyable(typeParamOf(t, pkg, "Union")), "the walk enters a union's terms")
	assert.False(t, uncopyable(typeParamOf(t, pkg, "Copies")), "and comes back out when no term is a lock")
	assert.False(t, uncopyable(counts), "the shorthand form is read like any other")
	assert.False(t, uncopyable(typeParamOf(t, pkg, "Anything")), "an empty constraint requires nothing")
}

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
