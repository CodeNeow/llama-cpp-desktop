//go:build android

package core

import (
	"encoding/json"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// SafeArea holds the system-bar insets in physical pixels: the status-bar /
// display-cutout band at the top, the gesture- or button-navigation band at
// the bottom, and the side cutouts. Populated on Android only; every other
// platform reports zeros (see safearea_other.go).
type SafeArea struct {
	Top    int `json:"top"`
	Bottom int `json:"bottom"`
	Left   int `json:"left"`
	Right  int `json:"right"`
}

// currentSafeArea reads the system-bar insets from the Wails Android runtime:
// application.Android.SafeAreaJSON() forwards to WailsBridge.getSafeAreaJson,
// which answers {"top","bottom","left","right"} in physical px. Any surprise —
// empty or malformed payload, wrong-typed fields — degrades to a zero inset
// for that side, so a bridge hiccup can never block startup or pad the UI
// with garbage.
func currentSafeArea() SafeArea {
	var fields map[string]float64
	if err := json.Unmarshal([]byte(application.Android.SafeAreaJSON()), &fields); err != nil {
		return SafeArea{}
	}
	return SafeArea{
		Top:    clampPx(fields["top"]),
		Bottom: clampPx(fields["bottom"]),
		Left:   clampPx(fields["left"]),
		Right:  clampPx(fields["right"]),
	}
}

// clampPx coerces one inset field to a non-negative int; missing keys read as
// 0 (Go map zero value).
func clampPx(v float64) int {
	if v < 0 {
		return 0
	}
	return int(v)
}
