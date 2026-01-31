package views

import (
	"fmt"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/cache"
	"scouter.client.qt/chart"
	"scouter.client.qt/groupnav"
	"scouter.client.qt/protocol"
	"scouter.client.qt/protocol/io"
	"scouter.client.qt/protocol/pack"
	"scouter.client.qt/server"
)

// GroupCounterView displays per-agent counter data as separate lines on one chart
type GroupCounterView struct {
	dock            *qt6.QDockWidget
	objectNameBytes []byte
	chart           *chart.Widget
	id              int
	groupName       string
	counterName     string
	counterDisplay  string
	objType         string
	maxObserved     int64
	autoScale       bool
	timer           *qt6.QTimer
	active          bool
}

// NewGroupCounterView creates a new group counter chart dock widget
func NewGroupCounterView(mainWindow *qt6.QMainWindow, groupName, objType, counterName, displayName string) *GroupCounterView {
	return NewGroupCounterViewWithID(mainWindow, 0, groupName, objType, counterName, displayName)
}

// NewGroupCounterViewWithID creates a new group counter chart dock widget with a specific ID
func NewGroupCounterViewWithID(mainWindow *qt6.QMainWindow, id int, groupName, objType, counterName, displayName string) *GroupCounterView {
	view := &GroupCounterView{
		id:             id,
		groupName:      groupName,
		counterName:    counterName,
		counterDisplay: displayName,
		objType:        objType,
		autoScale:      true,
		active:         true,
	}

	title := fmt.Sprintf("%s - %s", groupName, displayName)

	// Create dock widget
	view.dock = qt6.NewQDockWidget2(title)
	view.objectNameBytes = []byte(fmt.Sprintf("groupCounterDock_%d_%s_%s", id, groupName, counterName))
	objectNameView := qt6.NewQAnyStringView2(view.objectNameBytes)
	view.dock.SetObjectName(*objectNameView)
	view.dock.SetAllowedAreas(qt6.AllDockWidgetAreas)

	view.setupUI(title)

	// Start polling timer (2 seconds)
	view.timer = qt6.NewQTimer()
	view.timer.OnTimeout(func() {
		view.fetchAndUpdate()
	})
	view.timer.Start(2000)

	// Handle dock close
	view.dock.OnVisibilityChanged(func(visible bool) {
		if !visible {
			view.active = false
			if view.timer != nil {
				view.timer.Stop()
			}
		} else if !view.active {
			view.active = true
			if view.timer != nil {
				view.timer.Start(2000)
			}
		}
	})

	// Add to main window
	mainWindow.AddDockWidget(qt6.RightDockWidgetArea, view.dock)
	view.dock.Show()

	return view
}

// setupUI creates the UI components
func (v *GroupCounterView) setupUI(title string) {
	mainWidget := qt6.NewQWidget2()
	mainLayout := qt6.NewQVBoxLayout(mainWidget)
	mainLayout.SetContentsMargins(4, 4, 4, 4)
	mainLayout.SetSpacing(4)

	// Find matching counter type for MaxValue
	var maxValue int64 = 1000
	for _, ct := range AllCounterTypes() {
		if ct.Counter == v.counterName {
			maxValue = ct.MaxValue
			break
		}
	}

	config := chart.DefaultConfig()
	config.Title = title
	config.MaxValue = maxValue
	config.MinWidth = 100
	config.MinHeight = 80
	config.ShowMarkers = false
	config.YAxisFormatter = chart.FormatAbbreviated
	config.TimeRange = 300  // 5 minutes
	config.MaxPoints = 150  // 2s polling × 300s
	v.chart = chart.NewWithConfig(mainWidget, config)
	mainLayout.AddWidget(v.chart.QWidget())

	v.dock.SetWidget(mainWidget)
}

// fetchAndUpdate fetches counter data from servers and updates the chart (per-agent lines)
func (v *GroupCounterView) fetchAndUpdate() {
	// Get group members
	members := groupnav.GetManager().GetObjectsByGroup(v.groupName)
	if len(members) == 0 {
		return
	}

	// Build objHash list for request
	objHashList := &io.ListValue{}
	for hash := range members {
		objHashList.Add(io.NewDecimalValue(int32(hash)))
	}

	// Get connected servers
	servers := server.GetManager().GetConnectedServers()
	if len(servers) == 0 {
		return
	}

	// Collect per-agent values from all servers
	values := make(map[int32]float64)

	for _, srv := range servers {
		session := srv.Session()
		if session == nil {
			continue
		}

		param := pack.NewMapPack()
		param.PutText(protocol.ParamCounter, v.counterName)
		param.Put(protocol.ParamObjHash, objHashList)

		session.RequestStream(protocol.CMD_COUNTER_REAL_TIME_GROUP, param, func(p pack.Pack) bool {
			mp, ok := p.(*pack.MapPack)
			if !ok {
				return true
			}

			objHashLv := mp.GetListValue("objHash")
			valueLv := mp.GetListValue("value")
			if objHashLv == nil || valueLv == nil {
				return true
			}

			for i := 0; i < objHashLv.Size(); i++ {
				hash := objHashLv.GetInt32(i)
				val := valueLv.Get(i)
				if val == nil {
					continue
				}
				values[hash] = valueToFloat64(val)
			}
			return true
		})
	}

	if len(values) == 0 {
		return
	}

	// Add each agent's value as a separate series point
	var maxVal int64
	for hash, val := range values {
		name := cache.GetObjectCache().GetObjName(hash)
		if name == "" {
			name = fmt.Sprintf("obj-%d", hash)
		}
		v.chart.AddSeriesPoint(name, int64(val))
		if int64(val) > maxVal {
			maxVal = int64(val)
		}
	}

	// Auto-scale Y-axis based on max across all agents
	if v.autoScale && maxVal > v.maxObserved {
		v.maxObserved = maxVal
		newMax := ((v.maxObserved * 12 / 10) / 100) * 100
		if newMax < 100 {
			newMax = 100
		}
		v.chart.SetMaxValue(newMax)
	}
}

// valueToFloat64 converts an io.Value to float64
func valueToFloat64(val io.Value) float64 {
	switch v := val.(type) {
	case *io.FloatValue:
		return float64(v.Value)
	case *io.DoubleValue:
		return v.Value
	case *io.DecimalValue:
		return float64(v.Value)
	case *io.DecimalLongValue:
		return float64(v.Value)
	default:
		return 0
	}
}

// Dock returns the underlying dock widget
func (v *GroupCounterView) Dock() *qt6.QDockWidget {
	return v.dock
}

// GroupName returns the group name
func (v *GroupCounterView) GroupName() string { return v.groupName }

// ObjType returns the object type
func (v *GroupCounterView) ObjType() string { return v.objType }

// CounterName returns the counter name
func (v *GroupCounterView) CounterName() string { return v.counterName }

// CounterDisplay returns the display name
func (v *GroupCounterView) CounterDisplay() string { return v.counterDisplay }

// ID returns the view ID
func (v *GroupCounterView) ID() int { return v.id }

// Close closes the view and cleans up
func (v *GroupCounterView) Close() {
	v.active = false
	if v.timer != nil {
		v.timer.Stop()
	}
	v.dock.Close()
}
