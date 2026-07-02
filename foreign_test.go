package ptrparam

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/analysis"
)

// TestLocalToModule covers the module-identity branches the analysistest
// corpus cannot reach: its driver loads no module metadata, so pass.Module is
// always nil there.
func TestLocalToModule(t *testing.T) {
	self := types.NewPackage("example.test/mod/pkg", "pkg")
	other := types.NewPackage("example.test/other", "other")
	sub := types.NewPackage("example.test/mod/pkg/sub", "sub")

	assert.True(t, localToModule(&analysis.Pass{Pkg: self}, self), "the analyzed package is always local")
	assert.False(t, localToModule(&analysis.Pass{Pkg: self}, other), "nil module: only the analyzed package is local")
	assert.False(
		t,
		localToModule(&analysis.Pass{Pkg: self, Module: &analysis.Module{}}, other),
		"empty module path behaves like nil",
	)

	mod := &analysis.Module{Path: "example.test/mod"}
	assert.True(t, localToModule(&analysis.Pass{Pkg: self, Module: mod}, sub), "module subpath is local")
	assert.False(t, localToModule(&analysis.Pass{Pkg: self, Module: mod}, other), "foreign path is not local")
	root := types.NewPackage("example.test/mod", "mod")
	assert.True(t, localToModule(&analysis.Pass{Pkg: self, Module: mod}, root), "the module root package is local")
}

// TestObjectUsesPointerIgnoresNonAPIObjects covers the object kinds that
// establish no convention: package vars and alias TypeNames to unnamed types.
func TestObjectUsesPointerIgnoresNonAPIObjects(t *testing.T) {
	pkg := types.NewPackage("example.test/lib", "lib")
	named := types.NewNamed(types.NewTypeName(token.NoPos, pkg, "Node", nil), types.NewStruct(nil, nil), nil)

	v := types.NewVar(token.NoPos, pkg, "Default", types.NewPointer(named))
	assert.False(t, objectUsesPointer(v, named), "package vars establish no convention")

	alias := types.NewTypeName(token.NoPos, pkg, "Str", types.Typ[types.String])
	assert.False(t, objectUsesPointer(alias, named), "a TypeName for an unnamed type establishes no convention")
}
