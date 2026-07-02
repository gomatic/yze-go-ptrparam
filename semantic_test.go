package ptrparam

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mkNamed builds a named struct type in a throwaway package.
func mkNamed(pkg *types.Package, name string, fields ...*types.Var) *types.Named {
	st := types.NewStruct(fields, nil)
	return types.NewNamed(types.NewTypeName(token.NoPos, pkg, name, nil), st, nil)
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

func TestUncopyableAndLocks(t *testing.T) {
	pkg := types.NewPackage("x", "x")

	lock := mkNamed(pkg, "Lockish")
	addMethod(pkg, lock, "Lock", true, true)
	assert.True(t, uncopyable(lock, map[types.Type]bool{}), "a pointer-receiver nullary Lock marks must-not-copy")

	falseLock := mkNamed(pkg, "FalseLock")
	addMethod(pkg, falseLock, "Lock", true, false)
	assert.False(t, uncopyable(falseLock, map[types.Type]bool{}), "Lock with parameters is not the copylocks marker")

	holder := mkNamed(pkg, "Holder", types.NewVar(token.NoPos, pkg, "mu", lock))
	assert.True(t, uncopyable(holder, map[types.Type]bool{}), "a lock field poisons the struct")

	viaPtr := mkNamed(pkg, "ViaPtr", types.NewVar(token.NoPos, pkg, "mu", types.NewPointer(lock)))
	assert.False(t, uncopyable(viaPtr, map[types.Type]bool{}), "a lock behind a pointer copies safely")

	arr := mkNamed(pkg, "Arr", types.NewVar(token.NoPos, pkg, "cells", types.NewArray(lock, 2)))
	assert.True(t, uncopyable(arr, map[types.Type]bool{}), "array elements are walked")

	plainVar := types.NewVar(token.NoPos, pkg, "a", types.Typ[types.Int])
	twice := mkNamed(
		pkg, "Twice",
		types.NewVar(token.NoPos, pkg, "a", plainVar.Type()),
		types.NewVar(token.NoPos, pkg, "b", plainVar.Type()),
	)
	assert.False(t, uncopyable(twice, map[types.Type]bool{}), "plain fields, incl. the seen-set revisit, stay copyable")
	assert.False(t, uncopyable(types.Typ[types.Int], map[types.Type]bool{}), "basics are copyable")
}

func TestIsNullary(t *testing.T) {
	pkg := types.NewPackage("x", "x")
	nullary := types.NewFunc(token.NoPos, pkg, "f",
		types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(), false))
	assert.True(t, isNullary(nullary))
	assert.False(
		t,
		isNullary(types.NewVar(token.NoPos, pkg, "v", types.Typ[types.Int])),
		"a non-func object is not nullary",
	)
}
