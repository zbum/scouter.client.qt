package views

import (
	"fmt"
	"time"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/chart"
	"scouter.client.qt/model"
)

// CounterType represents different counter types
type CounterType struct {
	Name        string
	Counter     string
	Unit        string
	MaxValue    int64
	Description string
}

// Standard counter types
var (
	CounterTPS = CounterType{
		Name:        "TPS",
		Counter:     "TPS",
		Unit:        "req/s",
		MaxValue:    1000,
		Description: "Transactions Per Second",
	}
	CounterResponseTime = CounterType{
		Name:        "Response Time",
		Counter:     "ElapsedTime",
		Unit:        "ms",
		MaxValue:    5000,
		Description: "Average Response Time",
	}
	CounterActiveService = CounterType{
		Name:        "Active Service",
		Counter:     "ActiveService",
		Unit:        "count",
		MaxValue:    100,
		Description: "Active Service Count",
	}
	CounterCPU = CounterType{
		Name:        "CPU",
		Counter:     "Cpu",
		Unit:        "%",
		MaxValue:    100,
		Description: "CPU Usage",
	}
	CounterMemory = CounterType{
		Name:        "Memory",
		Counter:     "UsedMemory",
		Unit:        "MB",
		MaxValue:    1000,
		Description: "Used Memory",
	}
	CounterGCCount = CounterType{
		Name:        "GC Count",
		Counter:     "GcCount",
		Unit:        "count",
		MaxValue:    100,
		Description: "Garbage Collection Count",
	}
	CounterGCTime = CounterType{
		Name:        "GC Time",
		Counter:     "GcTime",
		Unit:        "ms",
		MaxValue:    1000,
		Description: "Garbage Collection Time",
	}
)

// AllCounterTypes returns all available counter types
func AllCounterTypes() []CounterType {
	return []CounterType{
		CounterTPS,
		CounterResponseTime,
		CounterActiveService,
		CounterCPU,
		CounterMemory,
		CounterGCCount,
		CounterGCTime,
	}
}

// CounterView represents a counter visualization view with real-time data
type CounterView struct {
	dock            *qt6.QDockWidget
	objectNameBytes []byte
	chart           *chart.Widget
	counterCombo    *qt6.QComboBox
	currentCounter  CounterType
	subscriptionID  int
	engine          *model.CounterEngine
	objHash         int32
	autoScale       bool
	autoScaleCheck  *qt6.QCheckBox
	maxObserved     int64
}

// NewCounterView creates a new counter view
func NewCounterView(mainWindow *qt6.QMainWindow, title string, objHash int32) *CounterView {
	view := &CounterView{
		engine:         model.GetCounterEngine(),
		objHash:        objHash,
		currentCounter: CounterTPS,
		autoScale:      true,
	}

	// Create dock widget
	view.dock = qt6.NewQDockWidget2(title)
	view.objectNameBytes = []byte(fmt.Sprintf("counterDock_%s", title))
	objectNameView := qt6.NewQAnyStringView2(view.objectNameBytes)
	view.dock.SetObjectName(*objectNameView)
	view.dock.SetAllowedAreas(qt6.AllDockWidgetAreas)

	view.setupUI()
	view.setupConnections()
	view.subscribeToCounter(view.currentCounter)

	// Add to main window
	mainWindow.AddDockWidget(qt6.RightDockWidgetArea, view.dock)

	return view
}

// setupUI creates the UI components
func (v *CounterView) setupUI() {
	// Create main widget and layout
	mainWidget := qt6.NewQWidget2()
	mainLayout := qt6.NewQVBoxLayout(mainWidget)
	mainLayout.SetContentsMargins(4, 4, 4, 4)
	mainLayout.SetSpacing(4)

	// Create toolbar
	toolbar := v.createToolbar()
	mainLayout.AddLayout(toolbar.QLayout)

	// Create chart
	config := chart.DefaultConfig()
	config.Title = v.currentCounter.Name
	config.MaxValue = v.currentCounter.MaxValue
	config.MinWidth = 350
	config.MinHeight = 200
	v.chart = chart.NewWithConfig(mainWidget, config)
	mainLayout.AddWidget(v.chart.QWidget())

	// Set main widget
	v.dock.SetWidget(mainWidget)
}

// createToolbar creates the toolbar with controls
func (v *CounterView) createToolbar() *qt6.QHBoxLayout {
	toolbar := qt6.NewQHBoxLayout2()
	toolbar.SetContentsMargins(0, 0, 0, 0)
	toolbar.SetSpacing(8)

	// Counter type selector
	counterLabel := qt6.NewQLabel3("Counter:")
	toolbar.AddWidget(counterLabel.QWidget)

	v.counterCombo = qt6.NewQComboBox2()
	for _, ct := range AllCounterTypes() {
		v.counterCombo.AddItem(ct.Name)
	}
	v.counterCombo.SetCurrentIndex(0) // Default to TPS
	v.counterCombo.SetMinimumWidth(120)
	toolbar.AddWidget(v.counterCombo.QWidget)

	// Auto-scale checkbox
	v.autoScaleCheck = qt6.NewQCheckBox3("Auto Scale")
	v.autoScaleCheck.SetChecked(v.autoScale)
	toolbar.AddWidget(v.autoScaleCheck.QWidget)

	// Clear button
	clearBtn := qt6.NewQPushButton3("Clear")
	clearBtn.OnClicked(func() {
		v.chart.Clear()
		v.maxObserved = 0
	})
	toolbar.AddWidget(clearBtn.QWidget)

	toolbar.AddStretch()

	return toolbar
}

