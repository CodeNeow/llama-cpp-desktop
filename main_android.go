//go:build android

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// Android entry hook (Wails v3 Android target). Android builds compile the Go
// code in c-shared mode (-buildmode=c-shared) and the Go runtime never calls
// main(): instead the Java WailsBridge invokes the JNI nativeInit export,
// which starts whatever function was registered here in a goroutine
// (v3/pkg/application/application_android.go — RegisterAndroidMain, and
// Java_com_wails_app_WailsBridge_nativeInit; same pattern as the upstream
// v3/examples/android/main_android.go). Without this registration the host
// logs "No main function registered" and the app never starts.
func init() {
	application.RegisterAndroidMain(main)
}
