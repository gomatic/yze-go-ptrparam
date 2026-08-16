// Foreign-convention detection: the analyzed module's own types are its design
// responsibility, but a foreign type follows its library's conventions.
package ptrparam

import (
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// foreignConvention reports whether a type from OUTSIDE the analyzed module is
// conventionally handled by pointer: the module's own types are the analyzed
// code's design responsibility, but a foreign type follows its library's
// design — when the library's exported API hands out or accepts *T (a parser
// returning *ast.Document whose nodes are aliased and mutated in place),
// forcing values onto it would be wrong code, not style.
//
// THE SIGNAL IS THE TYPE'S OWN PACKAGE AND NOTHING ELSE. No package the
// ANALYZED build supplies is read, whatever its import path, because an import
// path is a claim that build makes rather than one anybody published.
//
// It was read from the judged package's imports twice, and both readings were
// the same defect at different widths. The first read every direct import, on
// the reasoning that constructors often live beside the type rather than
// inside its package; the reasoning is right and it made `_ "weaver"` — one
// line that changes nothing else and reads as ordinary housekeeping, naming a
// package two lines long the author wrote themselves — turn a reported
// parameter silent. The second narrowed the scan to imports inside the space
// the type's package hangs off, on the reasoning that "an author cannot write
// a package there without owning the library's path". A go.mod `replace`
// directive makes that false: three files in the author's own tree — a
// seven-line go.mod declaring the path, a one-line `func Touch(n *ast.Node)`,
// and a `replace` — put a package at ANY import path, and the blank import
// silenced a library type under the standalone binary, under
// `go vet -vettool`, and under the yze runner alike (k1n828c6).
//
// The distinction the narrowing needed is one this analyzer has no instrument
// for, and that is measured rather than assumed. `types.Package` carries no
// module identity — Path, Name, GoVersion, Scope, Complete, Imports and
// nothing else — and `analysis.Pass.Module` describes the ANALYZED package's
// module alone, so a pass cannot tell a fetched package at an import path from
// one the build supplied through a `replace` or a workspace. The one remaining
// signal, an imported object's file position, is a path the build also
// chooses: `GOMODCACHE` is an environment variable and a vendored build puts
// every dependency in the local tree, while `Pass.Module.Dir` is empty under
// `go vet` entirely. So "read a sibling, but only a published one" is not a
// narrower scan; it is a scan with no test behind it.
//
// WHAT THAT COSTS, stated rather than implied, because it is the reason the
// scan existed. Go libraries put types in `ast/` and the operations on them
// beside it: `github.com/vektah/gqlparser/v2/ast` mentions `*QueryDocument`
// nowhere, and `parser`, `validator`, `formatter` and the root package all
// hand it out or take it. Such a type is reported here, and the remedy was
// built and RUN: passing the value loses the appended operation silently, with
// go vet and this analyzer both exiting 0 on the remedied source. That is a
// diagnostic whose remedy corrupts, and it is left standing deliberately —
// the move available to its author is the `-allow` entry, which is a line a
// reviewer and an inventory can both read, rather than a blank import that
// leaves no entry anywhere. Closing it properly needs the type's own module
// enumerated, which a pass cannot do and the framework can
// (ptrparam.sibling-published-convention-is-read, k1n81qfk).
//
// Where the loader did not materialise the library, the analyzer renders no
// verdict on it. go/types populates a package's scope only as far as the
// loader needed it, so a type reached through another package's alias
// re-export arrives with its own package holding one name and nothing else;
// scanning that scope answers "no convention" for a library whose convention
// was merely not loaded. An unreadable library is not a library with no
// pointer convention.
//
// THIS IS A DISABLEMENT CHANNEL, not merely a limitation, and it is recorded
// as one rather than described as a virtue. An author who writes a two-line
// alias package in their own module and names the type through it keeps the
// type's own package out of the load, and the parameter goes silent — one
// blank import of that package puts it back. Neither polarity is
// import-independent, because blindness cannot tell "this library publishes no
// pointer convention" from "this library was not loaded"; this one was chosen
// because it costs a new package a reviewer can see rather than one invisible
// import line. The repair is not a polarity: it is a loader that materialises
// the package, which belongs to the framework.
//
// A named type with basic underlying never gains the immunity: an API taking
// *T there (flag.DurationVar's *time.Duration) is an out-parameter binding to
// a value-idiomatic type, not a passing convention.
func foreignConvention(pass *analysis.Pass, named *types.Named) bool {
	if _, isBasic := named.Underlying().(*types.Basic); isBasic {
		return false
	}
	pkg := named.Obj().Pkg()
	if localToModule(pass, pkg) {
		return false
	}
	if !pkg.Complete() {
		return true
	}
	return apiUsesPointer(pkg, named)
}

// localToModule reports whether pkg belongs to the analyzed module. Without
// module metadata (a driver that does not load it), only the analyzed package
// itself counts as local — the conservative fallback.
func localToModule(pass *analysis.Pass, pkg *types.Package) bool {
	if pkg == pass.Pkg {
		return true
	}
	if pass.Module == nil || pass.Module.Path == "" {
		return false
	}
	return pkg.Path() == pass.Module.Path || strings.HasPrefix(pkg.Path(), pass.Module.Path+"/")
}
