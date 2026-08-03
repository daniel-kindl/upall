package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
)

// appID identifies the application to the desktop environment, and is what Fyne
// keys stored preferences on. It is reverse-DNS by convention, and changing it
// would orphan whatever a previous version had saved.
const appID = "io.github.daniel-kindl.upall"

// windowTitle is what the window manager and taskbar show.
const windowTitle = "upall"

// Default window size in device-independent pixels. Large enough for a provider
// list beside a plan, which is what M11 puts here.
const (
	defaultWidth  = 900
	defaultHeight = 640
)

// Run opens the upall window and blocks until the user closes it.
//
// It must be called from the goroutine that runs main. Fyne, like every desktop
// toolkit, requires its event loop to own the main thread, and no pipeline work
// may run on it.
func Run() {
	newMainWindow(app.NewWithID(appID)).ShowAndRun()
}

// newMainWindow builds the main window without showing it, which is what makes
// it testable: a test can construct one against Fyne's headless test app and
// inspect it, where [Run] would block on an event loop and need a display.
func newMainWindow(a fyne.App) fyne.Window {
	w := a.NewWindow(windowTitle)
	w.Resize(fyne.NewSize(defaultWidth, defaultHeight))

	// Deliberately empty. The provider list and plan view arrive at M11, and a
	// placeholder saying so would only have to be deleted then.
	w.SetContent(container.NewVBox())

	return w
}
