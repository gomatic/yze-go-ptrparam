// Package d imports a stdlib package (flag) whose exported API takes a
// pointer to a basic-underlying named type (flag.DurationVar's
// *time.Duration). That is an out-parameter binding to a value-idiomatic
// type, not a passing convention — exactly the exclusion discover.go's
// recordPointer applies to the generated allowlist — so the import must not
// grant foreign-convention immunity to *time.Duration parameters here.
package d

import (
	"flag"
	"time"
)

var timeout time.Duration

// register uses flag so the package is a direct import of the analyzed
// package, which is what feeds foreignConvention's direct-imports scan.
func register() { flag.DurationVar(&timeout, "timeout", 0, "") }

// setTimeout is flagged: time.Duration is value-idiomatic; flag.DurationVar's
// *time.Duration is an out-parameter, not a convention to inherit.
func setTimeout(d *time.Duration) { _ = d } // want `pointer parameter`
