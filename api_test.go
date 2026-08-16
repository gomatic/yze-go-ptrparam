package ptrparam

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
