//go:build android && !cgo

package core

// Non-cgo Android builds (wails3 generate bindings and similar tooling) never
// run this code: the stubs keep the package linkable while leaving the paths
// unresolved (bare cwd-relative names). Real Android apps always build with
// cgo and get the JNI implementation in androidpath_android.go.

func androidFilesDir() string   { return "" }
func androidModelsBase() string { return "" }
