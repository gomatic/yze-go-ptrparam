// The configurable allow-list: the one exemption that is set rather than
// computed, and the only one that leaves no trace when it fires.
package ptrparam

import "strings"

// allowExtra is the configurable allow-list of additional fully-qualified
// pointer-parameter types (pkgpath.Name), set via the -allow flag or config.
var allowExtra string

// allowCSV is the raw comma-separated -allow flag value listing extra
// fully-qualified pointer-parameter types.
type allowCSV string

// buildAllow is the configured extras, which are the whole allow-list.
//
// It used to merge two hard-coded maps as well: a generated 474-entry
// standard-library table and one entry for urfave/cli/v3's *cli.Command.
// Neither of them decided a verdict, on every input that was measured against
// binaries built with each map deleted — the measurement is in the commit that
// removed them. Both are answered by pointerIdiomatic and foreignConvention,
// which compute the same criteria from the types themselves rather than from a
// table that needs regenerating each time the toolchain moves.
func buildAllow(extra allowCSV) map[string]bool {
	allow := make(map[string]bool)
	for _, name := range splitNonEmpty(extra) {
		allow[name] = true
	}
	return allow
}

// splitNonEmpty splits the configured list, treating the unset flag as no
// entries at all.
//
// Its empty check cannot change a verdict, so no case should be written for
// one: without it an unset flag yields the single entry "", and every lookup
// is a package path, a dot and a type name, so the map would gain one key that
// nothing matches. It keeps the map honest; it decides nothing.
func splitNonEmpty(value allowCSV) []string {
	if value == "" {
		return nil
	}
	return strings.Split(string(value), ",")
}
