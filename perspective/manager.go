package perspective

import (
	"fmt"

	"scouter.client.qt/qtutil"
	"scouter.client.qt/settings"

	"github.com/mappu/miqt/qt6"
)

// ChartManagerFactory creates a new ChartManagerInterface instance with a perspective ID prefix
type ChartManagerFactory func(perspectiveID string) ChartManagerInterface

// Manager manages perspective registration, switching, and state persistence
type Manager struct {
	mainWindow         *qt6.QMainWindow
	tabBar             *qt6.QTabBar
	perspectives       []Perspective
	active             int // index into perspectives
	isSwitching        bool
	appSettings        *settings.AppSettings
	createChartManager ChartManagerFactory
	nextID             int // counter for generating unique IDs
}

// NewManager creates a perspective manager and adds a QTabBar to the toolbar
func NewManager(mainWindow *qt6.QMainWindow, appSettings *settings.AppSettings, factory ChartManagerFactory) *Manager {
	m := &Manager{
		mainWindow:         mainWindow,
		appSettings:        appSettings,
		active:             -1,
		createChartManager: factory,
		nextID:             0,
	}

	// Scan existing perspective IDs to find max ID counter
	for id := range appSettings.Perspectives {
		var n int
		if _, err := fmt.Sscanf(id, "persp_%d", &n); err == nil && n >= m.nextID {
			m.nextID = n + 1
		}
	}

	// Create toolbar for perspective tabs
	toolbar := qt6.NewQToolBar2("Perspectives")
	qtutil.SetObjectName(toolbar.QWidget.QObject, "perspectiveToolbar")
	toolbar.SetMovable(false)
	toolbar.SetFloatable(false)
	toolbar.SetContextMenuPolicy(qt6.PreventContextMenu)

	// Create tab bar
	m.tabBar = qt6.NewQTabBar2()
	m.tabBar.SetExpanding(false)
	m.tabBar.SetDocumentMode(true)
	m.tabBar.SetTabsClosable(false)
	m.tabBar.SetStyleSheet(`
		QTabBar::tab {
			padding: 4px 12px;
			margin-right: 2px;
			border: 1px solid #c0c0c0;
			border-bottom: none;
			border-top-left-radius: 4px;
			border-top-right-radius: 4px;
			background: #e8e8e8;
			color: #555;
		}
		QTabBar::tab:selected {
			background: #1976D2;
			color: white;
			font-weight: bold;
		}
		QTabBar::tab:hover:!selected {
			background: #bbdefb;
			color: #1565C0;
		}
	`)

	// Context menu on tab bar
	m.tabBar.SetContextMenuPolicy(qt6.CustomContextMenu)
	m.tabBar.OnCustomContextMenuRequested(func(pos *qt6.QPoint) {
		m.showTabContextMenu(pos)
	})

	toolbar.AddWidget(m.tabBar.QWidget)
	mainWindow.AddToolBar(qt6.TopToolBarArea, toolbar)

	// Connect tab change
	m.tabBar.OnCurrentChanged(func(index int) {
		if !m.isSwitching && index >= 0 && index < len(m.perspectives) {
			m.SetActive(m.perspectives[index].ID())
		}
	})

	return m
}

// Register adds a perspective and a corresponding tab
func (m *Manager) Register(p Perspective) {
	m.perspectives = append(m.perspectives, p)
	m.tabBar.AddTab(p.Name())
}

