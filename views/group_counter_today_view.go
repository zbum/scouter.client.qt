package views

import (
	"fmt"
	"time"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/cache"
	"scouter.client.qt/chart"
	"scouter.client.qt/groupnav"
	"scouter.client.qt/protocol"
	"scouter.client.qt/protocol/io"
	"scouter.client.qt/protocol/pack"
	"scouter.client.qt/qtutil"
	"scouter.client.qt/server"
)

// GroupCounterTodayView displays today's counter data for a group (daily chart)
type GroupCounterTodayView struct {
	dock           *qt6.QDockWidget
	chart          *chart.Widget
	id             int
	groupName      string
	counterName    string
	counterDisplay string
	objType        string
	viewMode       string // "live-daily-all" or "live-daily-total"
	maxObserved    int64
	autoScale      bool
	timer          *qt6.QTimer
	active         bool
}

// NewGroupCounterTodayView creates a new today counter chart dock widget
func NewGroupCounterTodayView(mainWindow *qt6.QMainWindow, id int, groupName, objType, counterName, displayName string, viewMode string) *GroupCounterTodayView {
	if viewMode == "" {
		viewMode = "live-daily-all"
	}

	modeLabel := " (Today All)"
	if viewMode == "live-daily-total" {
		modeLabel = " (Today Total)"
	}

	view := &GroupCounterTodayView{
		id:             id,
		groupName:      groupName,
		counterName:    counterName,
		counterDisplay: displayName,
		objType:        objType,
		viewMode:       viewMode,
		autoScale:      true,
		active:         true,
	}

	title := fmt.Sprintf("%s - %s%s", groupName, displayName, modeLabel)

	// Create dock widget
	view.dock = qt6.NewQDockWidget2(title)
	objectName := fmt.Sprintf("groupCounterTodayDock_%d_%s_%s_%s", id, groupName, counterName, viewMode)
	qtutil.SetObjectName(view.dock.QWidget.QObject, objectName)
	view.dock.SetAllowedAreas(qt6.AllDockWidgetAreas)

	view.setupUI(title)

	// Initial load
	view.fetchAndUpdate()

	// Refresh every 10 seconds (like Java reference)
	view.timer = qt6.NewQTimer()
	view.timer.OnTimeout(func() {
		view.fetchAndUpdate()
	})
	view.timer.Start(10000)

	// Handle dock visibility
	view.dock.OnVisibilityChanged(func(visible bool) {
		if !visible {
			view.active = false
			if view.timer != nil {
				view.timer.Stop()
			}
		} else if !view.active {
			view.active = true
			view.fetchAndUpdate()
			if view.timer != nil {
				view.timer.Start(10000)
			}
		}
	})

	// Add to main window
	mainWindow.AddDockWidget(qt6.RightDockWidgetArea, view.dock)
	view.dock.Show()

	return view
}

// setupUI creates the UI components
func (v *GroupCounterTodayView) setupUI(title string) {
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
	config.TimeRange = 86400 // 24 hours
	config.MaxPoints = 8640  // 10s intervals over 24h
	v.chart = chart.NewWithConfig(mainWidget, config)
	mainLayout.AddWidget(v.chart.QWidget())

	v.dock.SetWidget(mainWidget)
}

