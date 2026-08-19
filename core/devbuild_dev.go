//go:build dev

package core

// isDevBuild reports whether this binary was built by `wails dev` (the CLI
// passes the `dev` build tag; `wails build` production binaries do not carry
// it). Declared as a var rather than a const so plain `go test` runs — which
// never set the tag — can still exercise the dev branches by overriding it.
var isDevBuild = true
