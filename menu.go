package main

import (
	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/dialogs"
)

// MenuManager manages the application menu bar
type MenuManager struct {
	menuBar      *qt6.QMenuBar
	mainWindow   *qt6.QMainWindow
	chartManager *ChartManager
}

// NewMenuManager creates a new menu manager and sets up menus
func NewMenuManager(mainWindow *qt6.QMainWindow, chartManager *ChartManager) *MenuManager {
	mm := &MenuManager{
		menuBar:      qt6.NewQMenuBar2(),
		mainWindow:   mainWindow,
		chartManager: chartManager,
	}

	mainWindow.SetMenuBar(mm.menuBar)
	mm.setupServerMenu()
	mm.setupChartMenu()
	mm.setupHelpMenu()

	return mm
}

// setupServerMenu creates the Server menu
func (mm *MenuManager) setupServerMenu() {
	serverMenu := mm.menuBar.AddMenuWithTitle("Server")

	// Server Manager action
	manageAction := serverMenu.AddActionWithText("Server Manager...")
	manageAction.OnTriggered(func() {
		dlg := dialogs.NewServerListDialog(mm.mainWindow.QWidget)
		dlg.Exec()
	})

	serverMenu.AddSeparator()

	// Add Server action
	addAction := serverMenu.AddActionWithText("Add Server...")
	addAction.OnTriggered(func() {
		dlg := dialogs.NewServerDialog(mm.mainWindow.QWidget)
		dlg.Exec()
	})
}

// setupChartMenu creates the Chart menu
func (mm *MenuManager) setupChartMenu() {
	chartMenu := mm.menuBar.AddMenuWithTitle("View")

	// Add Chart action
	addChartAction := chartMenu.AddActionWithText("Add")
	addChartAction.OnTriggered(func() {
		mm.chartManager.AddChart()
	})

	// Remove Last Chart action
	removeChartAction := chartMenu.AddActionWithText("Remove Last")
	removeChartAction.OnTriggered(func() {
		mm.chartManager.RemoveLastChart()
	})
}

// setupHelpMenu creates the Help menu
func (mm *MenuManager) setupHelpMenu() {
	helpMenu := mm.menuBar.AddMenuWithTitle("Help")

	aboutAction := helpMenu.AddActionWithText("About Scouter")
	aboutAction.OnTriggered(func() {
		dialogs.ShowAboutDialog(mm.mainWindow.QWidget)
	})
}