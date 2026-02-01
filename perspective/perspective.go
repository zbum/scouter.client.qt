package perspective

import (
	"scouter.client.qt/settings"

	"github.com/mappu/miqt/qt6"
)

// ID identifies a perspective type
type ID string

const (
	ServiceID ID = "service"
)

// Perspective defines the interface for a switchable perspective
type Perspective interface {
	// ID returns the unique identifier for this perspective
	ID() ID

	// Name returns the display name for the tab
	Name() string

	// Activate makes this perspective visible. Docks are added to the main window.
	Activate(mainWindow *qt6.QMainWindow)

	// Deactivate hides this perspective. Docks are removed from the main window.
	Deactivate(mainWindow *qt6.QMainWindow)

	// SaveState returns the current state of this perspective
	SaveState(mainWindow *qt6.QMainWindow) *settings.PerspectiveState

	// RestoreState restores perspective state from saved data.
	// Returns true if window state was restored.
	RestoreState(mainWindow *qt6.QMainWindow, state *settings.PerspectiveState) bool

	// GetDocks returns all dock widgets owned by this perspective
	GetDocks() []*qt6.QDockWidget

	// SetupDefaultLayout creates and arranges default docks
	SetupDefaultLayout(mainWindow *qt6.QMainWindow)

	// CreateDocks creates dock widgets from the perspective state (without restoring layout)
	CreateDocks(mainWindow *qt6.QMainWindow, state *settings.PerspectiveState)
}
