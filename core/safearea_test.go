package core

import "testing"

// SafeArea must degrade to all-zero insets off-Android: the frontend composes
// max(env(), binding) for its safe-area padding, so a zero result has to be a
// clean no-op — never an error, negative value or partial garbage.
func TestSafeAreaNonAndroidZero(t *testing.T) {
	area := currentSafeArea()
	if area != (SafeArea{}) {
		t.Fatalf("expected zero insets on non-Android platforms, got %+v", area)
	}
}
