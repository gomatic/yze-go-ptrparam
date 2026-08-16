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

// TestForeignConventionExemptsAnUnmaterialisedPackage pins the SIXTH
// exemption, and it is named for what it does rather than for what an earlier
// version of this comment wished it did. go/types populates a package's scope
// only as far as the loader needed it, so a foreign type reached through
// another package's alias re-export arrives with its own package holding one
// name and nothing else, and this branch exempts it.
//
// THIS TEST DOES NOT SAY THE BEHAVIOUR IS RIGHT, and the previous comment came
// close to saying so. It justified the exemption on the ground that the
// alternative "follows the import list" — while this polarity follows the
// import list too, in the other direction, which is a limitation being blessed
// rather than a contract being asserted (RULE 13). Measured: a four-line
// `type Doc = ast.Doc` package in the author's own module silences the
// library's type under `go vet -vettool` while the identical source reports
// under a packages.Load driver, and one blank import flips the vet verdict
// back. Neither polarity is import-independent, because blindness cannot tell
// "this library publishes no pointer convention" from "this library was not
// loaded"; this one was chosen because it costs a package a reviewer can see
// rather than one import line nobody reads.
//
// So what is asserted here is the BRANCH, so that flipping the polarity is a
// deliberate act with a failing test behind it — not that the branch is
// correct. It is ptrparam.verdict-does-not-follow-the-loaders-reach (k1n81qeg),
// open, and the repair is a loader that materialises the package rather than
// either polarity. No corpus case can reach this: analysistest type-checks
// every dependency from source, so Complete() is always true there, which is
// why this synthetic pass is the only coverage the branch has.
func TestForeignConventionExemptsAnUnmaterialisedPackage(t *testing.T) {
	t.Parallel()
	analysed := types.NewPackage("example.test/mod/pkg", "pkg")
	analysed.MarkComplete()

	unloaded := types.NewPackage("example.test/lib", "lib")
	hidden := types.NewNamed(types.NewTypeName(token.NoPos, unloaded, "Node", nil), types.NewStruct(nil, nil), nil)
	require.False(t, unloaded.Complete(), "the loader materialised nothing but the name")
	assert.True(t, foreignConvention(&analysis.Pass{Pkg: analysed}, hidden),
		"an unmaterialised library is EXEMPTED, which is the branch under test and not a claim that it should be")

	loaded := types.NewPackage("example.test/lib2", "lib2")
	seen := types.NewNamed(types.NewTypeName(token.NoPos, loaded, "Options", nil), types.NewStruct(nil, nil), nil)
	loaded.Scope().Insert(seen.Obj())
	loaded.MarkComplete()
	assert.False(t, foreignConvention(&analysis.Pass{Pkg: analysed}, seen),
		"a library that WAS materialised and hands out no pointer establishes no convention")
}

// TestForeignConventionReadsNoPackageButTheTypesOwn is the k1n828c6 repro
// reduced to a unit test, and it is a unit test because no corpus case can
// carry it: analysistest loads GOPATH-style sources, so `pass.Module` is nil
// there and module locality is not assertable from a fixture at all
// (docs/i01.md, "analysistest has no module").
//
// The shape is the forgery the sibling scan could not refuse. A library
// publishes `Doc` in `github.com/fakelib/gql/v2/ast` and mentions `*Doc`
// nowhere. A package at `github.com/fakelib/gql/v2/weaver` — inside the
// library's import space, foreign to the analyzed module, and therefore
// everything the sibling scan asked for — hands the pointer out. Three files
// and a `replace` directive put that package in the author's own tree, so the
// marker cost an import path and an import path costs a line of `go.mod`.
// Nothing is published, the library still has no pointer convention, and
// `func Copy(d ast.Doc) ast.Doc` is still correct code: docs/s03.md's
// acquisition test fails outright, which is why the scan is gone rather than
// narrowed.
//
// The refusal has to be unconditional. `types.Package` carries no module
// identity (Path, Name, GoVersion, Scope, Complete, Imports and nothing else)
// and `analysis.Pass.Module` describes the ANALYZED package's module alone, so
// a pass cannot tell a fetched package at that path from one the build
// supplied — which makes "read a sibling, but only a published one" a
// distinction this analyzer has no instrument for.
func TestForeignConventionReadsNoPackageButTheTypesOwn(t *testing.T) {
	t.Parallel()
	lib := types.NewPackage("github.com/fakelib/gql/v2/ast", "ast")
	doc := mkNamed(lib, "Doc", types.NewVar(token.NoPos, lib, "n", types.Typ[types.Int]))
	lib.Scope().Insert(doc.Obj())
	lib.MarkComplete()

	inLibrarySpace := handsOutPointer(t, "github.com/fakelib/gql/v2/weaver", "weaver", doc)
	outsideIt := handsOutPointer(t, "example.com/app/forge", "forge", doc)

	assert.False(t, foreignConvention(judging(t, lib, inLibrarySpace), doc),
		"a package inside the library's import space is still a package the analyzed build chose to supply")
	assert.False(t, foreignConvention(judging(t, lib, outsideIt), doc),
		"and one outside it establishes nothing either — the type's own package is the whole signal")

	// The positive control: the same shape with the pointer handed out by the
	// type's OWN package. Without it the two refusals above are satisfied by an
	// analyzer that exempts nothing at all.
	results := types.NewTuple(types.NewVar(token.NoPos, lib, "", types.NewPointer(doc)))
	lib.Scope().Insert(types.NewFunc(token.NoPos, lib, "Parse",
		types.NewSignatureType(nil, nil, nil, types.NewTuple(), results, false)))
	assert.True(t, foreignConvention(judging(t, lib), doc),
		"the type's OWN package handing the pointer out still conventions it, so the refusals above discriminate")
}

// TestForeignConventionDoesNotFollowTheJudgedPackagesImports is the other side
// of the same refusal, stated as the property rather than as the forgery: the
// verdict on a foreign type is a function of the type and its library, so
// adding or removing an import must not move it. Reading the judged package's
// imports is what made a blank import a disablement (k1n80q9h) and what made
// the sibling scan forgeable (k1n828c6); both are the same read.
func TestForeignConventionDoesNotFollowTheJudgedPackagesImports(t *testing.T) {
	t.Parallel()
	lib := types.NewPackage("github.com/fakelib/gql/v2/ast", "ast")
	doc := mkNamed(lib, "Doc", types.NewVar(token.NoPos, lib, "n", types.Typ[types.Int]))
	lib.Scope().Insert(doc.Obj())
	lib.MarkComplete()

	publisher := handsOutPointer(t, "github.com/fakelib/gql/v2/formatter", "formatter", doc)

	bare := foreignConvention(judging(t, lib), doc)
	withImport := foreignConvention(judging(t, lib, publisher), doc)
	assert.Equal(t, bare, withImport,
		"one import line may not change the verdict on somebody else's type")
	assert.False(t, bare, "and the verdict both ways is the one the library's own package supports")
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

// judging builds a pass judging a package of example.com/app that imports the
// given packages. The module is fixed because every caller judges from the same
// module: what varies between the cases is which packages are on the import
// list, which is the dimension the refusals below are about. localToModule's
// own module-path arithmetic is covered directly by TestLocalToModule, which is
// where a second module path would earn its keep.
func judging(t *testing.T, imports ...*types.Package) *analysis.Pass {
	t.Helper()
	judged := types.NewPackage("example.com/app/judged", "judged")
	judged.SetImports(imports)
	judged.MarkComplete()
	return &analysis.Pass{Pkg: judged, Module: &analysis.Module{Path: "example.com/app"}}
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
