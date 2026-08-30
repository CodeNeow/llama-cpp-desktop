//go:build !android

package core

// Off-Android stubs: the Android branches only run behind pathsGOOS ==
// "android", which production never selects on other platforms. Keeping the
// symbols defined lets the desktop test binary drive those branches through
// the pathsAndroidFilesDir / pathsAndroidModelsBase seams.

func androidFilesDir() string   { return "" }
func androidModelsBase() string { return "" }
