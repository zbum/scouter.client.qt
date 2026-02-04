package main

//go:debug asyncpreemptoff=1

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"scouter.client.qt/assets"
	"scouter.client.qt/chart"
	"scouter.client.qt/groupnav"
	"scouter.client.qt/model"
	"scouter.client.qt/perspective"
	"scouter.client.qt/qtutil"
	"scouter.client.qt/server"
	"scouter.client.qt/settings"
	"scouter.client.qt/views"
	"scouter.client.qt/xlog"

	"github.com/mappu/miqt/qt6"
)

const appName = "scouter.client.go"

// ChartManager manages multiple chart dock widgets
type ChartManager struct {
	mainWindow         *qt6.QMainWindow
	charts             []*ChartDock
	groupCharts        []*views.GroupCounterView
	todayViews         []*views.GroupCounterTodayView
	pastViews          []*views.GroupCounterPastView
	xlogViews          []*xlog.View
	eqViews            []*views.GroupEQView
	activeServiceViews []*views.ActiveServiceView
	chartCount         int
	groupChartCount    int
	todayViewCount     int
	pastViewCount      int
	xlogCount          int
	eqCount            int
	timer              *qt6.QTimer
	onStateChanged     func() // Callback when dock state changes
	isRestoring        bool   // Flag to prevent saving during restore
	appSettings        *settings.AppSettings
	dockPrefix         string // Unique prefix for dock object names per perspective
}

// ChartDock holds a chart widget and its dock
type ChartDock struct {
	dock           *qt6.QDockWidget
	chart          *chart.Widget
	id             int
	counterName    string // Counter name for real data (empty = demo mode)
	subscriptionID int    // CounterEngine subscription ID
}

func NewChartManager(mainWindow *qt6.QMainWindow, appSettings *settings.AppSettings, dockPrefix string) *ChartManager {
	cm := &ChartManager{
		mainWindow:  mainWindow,
		chartCount:  0,
		appSettings: appSettings,
		dockPrefix:  dockPrefix,
	}

	// Timer for updating charts in demo mode
	cm.timer = qt6.NewQTimer()
	cm.timer.OnTimeout(func() {
		cm.updateCharts()
	})
	cm.timer.Start(1000)

	return cm
}

// updateCharts updates all charts with data from subscriptions
func (cm *ChartManager) updateCharts() {
	// Real data comes from CounterEngine subscriptions
	// This method is kept for future use if needed
}

// SetChartCounter sets the counter name for a chart and subscribes to real data
func (cm *ChartManager) SetChartCounter(cd *ChartDock, counterName string) {
	// Unsubscribe from previous counter
	if cd.subscriptionID != 0 {
		model.GetCounterEngine().Unsubscribe(cd.subscriptionID)
		cd.subscriptionID = 0
	}

	cd.counterName = counterName

	if counterName == "" {
		return
	}

	// Subscribe to counter updates
	cd.subscriptionID = model.GetCounterEngine().Subscribe(counterName, 0, func(objHash int32, counter string, value float64, timestamp time.Time) {
		// Update chart on main thread
		cd.chart.AddPoint(int64(value))
	})
}

func (cm *ChartManager) AddChart() *ChartDock {
	return cm.AddChartWithTitle("")
}

func (cm *ChartManager) RemoveChart(cd *ChartDock) {
	for i, c := range cm.charts {
		if c == cd {
			cm.charts = append(cm.charts[:i], cm.charts[i+1:]...)
			break
		}
	}
	cm.mainWindow.RemoveDockWidget(cd.dock)
	cd.dock.DeleteLater()
	cm.notifyStateChanged()
}

func (cm *ChartManager) RemoveLastChart() {
	if len(cm.charts) > 0 {
		cm.RemoveChart(cm.charts[len(cm.charts)-1])
	}
}

func (cm *ChartManager) ChartCount() int {
	return len(cm.charts)
}

// AddChartWithTitle adds a chart with a specific title (auto-assigns ID)
func (cm *ChartManager) AddChartWithTitle(title string) *ChartDock {
	cm.chartCount++
	return cm.addChartWithID(cm.chartCount, title)
}

