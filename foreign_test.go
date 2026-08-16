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

	sibling := types.NewPackage("example.test/modular", "modular")
	assert.False(t, localToModule(&analysis.Pass{Pkg: self, Module: mod}, sibling),
		"a module path is a whole path element: example.test/modular is not inside example.test/mod")
}

// TestForeignConventionRefusesTheAnalyzedModulesOwnTypes names the split the
// whole exemption rests on: a foreign type follows its library's design, and
// the analyzed module's own types are the analyzed code's design
// responsibility. Dropping the localToModule call would hand the exemption to
// every module-local type whose own package exports anything taking *T, and
// the rule very nearly self-cancels — every exported constructor returning *T
// establishes the convention for T.
//
// The corpus cannot hold this case: analysistest loads no module metadata, so
// pass.Module is nil there and a sibling package looks foreign to it.
func TestForeignConventionRefusesTheAnalyzedModulesOwnTypes(t *testing.T) {
	t.Parallel()
	mine := types.NewPackage("example.test/mod/lib", "lib")
	node := types.NewNamed(types.NewTypeName(token.NoPos, mine, "Node", nil), types.NewStruct(nil, nil), nil)
	parse := types.NewFunc(token.NoPos, mine, "Parse",
		types.NewSignatureType(nil, nil, nil, nil,
			types.NewTuple(types.NewVar(token.NoPos, mine, "", types.NewPointer(node))), false))
	mine.Scope().Insert(node.Obj())
	mine.Scope().Insert(parse)
	mine.MarkComplete()

	consumer := types.NewPackage("example.test/mod/pkg", "pkg")
	consumer.MarkComplete()
	mod := &analysis.Module{Path: "example.test/mod"}

	assert.True(t, apiUsesPointer(mine, node), "the package does hand out *Node — the scan is not the reason")
	assert.False(t, foreignConvention(&analysis.Pass{Pkg: consumer, Module: mod}, node),
		"a type inside the analyzed module is the analyzed code's own design, whatever its own package hands out")

	elsewhere := &analysis.Module{Path: "example.test/other"}
	assert.True(t, foreignConvention(&analysis.Pass{Pkg: consumer, Module: elsewhere}, node),
		"the identical type outside the analyzed module follows its library")
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
// The exclusion is the only thing standing between the rule and every
// *time.Duration, *int64 and *MyEnum parameter in the fleet.
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

// TestForeignConventionDeclinesToJudgeAnUnmaterialisedPackage names the
// scope limitation foreignConvention rests on. go/types populates a package's
// scope only as far as the loader needed it, so a foreign type reached
// through another package's alias re-export arrives with its own package
// holding one name and nothing else. Scanning that scope reports "no
// convention" for a library whose convention was merely not loaded — and one
// blank import of the unloaded package completes the scope and turns the same
// finding silent, which is a verdict that follows the import list rather than
// the code. Where the analyzer cannot read the library, it renders no verdict.
func TestForeignConventionDeclinesToJudgeAnUnmaterialisedPackage(t *testing.T) {
	t.Parallel()
	analysed := types.NewPackage("example.test/mod/pkg", "pkg")
	analysed.MarkComplete()

	unloaded := types.NewPackage("example.test/lib", "lib")
	hidden := types.NewNamed(types.NewTypeName(token.NoPos, unloaded, "Node", nil), types.NewStruct(nil, nil), nil)
	require.False(t, unloaded.Complete(), "the loader materialised nothing but the name")
	assert.True(t, foreignConvention(&analysis.Pass{Pkg: analysed}, hidden),
		"a library the loader never materialised is not a library with no pointer convention")

	loaded := types.NewPackage("example.test/lib2", "lib2")
	seen := types.NewNamed(types.NewTypeName(token.NoPos, loaded, "Options", nil), types.NewStruct(nil, nil), nil)
	loaded.Scope().Insert(seen.Obj())
	loaded.MarkComplete()
	assert.False(t, foreignConvention(&analysis.Pass{Pkg: analysed}, seen),
		"a library that WAS materialised and hands out no pointer establishes no convention")
}

// TestLibrarySiblingUsesPointerRefusesTheAnalyzedModule is the case no corpus
// case in this repo can be: analysistest loads GOPATH-style sources, so
// `pass.Module` is nil there and every package reads as foreign. Module
// locality is therefore only assertable against a pass built by hand, and it is
// load-bearing rather than decorative.
//
// The shape is a library whose type sits in its MODULE ROOT. `path.Dir` of
// `github.com/acme/lib` is `github.com/acme` — an owner namespace, not a
// library — and it contains `github.com/acme/app`. Without the module test an
// author writes two lines in their own module, blank-imports them, and the
// library's type goes silent: the one-line evasion this whole scan exists to
// prevent, wearing the scan's own uniform.
func TestLibrarySiblingUsesPointerRefusesTheAnalyzedModule(t *testing.T) {
	t.Parallel()
	lib := types.NewPackage("github.com/acme/lib", "lib")
	doc := mkNamed(lib, "Doc", types.NewVar(token.NoPos, lib, "n", types.Typ[types.Int]))
	lib.Scope().Insert(doc.Obj())
	lib.MarkComplete()

	own := handsOutPointer(t, "github.com/acme/app/forge", "forge", doc)
	library := handsOutPointer(t, "github.com/acme/lib/format", "format", doc)

	assert.False(t, foreignConvention(judging(t, "github.com/acme/app", lib, own), doc),
		"a package of the analyzed module cannot establish a convention for somebody else's type")
	assert.True(t, foreignConvention(judging(t, "github.com/acme/app", lib, library), doc),
		"a package the library owns still can, which is what makes the refusal above a discrimination")
	assert.True(t, foreignConvention(judging(t, "", lib, own), doc),
		"and without module metadata the conservative fallback stands: only the judged package is local")
}

// handsOutPointer builds a complete package at the given import path whose
// exported API hands out a pointer to named.
func handsOutPointer(t *testing.T, at, name string, named *types.Named) *types.Package {
	t.Helper()
	pkg := types.NewPackage(at, name)
	results := types.NewTuple(types.NewVar(token.NoPos, pkg, "", types.NewPointer(named)))
	sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(), results, false)
	pkg.Scope().Insert(types.NewFunc(token.NoPos, pkg, "Hand", sig))
	pkg.MarkComplete()
	return pkg
}

