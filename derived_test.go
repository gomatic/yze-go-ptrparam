package ptrparam

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInheritsHazardTakesTheLayoutHazardAndNeitherOtherExemption names the
// invariant the definition chain rests on, and it is asserted here because no
// corpus case can see it: a case can only observe that a parameter is
// reported, never WHICH of the five exemptions declined it, so a chain that
// quietly inherited a foreign library's convention or an -allow entry looks
// exactly like one that inherited nothing.
//
// What a definition copies is the LAYOUT, so what it inherits is the layout's
// copy hazard — a lock, or a method set that exists only on the pointer. A
// foreign library's convention is a claim about the library's own type, and no
// signature in that library can be handed the local one; an -allow entry names
// a single `pkgpath.Name`. Both were inherited before this test existed, and
// both made one `type` line the cheapest silence in the analyzer.
func TestInheritsHazardTakesTheLayoutHazardAndNeitherOtherExemption(t *testing.T) {
	t.Parallel()
	local := types.NewPackage("x", "x")

	// The three sources: one exempt by an -allow entry, one exempt by its own
	// library's published pointer, one exempt by the layout hazard itself.
	allowed := mkNamed(local, "Special", types.NewVar(token.NoPos, local, "n", types.Typ[types.Int]))
	handed := handedOutByPointer(t)
	locked := mkNamed(local, "Locky")
	addLocker(local, locked, true)

	overAllowed := mkNamedOver(local, "OverAllowed", allowed.Underlying())
	overHanded := mkNamedOver(local, "OverHanded", handed.Underlying())
	overLocked := mkNamedOver(local, "OverLocked", locked.Underlying())
	twoLinks := mkNamedOver(local, "TwoLinks", locked.Underlying())

	d := decision{
		pass:  passFor(local),
		allow: map[string]bool{"x.Special": true},
		over: definitions{
			overAllowed: allowed,
			overHanded:  handed,
			overLocked:  locked,
			twoLinks:    overLocked,
		},
	}

	require.True(t, conventionedHere(d, allowed), "the -allow entry answers for the type it names")
	require.True(t, conventionedHere(d, handed), "the library's own API answers for the library's type")

	assert.False(t, conventioned(d, overAllowed),
		"a configured disablement names one type and stops there")
	assert.False(t, conventioned(d, overHanded),
		"no signature in the library can be handed a type declared here, so its convention does not reach one")
	assert.True(t, conventioned(d, overLocked),
		"a lock in the layout is copied along with the layout")
	assert.True(t, conventioned(d, twoLinks),
		"and the chain does not stop after one link")
}

// handedOutByPointer builds a complete foreign package whose exported API hands
// out a pointer to a type carrying no hazard of its own: no lock, no method.
// It is the shape foreignConvention exists for and the shape a definition over
// it must not inherit.
func handedOutByPointer(t *testing.T) *types.Named {
	t.Helper()
	lib := types.NewPackage("y", "y")
	thing := mkNamed(lib, "Thing", types.NewVar(token.NoPos, lib, "n", types.Typ[types.Int]))
	results := types.NewTuple(types.NewVar(token.NoPos, lib, "", types.NewPointer(thing)))
	sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(), results, false)
	lib.Scope().Insert(types.NewFunc(token.NoPos, lib, "New", sig))
	lib.MarkComplete()
	return thing
}
