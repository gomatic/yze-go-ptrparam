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

// typeParamOf returns the sole type parameter of the package-scope function.
func typeParamOf(t *testing.T, pkg *types.Package, name string) *types.TypeParam {
	t.Helper()
	fn, isFunc := pkg.Scope().Lookup(name).(*types.Func)
	require.True(t, isFunc, "no func %s in the fixture", name)
	return fn.Type().(*types.Signature).TypeParams().At(0)
}
