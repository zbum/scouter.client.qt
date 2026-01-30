package main

import (
	"fmt"
	"os"
	"time"

	"scouter.client.qt/chart"
	"scouter.client.qt/groupnav"
	"scouter.client.qt/model"
	"scouter.client.qt/server"
	"scouter.client.qt/settings"
	"scouter.client.qt/views"
	"scouter.client.qt/xlog"

	"github.com/mappu/miqt/qt6"
)

const appName = "scouter.client.go"

// ChartManager manages multiple chart dock widgets
type ChartManager struct {
	mainWindow     *qt6.QMainWindow
	charts         []*ChartDock
	groupCharts    []*views.GroupCounterView
	xlogViews      []*xlog.View
	chartCount     int
	timer          *qt6.QTimer
	onStateChanged func() // Callback when dock state changes
	isRestoring    bool   // Flag to prevent saving during restore
	appSettings    *settings.AppSettings
}

// ChartDock holds a chart widget and its dock
type ChartDock struct {
	dock            *qt6.QDockWidget
	chart           *chart.Widget
	id              int
	objectNameBytes []byte // Keep object name bytes alive for QAnyStringView
	counterName     string // Counter name for real data (empty = demo mode)
	subscriptionID  int    // CounterEngine subscription ID
}

func NewChartManager(mainWindow *qt6.QMainWindow, appSettings *settings.AppSettings) *ChartManager {
	cm := &ChartManager{
		mainWindow:  mainWindow,
		chartCount:  0,
		appSettings: appSettings,
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
		title = fmt.Sprintf("Chart %d", id)
	}

	// Create dock widget
	dock := qt6.NewQDockWidget2(title)

	// Create object name bytes and keep in memory for Qt state save/restore
	objNameBytes := []byte(fmt.Sprintf("chartDock%d", id))
	objNameView := qt6.NewQAnyStringView2(objNameBytes)
	dock.SetObjectName(*objNameView)
	dock.SetAllowedAreas(qt6.AllDockWidgetAreas)

	// Create chart widget
	chartWidget := chart.New(nil)
	chartWidget.SetTitle(title)
	dock.SetWidget(chartWidget.QWidget())

	cd := &ChartDock{
		dock:            dock,
		chart:           chartWidget,
		id:              id,
		objectNameBytes: objNameBytes, // Keep in memory for Qt state restore
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

	// Notify state changed (new dock added)
	cm.notifyStateChanged()

	return cd
}

// AddGroupChart creates and tracks a group counter view
func (cm *ChartManager) AddGroupChart(groupName, objType, counterName, displayName string) *views.GroupCounterView {
	gcv := views.NewGroupCounterView(cm.mainWindow, groupName, objType, counterName, displayName)
	cm.groupCharts = append(cm.groupCharts, gcv)

	dock := gcv.Dock()
	dock.OnDockLocationChanged(func(area qt6.DockWidgetArea) {
		cm.notifyStateChanged()
	})
	dock.OnTopLevelChanged(func(topLevel bool) {
		cm.notifyStateChanged()
	})
	dock.OnVisibilityChanged(func(visible bool) {
		cm.notifyStateChanged()
	})

	cm.notifyStateChanged()
	return gcv
}

// AddXLogView creates and tracks a group XLog view
func (cm *ChartManager) AddXLogView(groupName, objType string) *xlog.View {
	xv := xlog.NewGroupXLogView(cm.mainWindow, groupName, objType)
	cm.xlogViews = append(cm.xlogViews, xv)

	dock := xv.Dock()
	dock.OnDockLocationChanged(func(area qt6.DockWidgetArea) {
		cm.notifyStateChanged()
	})
	dock.OnTopLevelChanged(func(topLevel bool) {
		cm.notifyStateChanged()
	})
	dock.OnVisibilityChanged(func(visible bool) {
		cm.notifyStateChanged()
	})

	cm.notifyStateChanged()
	return xv
}

// AddDefaultCharts adds 4 default charts in a 2x2 layout
func (cm *ChartManager) AddDefaultCharts() {
	titles := []string{"TPS", "Response Time", "Active Service", "CPU Usage"}

	var firstDock, secondDock *qt6.QDockWidget

	for i, title := range titles {
		cd := cm.AddChartWithTitle(title)

		if i == 0 {
			firstDock = cd.dock
		} else if i == 1 {
			secondDock = cd.dock
			// 두 번째 차트를 첫 번째 차트 아래에 배치
			cm.mainWindow.SplitDockWidget(firstDock, secondDock, qt6.Vertical)
		} else if i == 2 {
			// 세 번째 차트를 첫 번째 차트 오른쪽에 배치
			cm.mainWindow.SplitDockWidget(firstDock, cd.dock, qt6.Horizontal)
		} else if i == 3 {
			// 네 번째 차트를 세 번째 차트 아래에 배치
			cm.mainWindow.SplitDockWidget(cm.charts[2].dock, cd.dock, qt6.Vertical)
		}
	}
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

func (cm *ChartManager) SaveState() {
	// Save charts with IDs (for matching Qt object names on restore)
	chartConfigs := make([]settings.ChartConfig, len(cm.charts))
	for i, cd := range cm.charts {
		chartConfigs[i] = settings.ChartConfig{
			ID:    cd.id,
			Title: cd.chart.Title(),
		}
	}
	cm.appSettings.Charts = chartConfigs
	cm.appSettings.ChartCount = len(cm.charts) // backward compat

	// Save chart titles (backward compat)
	titles := make([]string, len(cm.charts))
	for i, cd := range cm.charts {
		titles[i] = cd.chart.Title()
	}
	cm.appSettings.ChartTitles = titles

	// Save group counter charts
	var groupConfigs []settings.GroupChartConfig
	for _, gcv := range cm.groupCharts {
		groupConfigs = append(groupConfigs, settings.GroupChartConfig{
			GroupName:   gcv.GroupName(),
			ObjType:     gcv.ObjType(),
			CounterName: gcv.CounterName(),
			DisplayName: gcv.CounterDisplay(),
		})
	}
	cm.appSettings.GroupCharts = groupConfigs

	// Save XLog views
	var xlogConfigs []settings.XLogViewConfig
	for _, xv := range cm.xlogViews {
		xlogConfigs = append(xlogConfigs, settings.XLogViewConfig{
			GroupName: xv.GroupName(),
			ObjType:   xv.ObjType(),
		})
	}
	cm.appSettings.XLogViews = xlogConfigs

	// Save window state (dock positions)
	cm.appSettings.WindowState = cm.mainWindow.SaveState()

	// Save to JSON file
	cm.appSettings.Save()
}

// RestoreState restores chart state, returns true if window state was restored
func (cm *ChartManager) RestoreState() bool {
	cm.isRestoring = true
	defer func() { cm.isRestoring = false }()

	// Restore charts with their original IDs (for Qt object name matching)
	if len(cm.appSettings.Charts) > 0 {
		for _, cc := range cm.appSettings.Charts {
			cm.addChartWithID(cc.ID, cc.Title)
		}
	} else {
		// Backward compat: no Charts field, use ChartCount + ChartTitles
		chartCount := cm.appSettings.ChartCount
		if chartCount < 1 {
			chartCount = 1
		}
		titles := cm.appSettings.ChartTitles
		for i := 0; i < chartCount; i++ {
			var title string
			if i < len(titles) && titles[i] != "" {
				title = titles[i]
			}
			cm.AddChartWithTitle(title)
		}
	}

	// Restore group counter charts
	for _, gc := range cm.appSettings.GroupCharts {
		cm.AddGroupChart(gc.GroupName, gc.ObjType, gc.CounterName, gc.DisplayName)
	}

	// Restore XLog views
	for _, xc := range cm.appSettings.XLogViews {
		cm.AddXLogView(xc.GroupName, xc.ObjType)
	}

	// Restore window state (dock positions and visibility)
	if len(cm.appSettings.WindowState) > 0 {
		cm.mainWindow.RestoreState(cm.appSettings.WindowState)
		return true
	}
	return false
}

func main() {
	app := qt6.NewQApplication(os.Args)
	_ = app

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

	// Chart Manager 생성
	chartManager := NewChartManager(mainWindow, appSettings)

	// Group Navigation View 생성 (좌측 탐색기)
	groupNavView := groupnav.NewView(mainWindow)

	// Restore navigation tree state (collapsed items, active tab)
	if len(appSettings.NavCollapsedItems) > 0 {
		groupNavView.SetCollapsedItems(appSettings.NavCollapsedItems)
	}
	if appSettings.NavActiveTab > 0 {
		groupNavView.SetActiveTab(appSettings.NavActiveTab)
	}

	// Set group chart callback
	groupNavView.SetOnAddGroupChart(func(groupName, objType, counterName, displayName string) {
		chartManager.AddGroupChart(groupName, objType, counterName, displayName)
	})

	// Set group XLog callback
	groupNavView.SetOnAddGroupXLog(func(groupName, objType string) {
		chartManager.AddXLogView(groupName, objType)
	})

	// 상태 저장 함수 (복원 중에는 저장하지 않음)
	saveWindowState := func() {
		if !chartManager.isRestoring {
			chartManager.SaveState()
		}
	}

	// Chart Manager 상태 변경 콜백 설정
	chartManager.SetOnStateChanged(saveWindowState)

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

	// dock 위젯을 중앙에 배치할 수 있도록 설정
	mainWindow.SetDockNestingEnabled(true)

	// 모든 dock 영역의 탭을 상단에 표시
	mainWindow.SetTabPosition(qt6.AllDockWidgetAreas, qt6.QTabWidget__North)

	// 메뉴 매니저 생성
	_ = NewMenuManager(mainWindow, chartManager)

	// 차트 상태 복원 먼저 시도
	stateRestored := chartManager.RestoreState()

	// 저장된 상태가 없으면 기본 차트 4개 추가
	if !stateRestored {
		chartManager.AddDefaultCharts()
	}

	// CounterEngine 시작 (서버에서 카운터 데이터 폴링)
	model.GetCounterEngine().Start()

	// AutoConnect가 활성화된 서버들에 자동 연결
	go server.GetManager().ConnectAll()

	// 창 닫을 때 차트 상태 저장
	mainWindow.OnCloseEvent(func(super func(event *qt6.QCloseEvent), event *qt6.QCloseEvent) {
		// Save navigation tree state
		appSettings.NavCollapsedItems = groupNavView.GetCollapsedItems()
		appSettings.NavActiveTab = groupNavView.GetActiveTab()

		chartManager.SaveState()
		super(event)
	})

	// 윈도우 표시
	mainWindow.Show()

	// 저장된 상태가 없으면 기본 비율 설정 (2:2:6)
	if !stateRestored {
		windowWidth := mainWindow.Width()
		// 비율 2:2:6 = 총 10 파트
		leftWidth := windowWidth * 2 / 10
		rightWidth := windowWidth * 6 / 10

		// 왼쪽 dock (Group Navigation)
		leftDock := groupNavView.Dock()

		// 오른쪽 docks (Charts)
		rightDocks := chartManager.GetDocks()

		if leftDock != nil {
			mainWindow.ResizeDocks([]*qt6.QDockWidget{leftDock}, []int{leftWidth}, qt6.Horizontal)
		}

		if len(rightDocks) > 0 {
			// 각 차트 dock에 동일한 너비 할당
			sizes := make([]int, len(rightDocks))
			for i := range sizes {
				sizes[i] = rightWidth / len(rightDocks)
			}
			mainWindow.ResizeDocks(rightDocks, sizes, qt6.Horizontal)
		}
	}

	// 이벤트 루프 실행
	qt6.QApplication_Exec()
}