// addChartWithID adds a chart with a specific ID and title
func (cm *ChartManager) addChartWithID(id int, title string) *ChartDock {
	if title == "" {
		title = fmt.Sprintf("%d", id)
	}

	// Create dock widget
	dock := qt6.NewQDockWidget2(title)

	// Set object name for Qt state save/restore
	objName := fmt.Sprintf("%s_chartDock%d", cm.dockPrefix, id)
	qtutil.SetObjectName(dock.QWidget.QObject, objName)
	dock.SetAllowedAreas(qt6.AllDockWidgetAreas)

	// Create chart widget
	chartWidget := chart.New(nil)
	chartWidget.SetTitle(title)
	dock.SetWidget(chartWidget.QWidget())

	cd := &ChartDock{
		dock:  dock,
		chart: chartWidget,
		id:    id,
	}

	// Connect dock change signals to save state
	dock.OnDockLocationChanged(func(area qt6.DockWidgetArea) {
		cm.notifyStateChanged()
	})
	dock.OnTopLevelChanged(func(topLevel bool) {
		cm.notifyStateChanged()
	})
	dock.OnVisibilityChanged(func(visible bool) {
		cm.notifyStateChanged()
	})

	// Add to main window
	cm.mainWindow.AddDockWidget(qt6.RightDockWidgetArea, dock)
	dock.Show() // Ensure dock is visible

	cm.charts = append(cm.charts, cd)

	// Track max ID for future allocations
	if id > cm.chartCount {
		cm.chartCount = id
	}

	// Redistribute dock heights so new dock gets fair space
	cm.redistributeDockHeights()

	// Notify state changed (new dock added)
	cm.notifyStateChanged()

	return cd
}

// AddGroupChart creates and tracks a group counter view
func (cm *ChartManager) AddGroupChart(groupName, objType, counterName, displayName string) *views.GroupCounterView {
	cm.groupChartCount++
	return cm.addGroupChartWithID(cm.groupChartCount, groupName, objType, counterName, displayName)
}

// addGroupChartWithMode creates a group chart with a specific view mode
func (cm *ChartManager) addGroupChartWithMode(groupName, objType, counterName, displayName string, viewMode string) *views.GroupCounterView {
	cm.groupChartCount++
	return cm.addGroupChartWithIDAndMode(cm.groupChartCount, groupName, objType, counterName, displayName, viewMode)
}

func (cm *ChartManager) addGroupChartWithIDAndMode(id int, groupName, objType, counterName, displayName string, viewMode string) *views.GroupCounterView {
	gcv := views.NewGroupCounterViewWithMode(cm.mainWindow, id, groupName, objType, counterName, displayName, viewMode)
	cm.groupCharts = append(cm.groupCharts, gcv)
	if id > cm.groupChartCount {
		cm.groupChartCount = id
	}

	dock := gcv.Dock()
	cm.setDockObjectName(dock, fmt.Sprintf("%s_groupCounterDock_%d_%s_%s_%s", cm.dockPrefix, id, groupName, counterName, viewMode))
	dock.OnDockLocationChanged(func(area qt6.DockWidgetArea) {
		cm.notifyStateChanged()
	})
	dock.OnTopLevelChanged(func(topLevel bool) {
		cm.notifyStateChanged()
	})
	dock.OnVisibilityChanged(func(visible bool) {
		cm.notifyStateChanged()
	})

	cm.redistributeDockHeights()
	cm.notifyStateChanged()
	return gcv
}

func (cm *ChartManager) addGroupChartWithID(id int, groupName, objType, counterName, displayName string) *views.GroupCounterView {
	gcv := views.NewGroupCounterViewWithID(cm.mainWindow, id, groupName, objType, counterName, displayName)
	cm.groupCharts = append(cm.groupCharts, gcv)
	if id > cm.groupChartCount {
		cm.groupChartCount = id
	}

	// Override dock object name with perspective prefix
	dock := gcv.Dock()
	cm.setDockObjectName(dock, fmt.Sprintf("%s_groupCounterDock_%d_%s_%s", cm.dockPrefix, id, groupName, counterName))
	dock.OnDockLocationChanged(func(area qt6.DockWidgetArea) {
		cm.notifyStateChanged()
	})
	dock.OnTopLevelChanged(func(topLevel bool) {
		cm.notifyStateChanged()
	})
	dock.OnVisibilityChanged(func(visible bool) {
		cm.notifyStateChanged()
	})

	cm.redistributeDockHeights()
	cm.notifyStateChanged()
	return gcv
}

// AddTodayView creates and tracks a group counter today view
func (cm *ChartManager) AddTodayView(groupName, objType, counterName, displayName string, viewMode string) *views.GroupCounterTodayView {
	cm.todayViewCount++
	return cm.addTodayViewWithID(cm.todayViewCount, groupName, objType, counterName, displayName, viewMode)
}

