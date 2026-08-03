package gui

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// TestNewMainWindow builds the window against Fyne's headless test app, which
// needs no display and so runs on a CI runner. Run itself cannot be tested this
// way: it blocks on the event loop until a user closes the window.
func TestNewMainWindow(t *testing.T) {
	w := newMainWindow(test.NewApp())

	if got := w.Title(); got != windowTitle {
		t.Errorf("window title = %q, want %q", got, windowTitle)
	}

	if w.Content() == nil {
		t.Error("window has no content; it should have an empty container, not nil")
	}

	if size := w.Canvas().Size(); size.Width <= 0 || size.Height <= 0 {
		t.Errorf("canvas size = %v, want both dimensions positive", size)
	}
}