// fetchAndUpdate fetches today's counter data and updates the chart
func (v *GroupCounterTodayView) fetchAndUpdate() {
	members := groupnav.GetManager().GetObjectsByGroup(v.groupName)
	if len(members) == 0 {
		return
	}

	objHashList := &io.ListValue{}
	for hash := range members {
		objHashList.Add(io.NewDecimalValue(int32(hash)))
	}

	servers := server.GetManager().GetConnectedServers()
	if len(servers) == 0 {
		return
	}

	// Clear chart before loading new data
	v.chart.ClearAllSeries()

	for _, srv := range servers {
		session := srv.Session()
		if session == nil {
			continue
		}

		param := pack.NewMapPack()
		param.PutText(protocol.ParamCounter, v.counterName)
		param.Put(protocol.ParamObjHash, objHashList)

		if v.viewMode == "live-daily-total" {
			// Collect all agent data, then sum
			agentData := make(map[int32]map[int64]float64) // objHash -> time -> value

			session.RequestStream(protocol.CMD_COUNTER_TODAY_GROUP, param, func(p pack.Pack) bool {
				mp, ok := p.(*pack.MapPack)
				if !ok {
					return true
				}

				objHash := mp.GetDecimal("objHash")
				timeLv := mp.GetListValue("time")
				valueLv := mp.GetListValue("value")
				if timeLv == nil || valueLv == nil {
					return true
				}

				data := make(map[int64]float64)
				for i := 0; i < timeLv.Size(); i++ {
					ts := timeLv.GetInt64(i)
					val := valueLv.Get(i)
					if val != nil {
						data[ts] = valueToFloat64(val)
					}
				}
				agentData[objHash] = data
				return true
			})

			// Sum all agents by timestamp
			totalByTime := make(map[int64]float64)
			for _, data := range agentData {
				for ts, val := range data {
					totalByTime[ts] += val
				}
			}

			// Sort timestamps and add to chart
			for ts, val := range totalByTime {
				v.chart.AddSeriesPointAt("Total", int64(val), time.UnixMilli(ts))
			}
			v.chart.SetSeriesFill("Total", qt6.NewQColor6("#30649FF0"))
		} else {
			// All mode: per-agent lines
			session.RequestStream(protocol.CMD_COUNTER_TODAY_GROUP, param, func(p pack.Pack) bool {
				mp, ok := p.(*pack.MapPack)
				if !ok {
					return true
				}

				objHash := mp.GetDecimal("objHash")
				timeLv := mp.GetListValue("time")
				valueLv := mp.GetListValue("value")
				if timeLv == nil || valueLv == nil {
					return true
				}

				name := cache.GetObjectCache().GetObjName(objHash)
				if name == "" {
					name = fmt.Sprintf("obj-%d", objHash)
				}

				now := time.Now().UnixMilli()
				for i := 0; i < timeLv.Size(); i++ {
					ts := timeLv.GetInt64(i)
					if ts > now {
						break
					}
					val := valueLv.Get(i)
					if val == nil {
						continue
					}
					v.chart.AddSeriesPointAt(name, int64(valueToFloat64(val)), time.UnixMilli(ts))
				}
				return true
			})
		}
	}

	// Auto-scale Y-axis
	if v.autoScale {
		visibleMax := v.chart.VisibleSeriesMax()
		newMax := ((visibleMax * 12 / 10) / 100) * 100
		if newMax < 100 {
			newMax = 100
		}
		if newMax != v.maxObserved {
			v.maxObserved = newMax
			v.chart.SetMaxValue(newMax)
		}
	}
}

// Dock returns the underlying dock widget
func (v *GroupCounterTodayView) Dock() *qt6.QDockWidget { return v.dock }

// GroupName returns the group name
func (v *GroupCounterTodayView) GroupName() string { return v.groupName }

// ObjType returns the object type
func (v *GroupCounterTodayView) ObjType() string { return v.objType }

// CounterName returns the counter name
func (v *GroupCounterTodayView) CounterName() string { return v.counterName }

// CounterDisplay returns the display name
func (v *GroupCounterTodayView) CounterDisplay() string { return v.counterDisplay }

// ViewMode returns the view mode
func (v *GroupCounterTodayView) ViewMode() string { return v.viewMode }

// ID returns the view ID
func (v *GroupCounterTodayView) ID() int { return v.id }

// Close closes the view and cleans up
func (v *GroupCounterTodayView) Close() {
	v.active = false
	if v.timer != nil {
		v.timer.Stop()
	}
	v.dock.Close()
}