func (cm *ChartManager) addTodayViewWithID(id int, groupName, objType, counterName, displayName string, viewMode string) *views.GroupCounterTodayView {
	tv := views.NewGroupCounterTodayView(cm.mainWindow, id, groupName, objType, counterName, displayName, viewMode)
	cm.todayViews = append(cm.todayViews, tv)
	if id > cm.todayViewCount {
		cm.todayViewCount = id
	}

	dock := tv.Dock()
	cm.setDockObjectName(dock, fmt.Sprintf("%s_todayDock_%d_%s_%s_%s", cm.dockPrefix, id, groupName, counterName, viewMode))
	dock.OnDockLocationChanged(func(area qt6.DockWidgetArea) {
		cm.notifyStateChanged()
	})
	dock.OnTopLevelChanged(func(topLevel bool) {
		cm.notifyStateChanged()
	})
	dock.OnVisibilityChanged(func(visible bool) {
		cm.notifyStateChanged()
	})

	cm.redistributeDockHeights()
	cm.notifyStateChanged()
	return tv
}

// AddPastView creates and tracks a group counter past view
func (cm *ChartManager) AddPastView(groupName, objType, counterName, displayName string, viewMode string) *views.GroupCounterPastView {
	cm.pastViewCount++
	return cm.addPastViewWithID(cm.pastViewCount, groupName, objType, counterName, displayName, viewMode)
}

func (cm *ChartManager) addPastViewWithID(id int, groupName, objType, counterName, displayName string, viewMode string) *views.GroupCounterPastView {
	pv := views.NewGroupCounterPastView(cm.mainWindow, id, groupName, objType, counterName, displayName, viewMode)
	cm.pastViews = append(cm.pastViews, pv)
	if id > cm.pastViewCount {
		cm.pastViewCount = id
	}

	dock := pv.Dock()
	cm.setDockObjectName(dock, fmt.Sprintf("%s_pastDock_%d_%s_%s_%s", cm.dockPrefix, id, groupName, counterName, viewMode))
	dock.OnDockLocationChanged(func(area qt6.DockWidgetArea) {
		cm.notifyStateChanged()
	})
	dock.OnTopLevelChanged(func(topLevel bool) {
		cm.notifyStateChanged()
	})
	dock.OnVisibilityChanged(func(visible bool) {
		cm.notifyStateChanged()
	})

	cm.redistributeDockHeights()
	cm.notifyStateChanged()
	return pv
}

// AddXLogView creates and tracks a group XLog view
func (cm *ChartManager) AddXLogView(groupName, objType string) *xlog.View {
	cm.xlogCount++
	return cm.addXLogViewWithID(cm.xlogCount, groupName, objType)
}

func (cm *ChartManager) addXLogViewWithID(id int, groupName, objType string) *xlog.View {
	xv := xlog.NewGroupXLogViewWithID(cm.mainWindow, id, groupName, objType)
	cm.xlogViews = append(cm.xlogViews, xv)
	if id > cm.xlogCount {
		cm.xlogCount = id
	}

	dock := xv.Dock()
	cm.setDockObjectName(dock, fmt.Sprintf("%s_xlogDock_%d_%s", cm.dockPrefix, id, groupName))
	dock.OnDockLocationChanged(func(area qt6.DockWidgetArea) {
		cm.notifyStateChanged()
	})
	dock.OnTopLevelChanged(func(topLevel bool) {
		cm.notifyStateChanged()
	})
	dock.OnVisibilityChanged(func(visible bool) {
		cm.notifyStateChanged()
	})

	cm.redistributeDockHeights()
	cm.notifyStateChanged()
	return xv
}

// AddEQView creates and tracks a group EQ view
func (cm *ChartManager) AddEQView(groupName, objType string) *views.GroupEQView {
	cm.eqCount++
	return cm.addEQViewWithID(cm.eqCount, groupName, objType)
}

