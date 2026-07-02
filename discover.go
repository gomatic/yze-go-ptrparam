//go:build ignore

// Command discover regenerates allowlist_std.go: the standard-library types
// where a pointer parameter is idiomatic. Run it with the toolchain the
// analyzer should track:
//
//	go run discover.go
//
// A type qualifies when a caller cannot sensibly hold or use it by value:
//
//   - uncopyable — the type (transitively, through non-pointer struct fields
//     and arrays) contains a lock: something with a pointer-receiver Lock
//     method, i.e. what go vet's copylocks protects. Copying corrupts it, so
//     the pointer is the only correct way to pass it.
//   - pointer-only methods — the type declares at least one exported method
//     and every one of them requires the pointer receiver. A value parameter
//     would be unable to do anything the type exists for, and the package's
//     own constructors hand out the pointer. (Declared methods, not the full
//     method set: a pointer-typed embed like text/template.Template's
//     *parse.Tree promotes pointer methods into the value set and would
//     otherwise mask the convention.)
//   - stdlib passes it — the standard library's own exported API takes *T as
//     a parameter somewhere (directly, or inside a func-typed parameter or an
//     exported struct field's func type, e.g. flag.Visit's func(*flag.Flag)).
//     Users implementing those callbacks have no choice but to accept the
//     pointer. Pointers to basic-kinded types are excluded: flag.DurationVar's
//     *time.Duration is an out-parameter binding to a value-idiomatic type,
//     not a passing convention.
//
// Value-idiomatic types (time.Time, netip.Addr, reflect.Value…) fail all
// three tests and stay flagged by the analyzer.
package main

import (
	"fmt"
	"go/types"
	"log"
	"os"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

const output = "allowlist_std.go"

func main() {
	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedName | packages.NeedTypes}, "std")
	if err != nil {
		log.Fatal(err)
	}
	entries := map[string]string{}
	for _, pkg := range pkgs {
		if skipPackage(pkg.PkgPath) {
			continue
		}
		collect(pkg.Types, entries)
	}
	for _, pkg := range pkgs {
		if skipPackage(pkg.PkgPath) {
			continue
		}
		collectPassed(pkg.Types, entries)
	}
	if err := os.WriteFile(output, render(entries), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %s: %d types\n", output, len(entries))
}

// skipPackage drops non-importable trees: internal packages and vendored
// module shims have no user-visible import path, so their types can never
// appear in analyzed signatures by name.
func skipPackage(path string) bool {
	return path == "internal" ||
		strings.HasPrefix(path, "internal/") ||
		strings.Contains(path, "/internal") ||
		strings.HasPrefix(path, "vendor/")
}

// collect records every exported qualifying type in scope.
func collect(pkg *types.Package, entries map[string]string) {
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj, ok := scope.Lookup(name).(*types.TypeName)
		if !ok || !obj.Exported() || obj.IsAlias() {
			continue
		}
		named, ok := obj.Type().(*types.Named)
		if !ok {
			continue
		}
		if reason := qualifies(named); reason != "" {
			entries[pkg.Path()+"."+name] = reason
		}
	}
}

// qualifies reports why a type is pointer-idiomatic, or "" when it is not.
func qualifies(named *types.Named) string {
	if uncopyable(named.Underlying(), map[types.Type]bool{}) {
		return "uncopyable"
	}
	if pointerOnlyMethods(named) {
		return "pointer-only methods"
	}
	return ""
}

// pointerOnlyMethods reports whether the type declares exported methods and
// every one of them takes the pointer receiver.
func pointerOnlyMethods(named *types.Named) bool {
	if _, isIface := named.Underlying().(*types.Interface); isIface {
		return false
	}
	exported := 0
	for i := range named.NumMethods() {
		m := named.Method(i)
		if !m.Exported() {
			continue
		}
		exported++
		recv := m.Type().(*types.Signature).Recv().Type()
		if _, isPtr := recv.(*types.Pointer); !isPtr {
			return false
		}
	}
	return exported > 0
}

// collectPassed records every exported type the package's exported API takes
// by pointer: parameters of exported functions and of exported types' methods,
// and func-typed exported struct fields — each walked recursively through
// func-typed parameters (callbacks).
func collectPassed(pkg *types.Package, entries map[string]string) {
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		collectPassedObject(obj, entries)
	}
}

