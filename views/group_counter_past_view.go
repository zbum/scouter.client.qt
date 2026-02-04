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

// GroupCounterPastView displays historical counter data for a group (load chart)
type GroupCounterPastView struct {
	dock           *qt6.QDockWidget
	chart          *chart.Widget
	id             int
	groupName      string
	counterName    string
	counterDisplay string
	objType        string
	viewMode       string // "load-time-all", "load-time-total", "load-daily-all", "load-daily-total"
	maxObserved    int64
	autoScale      bool

	// Time mode widgets
	sDateEdit *qt6.QDateTimeEdit
	eDateEdit *qt6.QDateTimeEdit

	// Daily mode widgets
	sDatePicker *qt6.QDateTimeEdit
	eDatePicker *qt6.QDateTimeEdit
}

// NewGroupCounterPastView creates a new past counter chart dock widget
func NewGroupCounterPastView(mainWindow *qt6.QMainWindow, id int, groupName, objType, counterName, displayName string, viewMode string) *GroupCounterPastView {
	if viewMode == "" {
		viewMode = "load-time-all"
	}

	modeLabelMap := map[string]string{
		"load-time-all":    " (Load Time All)",
		"load-time-total":  " (Load Time Total)",
		"load-daily-all":   " (Load Daily All)",
		"load-daily-total": " (Load Daily Total)",
	}
	modeLabel := modeLabelMap[viewMode]

	view := &GroupCounterPastView{
		id:             id,
		groupName:      groupName,
		counterName:    counterName,
		counterDisplay: displayName,
		objType:        objType,
		viewMode:       viewMode,
		autoScale:      true,
	}

	title := fmt.Sprintf("%s - %s%s", groupName, displayName, modeLabel)

	// Create dock widget
	view.dock = qt6.NewQDockWidget2(title)
	objectName := fmt.Sprintf("groupCounterPastDock_%d_%s_%s_%s", id, groupName, counterName, viewMode)
	qtutil.SetObjectName(view.dock.QWidget.QObject, objectName)
	view.dock.SetAllowedAreas(qt6.AllDockWidgetAreas)

	view.setupUI(title)

	// Add to main window
	mainWindow.AddDockWidget(qt6.RightDockWidgetArea, view.dock)
	view.dock.Show()

	return view
}

// isTimeMode returns true if this is a time-range mode (vs daily/date mode)
func (v *GroupCounterPastView) isTimeMode() bool {
	return v.viewMode == "load-time-all" || v.viewMode == "load-time-total"
}

// isTotalMode returns true if this should aggregate to a single total line
func (v *GroupCounterPastView) isTotalMode() bool {
	return v.viewMode == "load-time-total" || v.viewMode == "load-daily-total"
}