func (cm *ChartManager) addEQViewWithID(id int, groupName, objType string) *views.GroupEQView {
	ev := views.NewGroupEQViewWithID(cm.mainWindow, id, groupName, objType)
	cm.eqViews = append(cm.eqViews, ev)
	if id > cm.eqCount {
		cm.eqCount = id
	}

	// Double-click an agent bar → open Active Service List for that agent
	ev.SetOnAgentDoubleClick(func(objHash int32, objType string) {
		cm.openActiveServiceForAgent(objHash, objType)
	})

	dock := ev.Dock()
	cm.setDockObjectName(dock, fmt.Sprintf("%s_eqDock_%d_%s", cm.dockPrefix, id, groupName))
	dock.OnDockLocationChanged(func(area qt6.DockWidgetArea) {
		cm.notifyStateChanged()
	})
	dock.OnTopLevelChanged(func(topLevel bool) {
		cm.notifyStateChanged()
	})
	dock.OnVisibilityChanged(func(visible bool) {
		cm.notifyStateChanged()
	})

	cm.redistributeDockHeights()
	cm.notifyStateChanged()
	return ev
}

// openActiveServiceForAgent opens an Active Service List dock for a specific agent
func (cm *ChartManager) openActiveServiceForAgent(objHash int32, objType string) {
	// If a view for this agent already exists, just show/raise it
	for _, asv := range cm.activeServiceViews {
		if asv.ObjHash() == objHash {
			asv.Dock().Show()
			asv.Dock().Raise()
			return
		}
	}

	// Find the server ID for this agent
	serverId := 0
	servers := server.GetManager().GetConnectedServers()
	for _, srv := range servers {
		serverId = srv.ID
		break
	}

	asv := views.NewActiveServiceViewForAgent(cm.mainWindow, objHash, objType, serverId)
	cm.activeServiceViews = append(cm.activeServiceViews, asv)
}

// AddDefaultCharts is a no-op. New perspectives start empty;
// users add charts from the group navigation panel.
func (cm *ChartManager) AddDefaultCharts() {
}

// GetDocks returns all chart dock widgets
func (cm *ChartManager) GetDocks() []*qt6.QDockWidget {
	docks := make([]*qt6.QDockWidget, len(cm.charts))
	for i, cd := range cm.charts {
		docks[i] = cd.dock
	}
	return docks
}

// SetOnStateChanged sets callback for dock state changes
func (cm *ChartManager) SetOnStateChanged(callback func()) {
	cm.onStateChanged = callback
}

// notifyStateChanged calls the state changed callback if set
func (cm *ChartManager) notifyStateChanged() {
	if cm.isRestoring {
		return // Don't save during restore
	}
	if cm.onStateChanged != nil {
		cm.onStateChanged()
	}
}

// setDockObjectName overrides a dock's object name with a prefixed name
func (cm *ChartManager) setDockObjectName(dock *qt6.QDockWidget, name string) {
	qtutil.SetObjectName(dock.QWidget.QObject, name)
}

// SavePerspectiveState returns the current state as a PerspectiveState
// Note: WindowState is NOT saved here - it's managed by perspective.Manager
// because mainWindow.SaveState() is global and includes all docks.
func (cm *ChartManager) SavePerspectiveState() *settings.PerspectiveState {
	ps := &settings.PerspectiveState{}

	// Standalone charts are no longer used (group charts replace them)
	// Keep empty for backward compatibility
	ps.Charts = nil

	// Save group counter charts
	var groupConfigs []settings.GroupChartConfig
	for _, gcv := range cm.groupCharts {
		groupConfigs = append(groupConfigs, settings.GroupChartConfig{
			ID:          gcv.ID(),
			GroupName:   gcv.GroupName(),
			ObjType:     gcv.ObjType(),
			CounterName: gcv.CounterName(),
			DisplayName: gcv.CounterDisplay(),
			ViewMode:    gcv.ViewMode(),
		})
	}
	ps.GroupCharts = groupConfigs

	// Save today views
	var todayConfigs []settings.GroupChartConfig
	for _, tv := range cm.todayViews {
		todayConfigs = append(todayConfigs, settings.GroupChartConfig{
			ID:          tv.ID(),
			GroupName:   tv.GroupName(),
			ObjType:     tv.ObjType(),
			CounterName: tv.CounterName(),
			DisplayName: tv.CounterDisplay(),
			ViewMode:    tv.ViewMode(),
		})
	}
	ps.TodayViews = todayConfigs

	// Save past views
	var pastConfigs []settings.GroupChartConfig
	for _, pv := range cm.pastViews {
		pastConfigs = append(pastConfigs, settings.GroupChartConfig{
			ID:          pv.ID(),
			GroupName:   pv.GroupName(),
			ObjType:     pv.ObjType(),
			CounterName: pv.CounterName(),
			DisplayName: pv.CounterDisplay(),
			ViewMode:    pv.ViewMode(),
		})
	}
	ps.PastViews = pastConfigs

	// Save XLog views
	var xlogConfigs []settings.XLogViewConfig
	for _, xv := range cm.xlogViews {
		xlogConfigs = append(xlogConfigs, settings.XLogViewConfig{
			ID:        xv.ID(),
			GroupName: xv.GroupName(),
			ObjType:   xv.ObjType(),
		})
	}
	ps.XLogViews = xlogConfigs

	// Save EQ views
	var eqConfigs []settings.EQViewConfig
	for _, ev := range cm.eqViews {
		eqConfigs = append(eqConfigs, settings.EQViewConfig{
			ID:        ev.ID(),
			GroupName: ev.GroupName(),
			ObjType:   ev.ObjType(),
		})
	}
	ps.EQViews = eqConfigs

	return ps
}

