package perspective

import (
	"scouter.client.qt/settings"

	"github.com/mappu/miqt/qt6"
)

// ChartManagerInterface defines what ServicePerspective needs from ChartManager
type ChartManagerInterface interface {
	SavePerspectiveState() *settings.PerspectiveState
	RestorePerspectiveState(ps *settings.PerspectiveState) bool
	GetAllDocks() []*qt6.QDockWidget
	GetDocks() []*qt6.QDockWidget
	ShowAllDocks()
	HideAllDocks()
	AddDefaultCharts()
	Destroy()
}

// ServicePerspective wraps ChartManager as a perspective
type ServicePerspective struct {
	id           ID
	name         string
	chartManager ChartManagerInterface
}

// NewServicePerspective creates a new service perspective with configurable ID and name
func NewServicePerspective(id ID, name string, cm ChartManagerInterface) *ServicePerspective {
	return &ServicePerspective{
		id:           id,
		name:         name,
		chartManager: cm,
	}
}

func (sp *ServicePerspective) ID() ID {
	return sp.id
}

func (sp *ServicePerspective) Name() string {
	return sp.name
}

// SetName updates the perspective's display name
func (sp *ServicePerspective) SetName(name string) {
	sp.name = name
}

func (sp *ServicePerspective) Activate(mainWindow *qt6.QMainWindow) {
	for _, d := range sp.chartManager.GetAllDocks() {
		// If dock has no parent, it was newly created - add to mainWindow
		if d.Parent() == nil {
			mainWindow.AddDockWidget(qt6.RightDockWidgetArea, d)
		}
		d.Show()
	}
}

func (sp *ServicePerspective) Deactivate(mainWindow *qt6.QMainWindow) {
	for _, d := range sp.chartManager.GetAllDocks() {
		d.Hide()
	}
}

func (sp *ServicePerspective) SaveState(mainWindow *qt6.QMainWindow) *settings.PerspectiveState {
	ps := sp.chartManager.SavePerspectiveState()
	ps.Name = sp.name
	return ps
}

func (sp *ServicePerspective) RestoreState(mainWindow *qt6.QMainWindow, state *settings.PerspectiveState) bool {
	return sp.chartManager.RestorePerspectiveState(state)
}

func (sp *ServicePerspective) GetDocks() []*qt6.QDockWidget {
	return sp.chartManager.GetAllDocks()
}

func (sp *ServicePerspective) SetupDefaultLayout(mainWindow *qt6.QMainWindow) {
	sp.chartManager.AddDefaultCharts()
}

func (sp *ServicePerspective) CreateDocks(mainWindow *qt6.QMainWindow, state *settings.PerspectiveState) {
	sp.chartManager.RestorePerspectiveState(state)
}

// ChartManager returns the underlying chart manager interface
func (sp *ServicePerspective) ChartManager() ChartManagerInterface {
	return sp.chartManager
}

// Destroy cleans up the perspective's resources
func (sp *ServicePerspective) Destroy(mainWindow *qt6.QMainWindow) {
	// Fully remove docks from mainWindow before destroying
	for _, d := range sp.chartManager.GetAllDocks() {
		d.Hide()
		mainWindow.RemoveDockWidget(d)
	}
	sp.chartManager.Destroy()
}