// SetActive switches to the perspective with the given ID
func (m *Manager) SetActive(id ID) {
	targetIdx := -1
	for i, p := range m.perspectives {
		if p.ID() == id {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 || targetIdx == m.active {
		return
	}

	m.isSwitching = true
	defer func() { m.isSwitching = false }()

	// Freeze UI to prevent flicker during switch
	m.mainWindow.SetUpdatesEnabled(false)

	// Save current perspective state before switching
	if m.active >= 0 && m.active < len(m.perspectives) {
		curr := m.perspectives[m.active]
		state := curr.SaveState(m.mainWindow)
		// Hide inactive dock names so SaveState only captures active perspective layout
		restoreNames := m.HideInactiveDocksFromRestore()
		state.WindowState = m.mainWindow.SaveState()
		restoreNames()
		if m.appSettings.Perspectives == nil {
			m.appSettings.Perspectives = make(map[string]*settings.PerspectiveState)
		}
		m.appSettings.Perspectives[string(curr.ID())] = state
		curr.Deactivate(m.mainWindow)
	}

	// Activate target
	m.active = targetIdx
	target := m.perspectives[targetIdx]
	target.Activate(m.mainWindow)

	// Ensure only active perspective's docks are visible
	m.HideInactiveDocks()

	// Restore dock layout (positions + sizes) for this perspective
	// Remove inactive docks so RestoreState only affects active perspective
	if savedState, ok := m.appSettings.Perspectives[string(id)]; ok && len(savedState.WindowState) > 0 {
		restoreInactive := m.HideInactiveDocksFromRestore()
		size := m.mainWindow.Size()
		m.mainWindow.QWidget.SetFixedSize(size)
		m.mainWindow.RestoreState(savedState.WindowState)
		m.mainWindow.QWidget.SetMinimumSize2(0, 0)
		m.mainWindow.QWidget.SetMaximumSize2(16777215, 16777215)
		restoreInactive()
	}

	// Unfreeze UI
	m.mainWindow.SetUpdatesEnabled(true)

	// Sync tab bar
	if m.tabBar.CurrentIndex() != targetIdx {
		m.tabBar.SetCurrentIndex(targetIdx)
	}

	// Update settings
	m.appSettings.ActivePerspective = string(id)
}

// SetActiveInitial sets the active perspective without calling Activate/Deactivate.
// Use this when docks have already been created and added to the main window.
func (m *Manager) SetActiveInitial(id ID) {
	targetIdx := -1
	for i, p := range m.perspectives {
		if p.ID() == id {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return
	}

	m.isSwitching = true
	m.active = targetIdx
	if m.tabBar.CurrentIndex() != targetIdx {
		m.tabBar.SetCurrentIndex(targetIdx)
	}
	m.appSettings.ActivePerspective = string(id)
	m.isSwitching = false
}

// generateID creates a unique perspective ID
func (m *Manager) generateID() ID {
	id := ID(fmt.Sprintf("persp_%d", m.nextID))
	m.nextID++
	return id
}

// AddPerspective creates a new perspective with the given name, switches to it, and returns its ID
func (m *Manager) AddPerspective(name string) ID {
	id := m.generateID()
	cm := m.createChartManager(string(id))
	p := NewServicePerspective(id, name, cm)

	m.Register(p)

	// Save perspective order
	m.updatePerspectiveOrder()

	// Switch to new perspective
	m.SetActive(id)

	// Setup default layout for new perspective
	p.SetupDefaultLayout(m.mainWindow)

	// Save state
	m.SaveCurrentState()
	m.appSettings.Save()

	return id
}

// RemovePerspective removes a perspective by ID
func (m *Manager) RemovePerspective(id ID) {
	// Don't allow removing the last perspective
	if len(m.perspectives) <= 1 {
		return
	}

	targetIdx := -1
	for i, p := range m.perspectives {
		if p.ID() == id {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return
	}

	// Confirm deletion
	result := qt6.QMessageBox_Question(
		m.mainWindow.QWidget,
		"Delete Perspective",
		fmt.Sprintf("Delete perspective \"%s\"?", m.perspectives[targetIdx].Name()),
	)
	if result != qt6.QMessageBox__Yes {
		return
	}

	m.isSwitching = true

	// If deleting the active perspective, switch to adjacent
	if targetIdx == m.active {
		newIdx := targetIdx - 1
		if newIdx < 0 {
			newIdx = targetIdx + 1
		}
		if newIdx < len(m.perspectives) && newIdx != targetIdx {
			// Activate adjacent before removing
			m.active = -1 // Reset so SetActive doesn't skip
			m.isSwitching = false
			m.SetActive(m.perspectives[newIdx].ID())
			m.isSwitching = true
			// Recalculate targetIdx after potential reorder
			for i, p := range m.perspectives {
				if p.ID() == id {
					targetIdx = i
					break
				}
			}
		}
	} else if targetIdx < m.active {
		m.active--
	}

	// Destroy perspective resources
	sp, ok := m.perspectives[targetIdx].(*ServicePerspective)
	if ok {
		sp.Destroy(m.mainWindow)
	}

	// Remove from list
	m.perspectives = append(m.perspectives[:targetIdx], m.perspectives[targetIdx+1:]...)

	// Remove tab
	m.tabBar.RemoveTab(targetIdx)

	// Remove from settings
	delete(m.appSettings.Perspectives, string(id))
	m.updatePerspectiveOrder()

	m.isSwitching = false

	m.SaveCurrentState()
	m.appSettings.Save()
}

// RenamePerspective renames a perspective by ID using an input dialog
func (m *Manager) RenamePerspective(id ID) {
	targetIdx := -1
	for i, p := range m.perspectives {
		if p.ID() == id {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return
	}

	p := m.perspectives[targetIdx]
	ok := false
	newName := qt6.QInputDialog_GetText4(
		m.mainWindow.QWidget,
		"Rename Perspective",
		"Name:",
		qt6.QLineEdit__Normal,
		p.Name(),
		&ok,
	)
	if !ok || newName == "" {
		return
	}

	// Update perspective name
	if sp, ok := p.(*ServicePerspective); ok {
		sp.SetName(newName)
	}

	// Update tab text
	m.tabBar.SetTabText(targetIdx, newName)

	// Save
	m.SaveCurrentState()
	m.appSettings.Save()
}

// showTabContextMenu displays the context menu for tab bar
func (m *Manager) showTabContextMenu(pos *qt6.QPoint) {
	tabIdx := m.tabBar.TabAt(pos)

	menu := qt6.NewQMenu2()
	defer menu.DeleteLater()

	// New Perspective
	newAction := menu.AddActionWithText("New Perspective")
	newAction.OnTriggered(func() {
		ok := false
		name := qt6.QInputDialog_GetText4(
			m.mainWindow.QWidget,
			"New Perspective",
			"Name:",
			qt6.QLineEdit__Normal,
			"",
			&ok,
		)
		if ok && name != "" {
			m.AddPerspective(name)
		}
	})

	if tabIdx >= 0 && tabIdx < len(m.perspectives) {
		p := m.perspectives[tabIdx]

		menu.AddSeparator()

		// Rename
		renameAction := menu.AddActionWithText(fmt.Sprintf("Rename \"%s\"...", p.Name()))
		renameAction.OnTriggered(func() {
			m.RenamePerspective(p.ID())
		})

		// Delete (disabled if last one)
		deleteAction := menu.AddActionWithText(fmt.Sprintf("Delete \"%s\"", p.Name()))
		deleteAction.OnTriggered(func() {
			m.RemovePerspective(p.ID())
		})
		if len(m.perspectives) <= 1 {
			deleteAction.SetEnabled(false)
		}
	}

	globalPos := m.tabBar.MapToGlobalWithQPoint(pos)
	menu.ExecWithPos(globalPos)
}

// updatePerspectiveOrder saves the current tab order to settings
func (m *Manager) updatePerspectiveOrder() {
	order := make([]string, len(m.perspectives))
	for i, p := range m.perspectives {
		order[i] = string(p.ID())
	}
	m.appSettings.PerspectiveOrder = order
}

// redistributeActiveDocks evenly distributes vertical space among active perspective's docks
func (m *Manager) redistributeActiveDocks() {
	p := m.ActivePerspective()
	if p == nil {
		return
	}
	docks := p.GetDocks()
	var visible []*qt6.QDockWidget
	for _, d := range docks {
		if d.IsVisible() && !d.IsFloating() {
			visible = append(visible, d)
		}
	}
	if len(visible) == 0 {
		return
	}
	sizes := make([]int, len(visible))
	for i := range sizes {
		sizes[i] = 200
	}
	m.mainWindow.ResizeDocks(visible, sizes, qt6.Vertical)
}

// HideInactiveDocks ensures only the active perspective's docks are visible.
// This is needed because mainWindow.RestoreState() is global and may re-show
// docks belonging to other perspectives.
func (m *Manager) HideInactiveDocks() {
	p := m.ActivePerspective()
	if p == nil {
		return
	}
	activeID := p.ID()
	for _, pp := range m.perspectives {
		if pp.ID() == activeID {
			continue
		}
		for _, d := range pp.GetDocks() {
			if d.IsVisible() {
				d.Hide()
			}
		}
	}
}

// HideInactiveDocksFromRestore temporarily renames inactive perspective docks
// so that RestoreState() won't recognize or reposition them.
// Returns a function that restores the original object names.
func (m *Manager) HideInactiveDocksFromRestore() func() {
	p := m.ActivePerspective()
	if p == nil {
		return func() {}
	}
	activeID := p.ID()
	type saved struct {
		dock *qt6.QDockWidget
		name string
	}
	var items []saved
	for _, pp := range m.perspectives {
		if pp.ID() == activeID {
			continue
		}
		for _, d := range pp.GetDocks() {
			origName := d.ObjectName()
			// Prefix with _ so RestoreState won't match this dock
			tmpName := "_hidden_" + origName
			qtutil.SetObjectName(d.QWidget.QObject, tmpName)
			items = append(items, saved{dock: d, name: origName})
		}
	}
	return func() {
		for _, item := range items {
			qtutil.SetObjectName(item.dock.QWidget.QObject, item.name)
		}
	}
}

// ActivePerspective returns the currently active perspective, or nil
func (m *Manager) ActivePerspective() Perspective {
	if m.active >= 0 && m.active < len(m.perspectives) {
		return m.perspectives[m.active]
	}
	return nil
}

// IsSwitching returns true if a perspective switch is in progress
func (m *Manager) IsSwitching() bool {
	return m.isSwitching
}

// SaveCurrentState saves the active perspective's state to settings
func (m *Manager) SaveCurrentState() {
	if m.isSwitching {
		return
	}
	p := m.ActivePerspective()
	if p == nil {
		return
	}
	state := p.SaveState(m.mainWindow)
	// Hide inactive dock names so SaveState only captures active perspective layout
	restoreNames := m.HideInactiveDocksFromRestore()
	state.WindowState = m.mainWindow.SaveState()
	restoreNames()
	if m.appSettings.Perspectives == nil {
		m.appSettings.Perspectives = make(map[string]*settings.PerspectiveState)
	}
	m.appSettings.Perspectives[string(p.ID())] = state
}

// SaveAllStates saves all perspectives' states (for close event)
func (m *Manager) SaveAllStates() {
	m.SaveCurrentState()
	m.updatePerspectiveOrder()
}

// Perspectives returns all registered perspectives
func (m *Manager) Perspectives() []Perspective {
	return m.perspectives
}

// TabBar returns the QTabBar widget
func (m *Manager) TabBar() *qt6.QTabBar {
	return m.tabBar
}