// RestorePerspectiveState creates docks from a PerspectiveState (does NOT restore WindowState)
func (cm *ChartManager) RestorePerspectiveState(ps *settings.PerspectiveState) bool {
	if ps == nil {
		return false
	}

	cm.isRestoring = true
	defer func() { cm.isRestoring = false }()

	// Standalone charts (ps.Charts) are no longer restored - they were empty placeholder charts.
	// Only group charts, xlog views, and eq views are restored.

	// Restore group counter charts
	for _, gc := range ps.GroupCharts {
		if gc.ViewMode != "" && gc.ViewMode != "live-time-all" {
			cm.addGroupChartWithIDAndMode(gc.ID, gc.GroupName, gc.ObjType, gc.CounterName, gc.DisplayName, gc.ViewMode)
		} else {
			cm.addGroupChartWithID(gc.ID, gc.GroupName, gc.ObjType, gc.CounterName, gc.DisplayName)
		}
	}

	// Restore today views
	for _, tc := range ps.TodayViews {
		cm.addTodayViewWithID(tc.ID, tc.GroupName, tc.ObjType, tc.CounterName, tc.DisplayName, tc.ViewMode)
	}

	// Restore past views
	for _, pc := range ps.PastViews {
		cm.addPastViewWithID(pc.ID, pc.GroupName, pc.ObjType, pc.CounterName, pc.DisplayName, pc.ViewMode)
	}

	// Restore XLog views
	for _, xc := range ps.XLogViews {
		cm.addXLogViewWithID(xc.ID, xc.GroupName, xc.ObjType)
	}

	// Restore EQ views
	for _, ec := range ps.EQViews {
		cm.addEQViewWithID(ec.ID, ec.GroupName, ec.ObjType)
	}

	return len(ps.GroupCharts) > 0 || len(ps.TodayViews) > 0 || len(ps.PastViews) > 0 || len(ps.XLogViews) > 0 || len(ps.EQViews) > 0
}

// GetAllDocks returns all dock widgets (charts, group charts, today, past, xlogs, eqs)
func (cm *ChartManager) GetAllDocks() []*qt6.QDockWidget {
	var docks []*qt6.QDockWidget
	for _, cd := range cm.charts {
		docks = append(docks, cd.dock)
	}
	for _, gcv := range cm.groupCharts {
		docks = append(docks, gcv.Dock())
	}
	for _, tv := range cm.todayViews {
		docks = append(docks, tv.Dock())
	}
	for _, pv := range cm.pastViews {
		docks = append(docks, pv.Dock())
	}
	for _, xv := range cm.xlogViews {
		docks = append(docks, xv.Dock())
	}
	for _, ev := range cm.eqViews {
		docks = append(docks, ev.Dock())
	}
	return docks
}

// redistributeDockHeights evenly distributes vertical space among all visible docks
func (cm *ChartManager) redistributeDockHeights() {
	docks := cm.GetAllDocks()
	var visible []*qt6.QDockWidget
	for _, d := range docks {
		if d.IsVisible() && !d.IsFloating() {
			visible = append(visible, d)
		}
	}
	if len(visible) == 0 {
		return
	}
	// Equal height distribution
	sizes := make([]int, len(visible))
	for i := range sizes {
		sizes[i] = 200
	}
	cm.mainWindow.ResizeDocks(visible, sizes, qt6.Vertical)
}

// ShowAllDocks shows all dock widgets
func (cm *ChartManager) ShowAllDocks() {
	for _, d := range cm.GetAllDocks() {
		d.Show()
	}
}

// HideAllDocks hides all dock widgets
func (cm *ChartManager) HideAllDocks() {
	for _, d := range cm.GetAllDocks() {
		d.Hide()
	}
}

