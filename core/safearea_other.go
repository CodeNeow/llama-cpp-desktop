//go:build !android

package core

// SafeArea holds the system-bar insets in physical pixels: the status-bar /
// display-cutout band at the top, the gesture- or button-navigation band at
// the bottom, and the side cutouts. Populated on Android only (see
// safearea_android.go); every other platform reports zeros.
type SafeArea struct {
	Top    int `json:"top"`
	Bottom int `json:"bottom"`
	Left   int `json:"left"`
	Right  int `json:"right"`
}

// currentSafeArea returns zero insets on non-Android platforms: desktop
// windows are not edge-to-edge, so the frontend's CSS env(safe-area-inset-*)
// base layer stays the only safe-area source there and the binding must be a
// no-op.
func currentSafeArea() SafeArea {
	return SafeArea{}
}
