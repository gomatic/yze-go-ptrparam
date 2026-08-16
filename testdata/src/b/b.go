package b

// Special is a custom type callers may allow as a pointer parameter via config.
type Special struct{ n int }

// takesSpecial is permitted with -allow=b.Special.
func takesSpecial(s *Special) { _ = s }

// derivedSpecial is DEFINED over the allow-listed type. Flagged: -allow names
// one `pkgpath.Name`, and a configured disablement that spread to every type
// defined over the named one would silence types nobody configured. Special
// holds no lock and declares no method, so the definition inherits no hazard
// either.
type derivedSpecial Special

func takesDerivedSpecial(s *derivedSpecial) { _ = s } // want `pointer parameter`
