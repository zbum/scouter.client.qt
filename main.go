package main

import (
	"fmt"
	"math/rand"
	"os"

	"miqt-ex1/chart"

	"github.com/mappu/miqt/qt6"
)

const (
	orgName = "MiqtExample"
	appName = "MiqtDesktopApp"
)

var settings *qt6.QSettings

// ChartManager manages multiple chart dock widgets
type ChartManager struct {
	mainWindow *qt6.QMainWindow
	charts     []*ChartDock
	chartCount int
	timer      *qt6.QTimer
}

// ChartDock holds a chart widget and its dock
type ChartDock struct {
	dock   *qt6.QDockWidget
	chart  *chart.Widget
	id     int
}

func NewChartManager(mainWindow *qt6.QMainWindow) *ChartManager {
	cm := &ChartManager{
		mainWindow: mainWindow,
		chartCount: 0,
	}

	// Timer for updating all charts
	cm.timer = qt6.NewQTimer()
	cm.timer.OnTimeout(func() {
		for _, cd := range cm.charts {
			value := int64(50 + rand.Intn(450))
			cd.chart.AddPoint(value)
		}
	})
	cm.timer.Start(1000)

	return cm
}

func (cm *ChartManager) AddChart() *ChartDock {
	cm.chartCount++
	title := fmt.Sprintf("Chart %d", cm.chartCount)

	// Create dock widget
	dock := qt6.NewQDockWidget2(title)
	objectName := qt6.NewQAnyStringView3(fmt.Sprintf("chartDock%d", cm.chartCount))
	dock.SetObjectName(*objectName)
	dock.SetAllowedAreas(qt6.AllDockWidgetAreas)

	// Create chart widget
	chartWidget := chart.New(nil)
	dock.SetWidget(chartWidget.QWidget())

	// Add initial data
	for i := 0; i < 10; i++ {
		value := int64(50 + rand.Intn(450))
		chartWidget.AddPoint(value)
	}

	// Add to main window
	cm.mainWindow.AddDockWidget(qt6.RightDockWidgetArea, dock)

	cd := &ChartDock{
		dock:  dock,
		chart: chartWidget,
		id:    cm.chartCount,
	}
	cm.charts = append(cm.charts, cd)

	return cd
}

func (cm *ChartManager) RemoveChart(cd *ChartDock) {
	// Find and remove from slice
	for i, c := range cm.charts {
		if c == cd {
			cm.charts = append(cm.charts[:i], cm.charts[i+1:]...)
			break
		}
	}

	// Remove from main window
	cm.mainWindow.RemoveDockWidget(cd.dock)
	cd.dock.DeleteLater()
}

func (cm *ChartManager) RemoveLastChart() {
	if len(cm.charts) > 0 {
		cm.RemoveChart(cm.charts[len(cm.charts)-1])
	}
}

func (cm *ChartManager) ChartCount() int {
	return len(cm.charts)
}

func (cm *ChartManager) SaveState() {
	// Save chart count
	countKey := qt6.NewQAnyStringView3("chartCount")
	countValue := qt6.NewQVariant6(int64(cm.chartCount))
	settings.SetValue(*countKey, countValue)

	// Save window state (dock positions)
	stateKey := qt6.NewQAnyStringView3("windowState")
	stateValue := qt6.NewQVariant12(cm.mainWindow.SaveState())
	settings.SetValue(*stateKey, stateValue)

	settings.Sync()
}

func (cm *ChartManager) RestoreState() {
	// Restore chart count
	countKey := qt6.NewQAnyStringView3("chartCount")
	countValue := settings.ValueWithKey(*countKey)

	chartCount := 1 // default
	if countValue != nil {
		chartCount = countValue.ToInt()
		if chartCount < 1 {
			chartCount = 1
		}
	}

	// Create charts
	for i := 0; i < chartCount; i++ {
		cm.AddChart()
	}

	// Restore window state (dock positions)
	stateKey := qt6.NewQAnyStringView3("windowState")
	stateValue := settings.ValueWithKey(*stateKey)

	if stateValue != nil && len(stateValue.ToByteArray()) > 0 {
		cm.mainWindow.RestoreState(stateValue.ToByteArray())
	}
}