// setupUI creates the UI components
func (v *GroupCounterPastView) setupUI(title string) {
	mainWidget := qt6.NewQWidget2()
	mainLayout := qt6.NewQVBoxLayout(mainWidget)
	mainLayout.SetContentsMargins(4, 4, 4, 4)
	mainLayout.SetSpacing(4)

	// Toolbar with date/time pickers and query button
	toolbar := qt6.NewQHBoxLayout2()
	toolbar.SetContentsMargins(0, 0, 0, 0)
	toolbar.SetSpacing(8)

	now := time.Now()

	if v.isTimeMode() {
		// Time range mode: datetime pickers
		fromLabel := qt6.NewQLabel3("From:")
		toolbar.AddWidget(fromLabel.QWidget)

		fiveMinAgo := now.Add(-5 * time.Minute)
		sdt := qt6.NewQDateTime2(
			*qt6.NewQDate2(fiveMinAgo.Year(), int(fiveMinAgo.Month()), fiveMinAgo.Day()),
			*qt6.NewQTime4(fiveMinAgo.Hour(), fiveMinAgo.Minute(), fiveMinAgo.Second()))
		v.sDateEdit = qt6.NewQDateTimeEdit3(sdt)
		v.sDateEdit.SetDisplayFormat("yyyy-MM-dd HH:mm")
		v.sDateEdit.SetCalendarPopup(true)
		toolbar.AddWidget(v.sDateEdit.QWidget)

		toLabel := qt6.NewQLabel3("To:")
		toolbar.AddWidget(toLabel.QWidget)

		edt := qt6.NewQDateTime2(
			*qt6.NewQDate2(now.Year(), int(now.Month()), now.Day()),
			*qt6.NewQTime4(now.Hour(), now.Minute(), now.Second()))
		v.eDateEdit = qt6.NewQDateTimeEdit3(edt)
		v.eDateEdit.SetDisplayFormat("yyyy-MM-dd HH:mm")
		v.eDateEdit.SetCalendarPopup(true)
		toolbar.AddWidget(v.eDateEdit.QWidget)
	} else {
		// Daily mode: date pickers
		fromLabel := qt6.NewQLabel3("From:")
		toolbar.AddWidget(fromLabel.QWidget)

		weekAgo := now.AddDate(0, 0, -7)
		v.sDatePicker = qt6.NewQDateTimeEdit3(qt6.NewQDateTime2(
			*qt6.NewQDate2(weekAgo.Year(), int(weekAgo.Month()), weekAgo.Day()),
			*qt6.NewQTime4(0, 0, 0)))
		v.sDatePicker.SetDisplayFormat("yyyy-MM-dd")
		v.sDatePicker.SetCalendarPopup(true)
		toolbar.AddWidget(v.sDatePicker.QWidget)

		toLabel := qt6.NewQLabel3("To:")
		toolbar.AddWidget(toLabel.QWidget)

		v.eDatePicker = qt6.NewQDateTimeEdit3(qt6.NewQDateTime2(
			*qt6.NewQDate2(now.Year(), int(now.Month()), now.Day()),
			*qt6.NewQTime4(0, 0, 0)))
		v.eDatePicker.SetDisplayFormat("yyyy-MM-dd")
		v.eDatePicker.SetCalendarPopup(true)
		toolbar.AddWidget(v.eDatePicker.QWidget)
	}

	// Query button
	queryBtn := qt6.NewQPushButton3("Query")
	queryBtn.OnClicked(func() {
		v.fetchAndUpdate()
	})
	toolbar.AddWidget(queryBtn.QWidget)
	toolbar.AddStretch()

	mainLayout.AddLayout(toolbar.QLayout)

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
	if v.isTimeMode() {
		config.TimeRange = 300
		config.MaxPoints = 150
	} else {
		config.TimeRange = 86400 * 7
		config.MaxPoints = 8640
	}
	v.chart = chart.NewWithConfig(mainWidget, config)
	mainLayout.AddWidget(v.chart.QWidget())

	v.dock.SetWidget(mainWidget)
}