// judging builds a pass whose analyzed package imports the given packages and
// whose module is the one named, or none when the path is empty.
func judging(t *testing.T, module string, imports ...*types.Package) *analysis.Pass {
	t.Helper()
	judged := types.NewPackage("github.com/acme/app/judged", "judged")
	judged.SetImports(imports)
	judged.MarkComplete()
	pass := &analysis.Pass{Pkg: judged}
	if module != "" {
		pass.Module = &analysis.Module{Path: module}
	}
	return pass
}

// TestMentionsPointerNeverFollowsTheGenericOrigin names mentionsPointer's
// invariant: a library that hands out one instantiation of a generic type
// establishes the convention for THAT instantiation and for no other.
// Comparing generic origins instead would exempt every instantiation of any
// generic the library mentions once, which is the escape — for a foreign
// generic conventioned at a single instantiation, every other instantiation
// would be free.
func TestMentionsPointerNeverFollowsTheGenericOrigin(t *testing.T) {
	t.Parallel()
	pkg := checkedPkg(t, `package p

type Box[T any] struct{ V T }

func MakeInt() *Box[int] { return nil }

func MakeInts() []*Box[int] { return nil }
`)

	handed := resultOf(t, pkg, "MakeInt")
	intBox := namedOf(t, pkg, "Box")
	require.NotNil(t, intBox)

	instantiated, err := types.Instantiate(nil, intBox.Origin(), []types.Type{types.Typ[types.Int]}, true)
	require.NoError(t, err)
	other, err := types.Instantiate(nil, intBox.Origin(), []types.Type{types.Typ[types.String]}, true)
	require.NoError(t, err)

	assert.True(t, mentionsPointer(handed, instantiated.(*types.Named)),
		"the instantiation the library hands out is conventioned")
	assert.False(t, mentionsPointer(handed, other.(*types.Named)),
		"an instantiation the library never mentions is not conventioned by its sibling")

	slice := resultOf(t, pkg, "MakeInts")
	assert.True(t, mentionsPointer(slice, instantiated.(*types.Named)), "one container level deep still counts")
	assert.False(t, mentionsPointer(slice, other.(*types.Named)), "and it still does not cross instantiations")
}

// resultOf returns the sole result type of the package-scope function name.
func resultOf(t *testing.T, pkg *types.Package, name string) types.Type {
	t.Helper()
	fn, ok := pkg.Scope().Lookup(name).(*types.Func)
	require.True(t, ok, "no func %s in the fixture", name)
	return fn.Type().(*types.Signature).Results().At(0).Type()
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
