package main

import (
	"github.com/mappu/miqt/qt6"
)

// MenuManager manages the application menu bar
type MenuManager struct {
	menuBar      *qt6.QMenuBar
	chartManager *ChartManager
}

// NewMenuManager creates a new menu manager and sets up menus
func NewMenuManager(mainWindow *qt6.QMainWindow, chartManager *ChartManager) *MenuManager {
	mm := &MenuManager{
		menuBar:      qt6.NewQMenuBar2(),
		chartManager: chartManager,
	}

	mainWindow.SetMenuBar(mm.menuBar)
	mm.setupChartMenu()

	return mm
}

// setupChartMenu creates the Chart menu
func (mm *MenuManager) setupChartMenu() {
	chartMenu := mm.menuBar.AddMenuWithTitle("Chart")

	// Add Chart action
	addChartAction := chartMenu.AddActionWithText("Add Chart")
	addChartAction.OnTriggered(func() {
		mm.chartManager.AddChart()
	})

	// Remove Last Chart action
	removeChartAction := chartMenu.AddActionWithText("Remove Last Chart")
	removeChartAction.OnTriggered(func() {
		mm.chartManager.RemoveLastChart()
	})
}