func main() {
	app := qt6.NewQApplication(os.Args)
	_ = app

	// 설정 객체 생성 (global 변수에 할당)
	settings = qt6.NewQSettings7(orgName, appName)

	// 메인 윈도우 생성 (QMainWindow)
	mainWindow := qt6.NewQMainWindow2()
	mainWindow.SetWindowTitle("MIQT 데스크탑 앱")

	// 저장된 geometry 복원 시도
	geometryKey := qt6.NewQAnyStringView3("geometry")
	savedGeometry := settings.ValueWithKey(*geometryKey)

	if savedGeometry != nil && len(savedGeometry.ToByteArray()) > 0 {
		mainWindow.RestoreGeometry(savedGeometry.ToByteArray())
	} else {
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
	saveGeometry := func() {
		key := qt6.NewQAnyStringView3("geometry")
		value := qt6.NewQVariant12(mainWindow.SaveGeometry())
		settings.SetValue(*key, value)
		settings.Sync()
	}

	mainWindow.OnMoveEvent(func(super func(event *qt6.QMoveEvent), event *qt6.QMoveEvent) {
		super(event)
		saveGeometry()
	})

	mainWindow.OnResizeEvent(func(super func(event *qt6.QResizeEvent), event *qt6.QResizeEvent) {
		super(event)
		saveGeometry()
	})

	// Chart Manager 생성
	chartManager := NewChartManager(mainWindow)

	// Tree Manager 생성 (좌측 탐색기)
	_ = NewTreeManager(mainWindow)

	// 메뉴 매니저 생성
	_ = NewMenuManager(mainWindow, chartManager)

	// 중앙 위젯 (컨트롤 패널)
	centralWidget := qt6.NewQWidget2()
	centralLayout := qt6.NewQVBoxLayout(centralWidget)

	// 레이블 추가
	label := qt6.NewQLabel3("환영합니다! 이것은 MIQT로 만든 Go 데스크탑 앱입니다.")
	label.SetAlignment(qt6.AlignCenter)
	centralLayout.AddWidget(label.QWidget)

	// 텍스트 입력 필드
	lineEdit := qt6.NewQLineEdit2()
	lineEdit.SetPlaceholderText("여기에 텍스트를 입력하세요...")
	centralLayout.AddWidget(lineEdit.QWidget)

	// 결과 레이블
	resultLabel := qt6.NewQLabel3("")
	resultLabel.SetAlignment(qt6.AlignCenter)
	centralLayout.AddWidget(resultLabel.QWidget)

	// 버튼 추가
	button := qt6.NewQPushButton3("클릭하세요!")
	centralLayout.AddWidget(button.QWidget)

	// 버튼 클릭 이벤트 연결
	clickCount := 0
	button.OnClicked(func() {
		clickCount++
		text := lineEdit.Text()
		if text == "" {
			resultLabel.SetText(fmt.Sprintf("버튼이 %d번 클릭되었습니다!", clickCount))
		} else {
			resultLabel.SetText(fmt.Sprintf("안녕하세요, %s님!", text))
		}
	})

	// 차트 추가 버튼
	addChartButton := qt6.NewQPushButton3("차트 추가")
	centralLayout.AddWidget(addChartButton.QWidget)
	addChartButton.OnClicked(func() {
		chartManager.AddChart()
	})

	// 차트 삭제 버튼
	removeChartButton := qt6.NewQPushButton3("마지막 차트 삭제")
	centralLayout.AddWidget(removeChartButton.QWidget)
	removeChartButton.OnClicked(func() {
		chartManager.RemoveLastChart()
	})

	// 종료 버튼
	quitButton := qt6.NewQPushButton3("종료")
	centralLayout.AddWidget(quitButton.QWidget)

	quitButton.OnClicked(func() {
		qt6.QCoreApplication_Quit()
	})

	// Spacer
	centralLayout.AddStretchWithStretch(1)

	mainWindow.SetCentralWidget(centralWidget)

	// 차트 상태 복원 (저장된 차트 개수와 배치)
	chartManager.RestoreState()

	// 창 닫을 때 차트 상태 저장
	mainWindow.OnCloseEvent(func(super func(event *qt6.QCloseEvent), event *qt6.QCloseEvent) {
		chartManager.SaveState()
		super(event)
	})

	// 윈도우 표시
	mainWindow.Show()

	// 이벤트 루프 실행
	qt6.QApplication_Exec()
}