// fetchAndUpdate fetches past counter data and updates the chart
func (v *GroupCounterPastView) fetchAndUpdate() {
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

	// Clear chart before loading
	v.chart.ClearAllSeries()

	if v.isTimeMode() {
		v.loadTimeData(servers, objHashList)
	} else {
		v.loadDailyData(servers, objHashList)
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

// loadTimeData loads time-range data using CMD_COUNTER_PAST_TIME_GROUP
func (v *GroupCounterPastView) loadTimeData(servers []*server.Server, objHashList *io.ListValue) {
	sdt := v.sDateEdit.DateTime()
	edt := v.eDateEdit.DateTime()
	stime := sdt.ToMSecsSinceEpoch()
	etime := edt.ToMSecsSinceEpoch()

	if etime <= stime {
		return
	}

	// Update chart time range
	rangeSec := int((etime - stime) / 1000)
	v.chart.SetTimeRange(rangeSec)

	for _, srv := range servers {
		session := srv.Session()
		if session == nil {
			continue
		}

		param := pack.NewMapPack()
		param.PutDecimalLong(protocol.ParamFromTime, stime)
		param.PutDecimalLong(protocol.ParamToTime, etime)
		param.PutText(protocol.ParamCounter, v.counterName)
		param.Put(protocol.ParamObjHash, objHashList)

		if v.isTotalMode() {
			agentData := make(map[int32]map[int64]float64)

			session.RequestStream(protocol.CMD_COUNTER_PAST_TIME_GROUP, param, func(p pack.Pack) bool {
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

			totalByTime := make(map[int64]float64)
			for _, data := range agentData {
				for ts, val := range data {
					totalByTime[ts] += val
				}
			}
			for ts, val := range totalByTime {
				v.chart.AddSeriesPointAt("Total", int64(val), time.UnixMilli(ts))
			}
			v.chart.SetSeriesFill("Total", qt6.NewQColor6("#30649FF0"))
		} else {
			session.RequestStream(protocol.CMD_COUNTER_PAST_TIME_GROUP, param, func(p pack.Pack) bool {
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
				for i := 0; i < timeLv.Size(); i++ {
					ts := timeLv.GetInt64(i)
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
}

// loadDailyData loads daily data using CMD_COUNTER_PAST_LONGDATE_GROUP
func (v *GroupCounterPastView) loadDailyData(servers []*server.Server, objHashList *io.ListValue) {
	sdt := v.sDatePicker.DateTime()
	edt := v.eDatePicker.DateTime()

	stimeMs := sdt.ToMSecsSinceEpoch()
	// Add full day to end date
	etimeMs := edt.ToMSecsSinceEpoch() + 86400000 - 1

	if etimeMs <= stimeMs {
		return
	}

	// Update chart time range
	rangeSec := int((etimeMs - stimeMs) / 1000)
	v.chart.SetTimeRange(rangeSec)

	for _, srv := range servers {
		session := srv.Session()
		if session == nil {
			continue
		}

		param := pack.NewMapPack()
		param.PutDecimalLong(protocol.ParamFromTime, stimeMs)
		param.PutDecimalLong(protocol.ParamToTime, etimeMs)
		param.PutText(protocol.ParamCounter, v.counterName)
		param.Put(protocol.ParamObjHash, objHashList)

		if v.isTotalMode() {
			agentData := make(map[int32]map[int64]float64)

			session.RequestStream(protocol.CMD_COUNTER_PAST_LONGDATE_GROUP, param, func(p pack.Pack) bool {
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

			totalByTime := make(map[int64]float64)
			for _, data := range agentData {
				for ts, val := range data {
					totalByTime[ts] += val
				}
			}
			for ts, val := range totalByTime {
				v.chart.AddSeriesPointAt("Total", int64(val), time.UnixMilli(ts))
			}
			v.chart.SetSeriesFill("Total", qt6.NewQColor6("#30649FF0"))
		} else {
			session.RequestStream(protocol.CMD_COUNTER_PAST_LONGDATE_GROUP, param, func(p pack.Pack) bool {
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
				for i := 0; i < timeLv.Size(); i++ {
					ts := timeLv.GetInt64(i)
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
}

// Dock returns the underlying dock widget
func (v *GroupCounterPastView) Dock() *qt6.QDockWidget { return v.dock }

// GroupName returns the group name
func (v *GroupCounterPastView) GroupName() string { return v.groupName }

// ObjType returns the object type
func (v *GroupCounterPastView) ObjType() string { return v.objType }

// CounterName returns the counter name
func (v *GroupCounterPastView) CounterName() string { return v.counterName }

// CounterDisplay returns the display name
func (v *GroupCounterPastView) CounterDisplay() string { return v.counterDisplay }

// ViewMode returns the view mode
func (v *GroupCounterPastView) ViewMode() string { return v.viewMode }

// ID returns the view ID
func (v *GroupCounterPastView) ID() int { return v.id }

// Close closes the view and cleans up
func (v *GroupCounterPastView) Close() {
	v.dock.Close()
}