// collectPassedObject fans one package-scope object into the signature walk.
func collectPassedObject(obj types.Object, entries map[string]string) {
	switch o := obj.(type) {
	case *types.Func:
		passedParams(o.Type().(*types.Signature), entries)
	case *types.TypeName:
		collectPassedType(o, entries)
	}
}

// collectPassedType walks an exported type's declared exported methods and,
// for structs, its exported func-typed fields.
func collectPassedType(obj *types.TypeName, entries map[string]string) {
	named, ok := obj.Type().(*types.Named)
	if !ok {
		return
	}
	for i := range named.NumMethods() {
		if m := named.Method(i); m.Exported() {
			passedParams(m.Type().(*types.Signature), entries)
		}
	}
	structFields(named, entries)
}

// structFields walks exported func-typed fields of a struct type (callback
// configuration like tls.Config.GetCertificate).
func structFields(named *types.Named, entries map[string]string) {
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return
	}
	for f := range st.Fields() {
		sig, ok := f.Type().Underlying().(*types.Signature)
		if ok && f.Exported() {
			passedParams(sig, entries)
		}
	}
}

// passedParams records each exported named type appearing as a pointer
// parameter, recursing into func-typed parameters.
func passedParams(sig *types.Signature, entries map[string]string) {
	params := sig.Params()
	for i := range params.Len() {
		t := params.At(i).Type()
		if s, ok := t.(*types.Slice); ok {
			t = s.Elem()
		}
		recordPointer(t, entries)
		if inner, ok := t.Underlying().(*types.Signature); ok {
			passedParams(inner, entries)
		}
	}
}

// recordPointer records t when it is a pointer to an exported, non-internal,
// non-basic named type that has not already qualified. A pointer to a
// basic-kinded type (e.g. *time.Duration in flag.DurationVar) is an
// out-parameter binding, not a passing convention.
func recordPointer(t types.Type, entries map[string]string) {
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return
	}
	named, ok := ptr.Elem().(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return
	}
	if !named.Obj().Exported() || skipPackage(named.Obj().Pkg().Path()) {
		return
	}
	if _, isBasic := named.Underlying().(*types.Basic); isBasic {
		return
	}
	key := named.Obj().Pkg().Path() + "." + named.Obj().Name()
	if _, seen := entries[key]; !seen {
		entries[key] = "stdlib passes it"
	}
}

// uncopyable reports whether t transitively holds a lock — the copylocks
// criterion: any component type whose pointer method set has a Lock method.
func uncopyable(t types.Type, seen map[types.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	if hasPointerLock(t) {
		return true
	}
	switch u := t.Underlying().(type) {
	case *types.Struct:
		for f := range u.Fields() {
			if uncopyable(f.Type(), seen) {
				return true
			}
		}
	case *types.Array:
		return uncopyable(u.Elem(), seen)
	}
	return false
}

// hasPointerLock reports whether *t has a Lock() method — the marker vet's
// copylocks uses for must-not-copy types (sync primitives and noCopy).
func hasPointerLock(t types.Type) bool {
	if _, isPtr := t.Underlying().(*types.Pointer); isPtr {
		return false
	}
	set := types.NewMethodSet(types.NewPointer(t))
	for i := range set.Len() {
		fn := set.At(i).Obj()
		if fn.Name() != "Lock" || !isNullary(fn) {
			continue
		}
		return true
	}
	return false
}

// isNullary reports a func with no parameters and no results — Lock()'s shape.
func isNullary(obj types.Object) bool {
	sig, ok := obj.Type().(*types.Signature)
	return ok && sig.Params().Len() == 0 && sig.Results().Len() == 0
}

// render produces the generated Go source: one sorted map of qualified type
// names to their qualifying reason.
func render(entries map[string]string) []byte {
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("// Code generated by discover.go (go run discover.go); DO NOT EDIT.\n\n")
	b.WriteString("package ptrparam\n\n")
	b.WriteString("// stdPointerParams are the standard-library types where a pointer parameter\n")
	b.WriteString("// is idiomatic: the type is uncopyable (transitively holds a lock) or exports\n")
	b.WriteString("// only pointer-receiver methods (a value is unusable). Regenerate with\n")
	b.WriteString("// `go run discover.go` after a toolchain bump.\n")
	b.WriteString("var stdPointerParams = map[string]bool{\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "\t%q: true, // %s\n", k, entries[k])
	}
	b.WriteString("}\n")
	return []byte(b.String())
}