// setupConnections sets up signal-slot connections
func (v *CounterView) setupConnections() {
	// Counter type change
	v.counterCombo.OnCurrentIndexChanged(func(index int) {
		v.onCounterTypeChanged(index)
	})

	// Auto-scale change
	v.autoScaleCheck.OnStateChanged(func(state int) {
		v.autoScale = state == int(qt6.Checked)
	})

	// Handle dock close
	v.dock.OnVisibilityChanged(func(visible bool) {
		if !visible {
			v.unsubscribeFromCounter()
		} else if v.subscriptionID == 0 {
			v.subscribeToCounter(v.currentCounter)
		}
	})
}

// onCounterTypeChanged handles counter type selection changes
func (v *CounterView) onCounterTypeChanged(index int) {
	counterTypes := AllCounterTypes()
	if index < 0 || index >= len(counterTypes) {
		return
	}

	newCounter := counterTypes[index]

	// Unsubscribe from old counter
	v.unsubscribeFromCounter()

	// Update current counter
	v.currentCounter = newCounter
	v.chart.SetTitle(newCounter.Name)
	v.chart.SetMaxValue(newCounter.MaxValue)
	v.chart.Clear()
	v.maxObserved = 0

	// Subscribe to new counter
	v.subscribeToCounter(newCounter)
}

// subscribeToCounter subscribes to counter updates
func (v *CounterView) subscribeToCounter(ct CounterType) {
	v.subscriptionID = v.engine.Subscribe(ct.Counter, v.objHash, func(objHash int32, counter string, value float64, timestamp time.Time) {
		displayValue := int64(value)

		// Auto-scale Y-axis if enabled
		if v.autoScale {
			if displayValue > v.maxObserved {
				v.maxObserved = displayValue
			}
			// Scale to 120% of max observed, rounded to nice number
			if v.maxObserved > 0 {
				newMax := ((v.maxObserved * 12 / 10) / 100) * 100
				if newMax < 100 {
					newMax = 100
				}
				if newMax > v.currentCounter.MaxValue {
					v.chart.SetMaxValue(newMax)
				}
			}
		}

		// Add data point to chart
		v.chart.AddPoint(displayValue)
	})
}

// unsubscribeFromCounter unsubscribes from counter updates
func (v *CounterView) unsubscribeFromCounter() {
	if v.subscriptionID != 0 {
		v.engine.Unsubscribe(v.subscriptionID)
		v.subscriptionID = 0
	}
}

// Dock returns the underlying dock widget
func (v *CounterView) Dock() *qt6.QDockWidget {
	return v.dock
}

// SetObjHash sets the object hash to monitor
func (v *CounterView) SetObjHash(objHash int32) {
	if v.objHash == objHash {
		return
	}

	v.objHash = objHash
	v.chart.Clear()
	v.maxObserved = 0

	// Resubscribe with new object hash
	v.unsubscribeFromCounter()
	v.subscribeToCounter(v.currentCounter)
}

// SetCounterType sets the counter type to display
func (v *CounterView) SetCounterType(ct CounterType) {
	// Find index of counter type
	for i, counterType := range AllCounterTypes() {
		if counterType.Counter == ct.Counter {
			v.counterCombo.SetCurrentIndex(i)
			return
		}
	}
}

// Close closes the view and cleans up subscriptions
func (v *CounterView) Close() {
	v.unsubscribeFromCounter()
	v.dock.Close()
}

// SetTitle sets the dock widget title
func (v *CounterView) SetTitle(title string) {
	v.dock.SetWindowTitle(title)
}

// IsVisible returns whether the dock is visible
func (v *CounterView) IsVisible() bool {
	return v.dock.IsVisible()
}

// Show shows the dock widget
func (v *CounterView) Show() {
	v.dock.Show()
	if v.subscriptionID == 0 {
		v.subscribeToCounter(v.currentCounter)
	}
}

// Hide hides the dock widget
func (v *CounterView) Hide() {
	v.dock.Hide()
	v.unsubscribeFromCounter()
}

// LoadHistoricalData loads historical data for the current counter
func (v *CounterView) LoadHistoricalData(duration time.Duration) {
	v.chart.Clear()
	v.maxObserved = 0

	history := v.engine.GetHistory(v.objHash, v.currentCounter.Counter, duration)
	if history == nil {
		return
	}

	for _, data := range history {
		displayValue := int64(data.Value)
		if displayValue > v.maxObserved {
			v.maxObserved = displayValue
		}
		v.chart.AddPoint(displayValue)
	}
}

// GetCurrentValue returns the latest counter value as formatted string
func (v *CounterView) GetCurrentValue() string {
	latest := v.engine.GetLatest(v.objHash, v.currentCounter.Counter)
	if latest == nil {
		return "N/A"
	}
	return fmt.Sprintf("%.2f %s", latest.Value, v.currentCounter.Unit)
}
