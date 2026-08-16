// Package constraintcycle holds the type-parameter constraint spellings that
// refer to the parameter they constrain. Each one is legal Go — `go build` and
// `go vet` both exit 0 on every declaration here — and each one asks the
// uncopyable walk to descend from a generic type's field, into the field's type
// parameter, into that parameter's constraint, and back to the same parameter.
// Without a cycle guard the walk never returns and the process dies with
// `fatal error: stack overflow`, which costs every analyzer in the run and not
// only this one.
//
// None of these constraints admits a lock, so every subject parameter below is
// REPORTED. That is what makes the file a case rather than a smoke test: a
// guard implemented as "give up and call it uncopyable" would silence them all.
package constraintcycle

// Ctl is an ordinary local struct, and the control below is reported at every
// revision of this analyzer. It is here so a file that reports nothing at all
// — because the run died before it started, or because a guard silenced
// everything — is distinguishable from a file whose subjects are exempt.
type Ctl struct{ N int }

// control is reported: a pointer to a local struct with no pointer convention.
func control(c *Ctl) { _ = c } // want `pointer parameter`

// G is the generic carrier. Its field is what takes the walk from a named type
// into a type parameter; a constraint is unreachable without one.
type G[T any] struct{ V T }

// Elem is an F-bounded constraint: its type set is spelled in terms of the
// parameter it constrains.
type Elem[T any] interface{ ~struct{ X T } }

// AliasElem is the same constraint written as a generic alias, which arrives at
// the walk as an alias rather than as a named type.
type AliasElem[T any] = interface{ ~struct{ X T } }

// Inner is an F-bounded constraint reached only through an embedded interface.
type Inner[T any] interface{ ~struct{ X T } }

// Outer embeds Inner rather than spelling its type set, which is the one
// spelling in this file the walk does not descend into.
type Outer[T any] interface{ Inner[T] }

// inlineConstraint is reported: the constraint is written at the parameter.
func inlineConstraint[T interface{ ~struct{ N T } }](g *G[T]) { _ = g } // want `pointer parameter`

// namedConstraint is reported: the same cycle through a named constraint.
func namedConstraint[T Elem[T]](g *G[T]) { _ = g } // want `pointer parameter`

// mutualConstraints is reported: neither parameter names itself, and the cycle
// closes across the pair.
func mutualConstraints[U Elem[V], V Elem[U]](g *G[U]) { _ = g } // want `pointer parameter`

// arrayTerm is reported: the cycle closes through an array element rather than
// through a struct field.
func arrayTerm[T interface{ ~struct{ X [1]T } }](g *G[T]) { _ = g } // want `pointer parameter`

// bareArrayTerm is reported: the constraint element is an array and no struct
// takes part at all, so a guard written for struct terms alone would miss it.
func bareArrayTerm[T interface{ ~[1]T }](g *G[T]) { _ = g } // want `pointer parameter`

// aliasConstraint is reported: the cycle closes through a generic alias.
func aliasConstraint[T AliasElem[T]](g *G[T]) { _ = g } // want `pointer parameter`

// unionSecondTerm is reported: the first term of the union is copyable, so the
// walk does not short-circuit before reaching the cyclic second term.
func unionSecondTerm[T interface{ ~struct{ A int } | ~struct{ N T } }](g *G[T]) { _ = g } // want `pointer parameter`

// instantiationTerm is reported: the cyclic term mentions the carrier's own
// instantiation rather than the parameter directly.
func instantiationTerm[T interface{ ~struct{ X G[T] } }](g *G[T]) { _ = g } // want `pointer parameter`

// embeddedConstraint is reported, and the cycle it closes runs THROUGH an
// embedded interface: the walk enters Outer's element, finds Inner, enters
// Inner's type set, and arrives back at T. An earlier revision of this file
// asserted the opposite — that the walk stopped at the embedding — and pinned
// it with this same `want`. The assertion was insensitive either way, because
// Outer and Inner hold no lock, so the comment made a defect look covered while
// the case could not tell the two behaviours apart. The lock-bearing sibling
// that CAN tell them apart is takesEmbeddedLock in testdata/src/a.
func embeddedConstraint[T Outer[T]](g *G[T]) { _ = g } // want `pointer parameter`
