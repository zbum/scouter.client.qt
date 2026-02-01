package main

import (
	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/dialogs"
	"scouter.client.qt/perspective"
)

// MenuManager manages the application menu bar
type MenuManager struct {
	menuBar    *qt6.QMenuBar
	mainWindow *qt6.QMainWindow
	perspMgr   *perspective.Manager
}

// NewMenuManager creates a new menu manager and sets up menus
func NewMenuManager(mainWindow *qt6.QMainWindow, perspMgr *perspective.Manager) *MenuManager {
	mm := &MenuManager{
		menuBar:    qt6.NewQMenuBar2(),
		mainWindow: mainWindow,
		perspMgr:   perspMgr,
	}

	mainWindow.SetMenuBar(mm.menuBar)
	mm.setupServerMenu()
	mm.setupChartMenu()
	mm.setupWindowMenu()
	mm.setupHelpMenu()

	return mm
}

// getActiveChartManager returns the active perspective's ChartManager
func (mm *MenuManager) getActiveChartManager() *ChartManager {
	p := mm.perspMgr.ActivePerspective()
	if p == nil {
		return nil
	}
	if sp, ok := p.(*perspective.ServicePerspective); ok {
		if cm, ok := sp.ChartManager().(*ChartManager); ok {
			return cm
		}
	}
	return nil
}

// setupServerMenu creates the Server menu
func (mm *MenuManager) setupServerMenu() {
	serverMenu := mm.menuBar.AddMenuWithTitle("Server")

	manageAction := serverMenu.AddActionWithText("Server Manager...")
	manageAction.OnTriggered(func() {
		dlg := dialogs.NewServerListDialog(mm.mainWindow.QWidget)
		dlg.Exec()
	})

	serverMenu.AddSeparator()

	addAction := serverMenu.AddActionWithText("Add Server...")
	addAction.OnTriggered(func() {
		dlg := dialogs.NewServerDialog(mm.mainWindow.QWidget)
		dlg.Exec()
	})
}

// setupChartMenu creates the View menu
func (mm *MenuManager) setupChartMenu() {
	_ = mm.menuBar.AddMenuWithTitle("View")
}

// setupWindowMenu creates the Window menu with perspective management
func (mm *MenuManager) setupWindowMenu() {
	windowMenu := mm.menuBar.AddMenuWithTitle("Window")

	newPerspAction := windowMenu.AddActionWithText("New Perspective...")
	newPerspAction.OnTriggered(func() {
		ok := false
		name := qt6.QInputDialog_GetText4(
			mm.mainWindow.QWidget,
			"New Perspective",
			"Name:",
			qt6.QLineEdit__Normal,
			"",
			&ok,
		)
		if ok && name != "" {
			mm.perspMgr.AddPerspective(name)
		}
	})

	windowMenu.AddSeparator()

	for _, p := range mm.perspMgr.Perspectives() {
		pID := p.ID()
		action := windowMenu.AddActionWithText(p.Name())
		action.OnTriggered(func() {
			mm.perspMgr.SetActive(pID)
		})
	}
}

// setupHelpMenu creates the Help menu
func (mm *MenuManager) setupHelpMenu() {
	helpMenu := mm.menuBar.AddMenuWithTitle("Help")

	aboutAction := helpMenu.AddActionWithText("About Scouter")
	aboutAction.OnTriggered(func() {
		dialogs.ShowAboutDialog(mm.mainWindow.QWidget)
	})
}