// Destroy cleans up all docks and stops the timer
func (cm *ChartManager) Destroy() {
	cm.timer.Stop()
	for _, cd := range cm.charts {
		cd.dock.DeleteLater()
	}
	for _, gcv := range cm.groupCharts {
		gcv.Close()
		gcv.Dock().DeleteLater()
	}
	for _, tv := range cm.todayViews {
		tv.Close()
		tv.Dock().DeleteLater()
	}
	for _, pv := range cm.pastViews {
		pv.Close()
		pv.Dock().DeleteLater()
	}
	for _, xv := range cm.xlogViews {
		xv.Close()
		xv.Dock().DeleteLater()
	}
	for _, ev := range cm.eqViews {
		ev.Close()
		ev.Dock().DeleteLater()
	}
	for _, asv := range cm.activeServiceViews {
		asv.Stop()
		asv.Dock().DeleteLater()
	}
	cm.charts = nil
	cm.groupCharts = nil
	cm.todayViews = nil
	cm.pastViews = nil
	cm.xlogViews = nil
	cm.eqViews = nil
	cm.activeServiceViews = nil
}

func main() {
	app := qt6.NewQApplication(os.Args)
	_ = app

	// 앱 아이콘 설정
	pixmap := qt6.NewQPixmap()
	pixmap.LoadFromDataWithData(assets.AppIconPNG)
	appIcon := qt6.NewQIcon2(pixmap)
	qt6.QGuiApplication_SetWindowIcon(appIcon)

	// 메인 윈도우 생성 (QMainWindow)
	mainWindow := qt6.NewQMainWindow2()
	mainWindow.SetWindowTitle(appName)

	// JSON 파일에서 저장된 설정 로드
	appSettings := settings.Load()

	// 저장된 geometry 복원 시도
	geometryRestored := false
	if len(appSettings.Geometry) > 0 {
		mainWindow.RestoreGeometry(appSettings.Geometry)
		geometryRestored = true
	}

	if !geometryRestored {
		screen := qt6.QGuiApplication_PrimaryScreen()
		screenGeometry := screen.AvailableGeometry()

		screenWidth := screenGeometry.Width()
		screenHeight := screenGeometry.Height()

		windowWidth := screenWidth * 70 / 100
		windowHeight := screenHeight * 70 / 100

		x := (screenWidth - windowWidth) / 2
		y := (screenHeight - windowHeight) / 2

		mainWindow.SetGeometry(x, y, windowWidth, windowHeight)
	}

	// 창 이동/크기 변경 시 저장
	saveGeometryToFile := func() {
		appSettings.SetGeometry(mainWindow.SaveGeometry())
	}

	mainWindow.OnMoveEvent(func(super func(event *qt6.QMoveEvent), event *qt6.QMoveEvent) {
		super(event)
		saveGeometryToFile()
	})

	mainWindow.OnResizeEvent(func(super func(event *qt6.QResizeEvent), event *qt6.QResizeEvent) {
		super(event)
		saveGeometryToFile()
	})

	// dock 위젯을 중앙에 배치할 수 있도록 설정
	mainWindow.SetDockNestingEnabled(true)

	// 모든 dock 영역의 탭을 상단에 표시
	mainWindow.SetTabPosition(qt6.AllDockWidgetAreas, qt6.QTabWidget__North)

	// Perspective Manager 생성 (factory set below after saveWindowState is defined)
	var perspMgr *perspective.Manager

	// Import 후 상태 저장 방지 플래그
	skipSave := false

	// 상태 저장 함수
	saveWindowState := func() {
		if perspMgr != nil && !perspMgr.IsSwitching() && !skipSave {
			perspMgr.SaveCurrentState()
			appSettings.Save()
		}
	}

	// ChartManager 팩토리 함수 (includes saveWindowState callback)
	chartManagerFactory := func(perspID string) perspective.ChartManagerInterface {
		cm := NewChartManager(mainWindow, appSettings, perspID)
		cm.SetOnStateChanged(saveWindowState)
		return cm
	}

	perspMgr = perspective.NewManager(mainWindow, appSettings, chartManagerFactory)

	// 모든 perspective 복원 (PerspectiveOrder 순서대로)
	stateRestored := false
	activePerspID := perspective.ID(appSettings.ActivePerspective)

	if len(appSettings.PerspectiveOrder) > 0 {
		// PerspectiveOrder에 따라 복원
		for _, id := range appSettings.PerspectiveOrder {
			ps := appSettings.GetPerspectiveState(id)
			name := ps.Name
			if name == "" {
				name = id // fallback
			}
			cm := NewChartManager(mainWindow, appSettings, id)
			cm.SetOnStateChanged(saveWindowState)
			p := perspective.NewServicePerspective(perspective.ID(id), name, cm)
			perspMgr.Register(p)

			// 모든 perspective의 dock 생성
			hasState := len(ps.GroupCharts) > 0 || len(ps.TodayViews) > 0 || len(ps.PastViews) > 0 || len(ps.XLogViews) > 0 || len(ps.EQViews) > 0
			if hasState {
				p.RestoreState(mainWindow, ps)
				stateRestored = true
			}

			// 비활성 perspective는 즉시 Deactivate (dock hide)
			if perspective.ID(id) != activePerspID {
				p.Deactivate(mainWindow)
			}
		}
	} else if _, ok := appSettings.Perspectives["service"]; ok {
		// PerspectiveOrder 없지만 service perspective 존재 (마이그레이션 직후)
		ps := appSettings.GetPerspectiveState("service")
		name := ps.Name
		if name == "" {
			name = "Service"
		}
		cm := NewChartManager(mainWindow, appSettings, "service")
		cm.SetOnStateChanged(saveWindowState)
		p := perspective.NewServicePerspective(perspective.ServiceID, name, cm)
		perspMgr.Register(p)
		activePerspID = perspective.ServiceID
		hasState := len(ps.GroupCharts) > 0 || len(ps.TodayViews) > 0 || len(ps.PastViews) > 0 || len(ps.XLogViews) > 0 || len(ps.EQViews) > 0
		if hasState {
			p.RestoreState(mainWindow, ps)
			stateRestored = true
		}
	}

	// perspective가 하나도 없으면 기본 Service perspective 생성
	if len(perspMgr.Perspectives()) == 0 {
		cm := NewChartManager(mainWindow, appSettings, "service")
		cm.SetOnStateChanged(saveWindowState)
		p := perspective.NewServicePerspective(perspective.ServiceID, "Service", cm)
		perspMgr.Register(p)
		activePerspID = perspective.ServiceID
	}

	// Group Navigation View 생성 (좌측 탐색기)
	groupNavView := groupnav.NewView(mainWindow)

	// Restore navigation tree state (collapsed items, active tab)
	if len(appSettings.NavCollapsedItems) > 0 {
		groupNavView.SetCollapsedItems(appSettings.NavCollapsedItems)
	}
	if appSettings.NavActiveTab > 0 {
		groupNavView.SetActiveTab(appSettings.NavActiveTab)
	}

	// getActiveChartManager returns the active perspective's ChartManager
	getActiveChartManager := func() *ChartManager {
		p := perspMgr.ActivePerspective()
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

	// Set group chart callback - delegates to active perspective with viewMode routing
	groupNavView.SetOnAddGroupChart(func(groupName, objType, counterName, displayName string, viewMode string) {
		cm := getActiveChartManager()
		if cm == nil {
			return
		}
		switch viewMode {
		case "live-time-all":
			cm.AddGroupChart(groupName, objType, counterName, displayName)
		case "live-time-total":
			cm.addGroupChartWithMode(groupName, objType, counterName, displayName, "live-time-total")
		case "live-daily-all":
			cm.AddTodayView(groupName, objType, counterName, displayName, "live-daily-all")
		case "live-daily-total":
			cm.AddTodayView(groupName, objType, counterName, displayName, "live-daily-total")
		case "load-time-all":
			cm.AddPastView(groupName, objType, counterName, displayName, "load-time-all")
		case "load-time-total":
			cm.AddPastView(groupName, objType, counterName, displayName, "load-time-total")
		case "load-daily-all":
			cm.AddPastView(groupName, objType, counterName, displayName, "load-daily-all")
		case "load-daily-total":
			cm.AddPastView(groupName, objType, counterName, displayName, "load-daily-total")
		default:
			cm.AddGroupChart(groupName, objType, counterName, displayName)
		}
	})

	// Set group XLog callback
	groupNavView.SetOnAddGroupXLog(func(groupName, objType string) {
		if cm := getActiveChartManager(); cm != nil {
			cm.AddXLogView(groupName, objType)
		}
	})

	// Set group EQ callback
	groupNavView.SetOnAddGroupEQ(func(groupName, objType string) {
		if cm := getActiveChartManager(); cm != nil {
			cm.AddEQView(groupName, objType)
		}
	})

	// Group Navigation dock 변경 이벤트 연결
	groupNavDock := groupNavView.Dock()
	groupNavDock.OnDockLocationChanged(func(area qt6.DockWidgetArea) {
		saveWindowState()
	})
	groupNavDock.OnTopLevelChanged(func(topLevel bool) {
		saveWindowState()
	})
	groupNavDock.OnVisibilityChanged(func(visible bool) {
		saveWindowState()
	})

	// 메뉴 매니저 생성
	menuMgr := NewMenuManager(mainWindow, perspMgr, groupNavView.Dock())

	// Export 전 현재 앱 상태 저장 콜백
	menuMgr.SetOnBeforeExport(func() {
		appSettings.NavCollapsedItems = groupNavView.GetCollapsedItems()
		appSettings.NavActiveTab = groupNavView.GetActiveTab()
		perspMgr.SaveAllStates()
		appSettings.Save()
	})

	// Import 후 자동 재시작 콜백
	menuMgr.SetOnAfterImport(func() {
		skipSave = true
		exe, err := os.Executable()
		if err != nil {
			qt6.QMessageBox_Information(mainWindow.QWidget, "Import Settings",
				"Settings imported successfully.\nPlease restart the application to apply.")
			return
		}
		qt6.QMessageBox_Information(mainWindow.QWidget, "Import Settings",
			"Settings imported successfully.\nThe application will restart now.")
		syscall.Exec(exe, os.Args, os.Environ())
	})

	// 저장된 상태가 없으면 기본 차트 4개 추가
	if !stateRestored {
		if p := perspMgr.Perspectives()[0]; p != nil {
			p.SetupDefaultLayout(mainWindow)
		}
	}

	// 활성 perspective 설정 (탭 선택만, dock은 이미 추가됨)
	perspMgr.SetActiveInitial(activePerspID)

	// 비활성 perspective의 dock이 보이지 않도록 정리
	perspMgr.HideInactiveDocks()

	// 활성 perspective의 dock 레이아웃 복원 (위치 + 비율)
	// 비활성 dock을 임시 제거하여 RestoreState가 활성 dock만 복원하도록 함
	if ps := appSettings.GetPerspectiveState(string(activePerspID)); ps != nil && len(ps.WindowState) > 0 {
		restoreInactive := perspMgr.HideInactiveDocksFromRestore()
		size := mainWindow.Size()
		mainWindow.QWidget.SetFixedSize(size)
		mainWindow.RestoreState(ps.WindowState)
		mainWindow.QWidget.SetMinimumSize2(0, 0)
		mainWindow.QWidget.SetMaximumSize2(16777215, 16777215)
		restoreInactive()
	}

	// CounterEngine 시작 (서버에서 카운터 데이터 폴링)
	model.GetCounterEngine().Start()

	// AutoConnect가 활성화된 서버들에 자동 연결
	go server.GetManager().ConnectAll()

	// 창 닫을 때 perspective 상태 저장
	mainWindow.OnCloseEvent(func(super func(event *qt6.QCloseEvent), event *qt6.QCloseEvent) {
		// Alert View 스트리밍 고루틴 정리
		menuMgr.StopAlertView()

		// Import 후에는 저장하지 않음 (imported 파일을 덮어쓰지 않도록)
		if !skipSave {
			appSettings.NavCollapsedItems = groupNavView.GetCollapsedItems()
			appSettings.NavActiveTab = groupNavView.GetActiveTab()

			perspMgr.SaveAllStates()
			appSettings.Save()
		}
		super(event)
	})

	// 윈도우 표시
	mainWindow.Show()

	// 저장된 상태가 없으면 기본 비율 설정 (2:2:6)
	if !stateRestored {
		windowWidth := mainWindow.Width()
		leftWidth := windowWidth * 2 / 10
		rightWidth := windowWidth * 6 / 10

		leftDock := groupNavView.Dock()
		if leftDock != nil {
			mainWindow.ResizeDocks([]*qt6.QDockWidget{leftDock}, []int{leftWidth}, qt6.Horizontal)
		}

		if cm := getActiveChartManager(); cm != nil {
			rightDocks := cm.GetDocks()
			if len(rightDocks) > 0 {
				sizes := make([]int, len(rightDocks))
				for i := range sizes {
					sizes[i] = rightWidth / len(rightDocks)
				}
				mainWindow.ResizeDocks(rightDocks, sizes, qt6.Horizontal)
			}
		}
	}

	// 이벤트 루프 실행
	qt6.QApplication_Exec()
}
