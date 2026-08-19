//go:build !dev

package core

// isDevBuild is false for production builds (`wails build`, `go build`).
var isDevBuild = false
