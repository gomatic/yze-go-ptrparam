package ptrparam

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestForeignConventionNeverImmunisesABasicUnderlyingType names
// foreignConvention's exclusion. The immunity exists for foreign types whose
// own API is pointer-based — forcing values onto them would be wrong code, not
// style — but a named type with a BASIC underlying never earns it. The example
// in the doc is the reason: flag.DurationVar takes *time.Duration, and that is
// an OUT-PARAMETER binding to a value-idiomatic type, not a passing convention.
// Granting immunity there would exempt every *time.Duration parameter in the
// fleet from the rule.
//
// The exclusion mirrors recordPointer's in discover.go, so the generated
// allowlist and this runtime check agree; if they diverged, a type would be
// immune under one and reported under the other depending on which path ran.
func TestForeignConventionNeverImmunisesABasicUnderlyingType(t *testing.T) {
	t.Parallel()
	pkg := checkedPkg(t, `package p

import "time"

type Duration = time.Duration

type Seconds int64

type Buffer struct{ b []byte }
`)

	basic := namedOf(t, pkg, "Seconds")
	require.NotNil(t, basic)
	assert.False(t, foreignConvention(nil, basic),
		"a named type over a basic underlying is value-idiomatic; *T there is an out-parameter")

	structured := namedOf(t, pkg, "Buffer")
	require.NotNil(t, structured)
	assert.NotPanics(t, func() { _ = foreignConvention(passFor(pkg), structured) },
		"a structured type reaches the rest of the decision rather than being excluded outright")
}

// checkedPkg type-checks src and returns its package.
func checkedPkg(t *testing.T, src string) *types.Package {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, 0)
	require.NoError(t, err)
	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check("example.test/p", fset, []*ast.File{file}, nil)
	require.NoError(t, err)
	return pkg
}

// namedOf returns the named type called name, or nil when it is not one.
func namedOf(t *testing.T, pkg *types.Package, name string) *types.Named {
	t.Helper()
	obj := pkg.Scope().Lookup(name)
	require.NotNil(t, obj, "no type %s in the fixture", name)
	named, _ := types.Unalias(obj.Type()).(*types.Named)
	return named
}

// passFor builds a pass whose package is the fixture, which is what
// foreignConvention consults to decide whether a type is local to the module.
func passFor(pkg *types.Package) *analysis.Pass {
	return &analysis.Pass{Pkg: pkg}
}
