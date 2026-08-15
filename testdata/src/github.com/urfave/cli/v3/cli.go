// Package cli is a minimal stand-in for github.com/urfave/cli/v3, the
// sanctioned CLI framework whose *Command callback parameters are allowed.
//
// It carries the callback field that ESTABLISHES that, because that is what
// the real framework carries and what the real exemption reads. A hard-coded
// "github.com/urfave/cli/v3.Command" entry used to answer for it, and that
// entry was measured to decide nothing against the real library: the
// framework's own API takes *Command in every Action/Before/After signature,
// so the foreign-convention path answers it. A stand-in with no API would
// have kept the case passing while proving the wrong mechanism.
package cli

// ActionFunc is the callback shape the framework imposes on every command.
type ActionFunc func(cmd *Command) error

// Command mirrors the framework's command type.
type Command struct {
	Name   string
	Action ActionFunc
}
