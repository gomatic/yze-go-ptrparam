// Package main is a command, and a command is code: nothing about an
// entrypoint takes it out of the rule. An early return keyed on the package
// clause would exempt every CLI in the fleet and add no statement to see it
// by, so this package is what refuses it.
package main

// Opts is an ordinary value type.
type Opts struct{ n int }

func take(o *Opts) { _ = o } // want `pointer parameter`

func main() { take(&Opts{}) }
