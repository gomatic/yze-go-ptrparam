package b

// Special is a custom type callers may allow as a pointer parameter via config.
type Special struct{ n int }

// takesSpecial is permitted with -allow=b.Special.
func takesSpecial(s *Special) { _ = s }
