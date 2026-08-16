// Package ast is the ordinary Go library layout the foreign-convention
// exemption was written for and stopped covering: the type lives here and
// every operation on it lives beside it. Nothing in this package mentions
// *Doc, exactly as gqlparser's ast package mentions *QueryDocument nowhere.
package ast

// Doc is the parser output whose nodes are aliased and mutated in place.
type Doc struct{ Ops []int }
