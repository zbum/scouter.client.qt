package xlog

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/cache"
	"scouter.client.qt/groupnav"
	"scouter.client.qt/net"
	"scouter.client.qt/protocol"
	"scouter.client.qt/protocol/io"
	"scouter.client.qt/protocol/pack"
	"scouter.client.qt/server"
)

// View represents the XLog dock widget
type View struct {
	dock            *qt6.QDockWidget
	objectNameBytes []byte
	chart           *Chart
	id              int

	// Real-time streaming control
	timer   *qt6.QTimer
	paused  bool

	// Group context
	groupName string
	objType   string

	// Per-server cursor state (loop/index for incremental fetch)
	serverParams map[int]*pack.MapPack
	paramsMu     sync.Mutex

	// Fetch timer (2s interval)
	fetchTimer *qt6.QTimer
	active     bool
	fetching   atomic.Bool

	// Text cache for resolving hashes
	textCache *cache.TextCache

}

// NewView creates a new XLog view (standalone, no group context)
func NewView(mainWindow *qt6.QMainWindow) *View {
	v := &View{
		textCache: cache.GetTextCache(),
	}

	v.initUI(mainWindow, "XLog", "xlogDock")

	// Setup timer for flushing repaints and clearing old points (main thread)
	v.timer = qt6.NewQTimer()
	v.timer.OnTimeout(func() {
		v.chart.FlushRepaint()
		if !v.paused {
			v.chart.ClearOldPoints()
		}
	})
	v.timer.Start(500)

	return v
}

// NewGroupXLogView creates a new XLog view for a specific group with real-time fetching
func NewGroupXLogView(mainWindow *qt6.QMainWindow, groupName, objType string) *View {
	return NewGroupXLogViewWithID(mainWindow, 0, groupName, objType)
}

// NewGroupXLogViewWithID creates a new XLog view with a specific ID
func NewGroupXLogViewWithID(mainWindow *qt6.QMainWindow, id int, groupName, objType string) *View {
	v := &View{
		id:           id,
		textCache:    cache.GetTextCache(),
		groupName:    groupName,
		objType:      objType,
		serverParams: make(map[int]*pack.MapPack),
		active:       true,
	}

	title := fmt.Sprintf("%s - XLog", groupName)
	objectName := fmt.Sprintf("xlogDock_%d_%s", id, groupName)
	v.initUI(mainWindow, title, objectName)

	// Setup fetch timer (2s interval)
	v.fetchTimer = qt6.NewQTimer()
	v.fetchTimer.OnTimeout(func() {
		v.fetchAndUpdate()
	})
	v.fetchTimer.Start(2000)

	// Handle dock visibility toggle
	v.dock.OnVisibilityChanged(func(visible bool) {
		if !visible {
			v.active = false
			if v.fetchTimer != nil {
				v.fetchTimer.Stop()
			}
		} else if !v.active {
			v.active = true
			if v.fetchTimer != nil {
				v.fetchTimer.Start(2000)
			}
		}
	})

	// Setup timer for flushing repaints and clearing old points (main thread)
	v.timer = qt6.NewQTimer()
	v.timer.OnTimeout(func() {
		v.chart.FlushRepaint()
		if !v.paused {
			v.chart.ClearOldPoints()
		}
	})
	v.timer.Start(500)

	return v
}

// initUI creates the shared UI components
func (v *View) initUI(mainWindow *qt6.QMainWindow, title, objectName string) {
	// Create dock widget
	v.dock = qt6.NewQDockWidget2(title)
	v.objectNameBytes = []byte(objectName)
	objectNameView := qt6.NewQAnyStringView2(v.objectNameBytes)
	v.dock.SetObjectName(*objectNameView)
	v.dock.SetAllowedAreas(qt6.AllDockWidgetAreas)

	// Create container
	container := qt6.NewQWidget2()
	layout := qt6.NewQVBoxLayout(container)
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(4)

	// Control bar
	controlBar := qt6.NewQHBoxLayout2()

	pauseBtn := qt6.NewQPushButton3("Pause")
	pauseBtn.SetCheckable(true)
	pauseBtn.OnToggled(func(checked bool) {
		v.paused = checked
		if checked {
			pauseBtn.SetText("Resume")
		} else {
			pauseBtn.SetText("Pause")
		}
	})
	controlBar.AddWidget(pauseBtn.QWidget)

	clearBtn := qt6.NewQPushButton3("Clear")
	clearBtn.OnClicked(func() {
		v.chart.Clear()
	})
	controlBar.AddWidget(clearBtn.QWidget)

	controlBar.AddStretch()

	// Max elapsed combo
	maxLabel := qt6.NewQLabel3("Max:")
	controlBar.AddWidget(maxLabel.QWidget)

	maxCombo := qt6.NewQComboBox2()
	maxCombo.AddItems([]string{"1s", "3s", "5s", "10s", "30s"})
	maxCombo.SetCurrentIndex(2) // 5s default
	maxCombo.OnCurrentIndexChanged(func(index int) {
		maxValues := []int32{1000, 3000, 5000, 10000, 30000}
		if index >= 0 && index < len(maxValues) {
			v.chart.SetMaxElapsed(maxValues[index])
		}
	})
	controlBar.AddWidget(maxCombo.QWidget)

	// Time range combo
	rangeLabel := qt6.NewQLabel3("Range:")
	controlBar.AddWidget(rangeLabel.QWidget)

	rangeCombo := qt6.NewQComboBox2()
	rangeCombo.AddItems([]string{"30s", "1m", "3m", "5m", "10m"})
	rangeCombo.SetCurrentIndex(3) // 5m default
	rangeCombo.OnCurrentIndexChanged(func(index int) {
		rangeValues := []int{30, 60, 180, 300, 600}
		if index >= 0 && index < len(rangeValues) {
			v.chart.SetTimeRange(rangeValues[index])
		}
	})
	controlBar.AddWidget(rangeCombo.QWidget)

	layout.AddLayout(controlBar.QLayout)

	// Chart
	v.chart = NewChart(nil)
	v.chart.SetOnPointSelected(func(point XLogPoint) {
		v.handlePointSelected(point)
	})
	v.chart.SetOnRangeSelected(func(points []XLogPoint) {
		v.handleRangeSelected(points)
	})
	layout.AddWidget(v.chart.QWidget())

	v.dock.SetWidget(container)

	// Add to main window
	mainWindow.AddDockWidget(qt6.RightDockWidgetArea, v.dock)
	v.dock.Show()
}

// fetchAndUpdate fetches real-time XLog data from servers for the group.
// Runs the network call in a goroutine to avoid blocking the main thread
// and competing for the shared TCP connection with other operations.
func (v *View) fetchAndUpdate() {
	if v.paused || v.groupName == "" {
		return
	}

	// Prevent overlapping fetches
	if !v.fetching.CompareAndSwap(false, true) {
		return
	}

	// Get group members
	members := groupnav.GetManager().GetObjectsByGroup(v.groupName)
	if len(members) == 0 {
		v.fetching.Store(false)
		return
	}

	// Build objHash list
	objHashList := &io.ListValue{}
	for hash := range members {
		objHashList.Add(io.NewDecimalValue(int32(hash)))
	}

	// Get connected servers
	servers := server.GetManager().GetConnectedServers()
	if len(servers) == 0 {
		v.fetching.Store(false)
		return
	}

	// Snapshot server info for goroutine (only servers with valid sessions)
	type serverInfo struct {
		id      int
		session *net.Session
	}
	var srvList []serverInfo
	for _, srv := range servers {
		s := srv.Session()
		if s != nil && s.Proxy() != nil && s.Proxy().SessionID() != 0 {
			srvList = append(srvList, serverInfo{id: srv.ID, session: s})
		}
	}

	if len(srvList) == 0 {
		v.fetching.Store(false)
		return
	}

	go func() {
		defer v.fetching.Store(false)

		for _, srv := range srvList {
			// Use saved cursor MapPack from server (like Java client)
			// The server's response MapPack becomes the param for the next call
			v.paramsMu.Lock()
			sendParam := v.serverParams[srv.id]
			if sendParam == nil {
				sendParam = pack.NewMapPack()
			}
			v.paramsMu.Unlock()

			// Add/overwrite objHash and limit for this request
			sendParam.Put(protocol.ParamObjHash, objHashList)
			sendParam.PutDecimal("limit", 0)

			err := srv.session.RequestStream(protocol.CMD_TRANX_REAL_TIME_GROUP, sendParam, func(p pack.Pack) bool {
				switch pk := p.(type) {
				case *pack.MapPack:
					// Cursor update - save the server's response as cursor for next call
					v.paramsMu.Lock()
					v.serverParams[srv.id] = pk
					v.paramsMu.Unlock()

				case *pack.XLogPack:
					v.AddXLog(pk, srv.id)
				}
				return true
			})
			if err != nil {
				log.Printf("[XLog] srv=%d error: %v", srv.id, err)
			}
		}
	}()
}

// Dock returns the dock widget
func (v *View) Dock() *qt6.QDockWidget {
	return v.dock
}

// Chart returns the chart widget
func (v *View) Chart() *Chart {
	return v.chart
}

// GroupName returns the group name
func (v *View) GroupName() string {
	return v.groupName
}

// ObjType returns the object type
func (v *View) ObjType() string {
	return v.objType
}

// ID returns the view ID
func (v *View) ID() int {
	return v.id
}

// AddXLog adds an XLog to the chart with server ID for color assignment
func (v *View) AddXLog(xlog *pack.XLogPack, serverID int) {
	if v.paused {
		return
	}

	point := XLogPoint{
		EndTime:      time.UnixMilli(xlog.EndTime),
		Elapsed:      xlog.Elapsed,
		TxID:         xlog.TxID,
		GxID:         xlog.GxID,
		Service:      xlog.Service,
		ObjHash:      xlog.ObjHash,
		ServerID:     serverID,
		IsError:      xlog.IsError(),
		CPU:          xlog.CPU,
		SQLCount:     xlog.SQLCount,
		SQLTime:      xlog.SQLTime,
		APICallCount: xlog.APICallCount,
		APICallTime:  xlog.APICallTime,
		KBytes:       xlog.KBytes,
		IPAddr:       xlog.IPAddr,
		Login:        xlog.Login,
		Desc:         xlog.Desc,
		Error:        xlog.Error,
		UserAgent:    xlog.UserAgent,
		HasDump:      xlog.HasDump,
	}
	v.chart.AddPoint(point)
}


// Close stops timers and cleans up
func (v *View) Close() {
	v.active = false
	if v.fetchTimer != nil {
		v.fetchTimer.Stop()
	}
	if v.timer != nil {
		v.timer.Stop()
	}
	v.dock.Close()
}

func (v *View) handlePointSelected(point XLogPoint) {
	startTimeMs := point.EndTime.UnixMilli() - int64(point.Elapsed)
	v.showProfileDialog(point.TxID, startTimeMs)
}

func (v *View) handleRangeSelected(points []XLogPoint) {
	if len(points) > 0 {
		v.showTransactionListDialog(points)
	}
}

// showTransactionListDialog shows a dialog with the selected transactions
func (v *View) showTransactionListDialog(points []XLogPoint) {
	dialog := qt6.NewQDialog2()
	dialog.SetWindowTitle(fmt.Sprintf("Selected Transactions (%d)", len(points)))
	dialog.Resize(1100, 500)

	layout := qt6.NewQVBoxLayout(dialog.QWidget)

	// Columns matching Java client's default visible columns
	headers := []string{
		"Object", "Elapsed", "Service", "StartTime", "EndTime",
		"CPU", "SQL Count", "SQL Time", "API Count", "API Time",
		"KBytes", "IP", "Login", "Desc", "Error",
		"Dump", "Txid", "Gxid",
	}

	table := qt6.NewQTableWidget2()
	table.SetColumnCount(len(headers))
	table.SetHorizontalHeaderLabels(headers)
	table.SetRowCount(len(points))
	table.SetSelectionBehavior(qt6.QAbstractItemView__SelectRows)
	table.SetEditTriggers(qt6.QAbstractItemView__NoEditTriggers)
	table.SetAlternatingRowColors(true)
	table.SetSortingEnabled(true)

	objCache := cache.GetObjectCache()

	// Error row color
	errorColor := qt6.NewQColor3(220, 50, 50)

	for i, p := range points {
		col := 0
		setItem := func(text string) {
			item := qt6.NewQTableWidgetItem2(text)
			if p.IsError {
				item.SetForeground(qt6.NewQBrush3(errorColor))
			}
			table.SetItem(i, col, item)
			col++
		}
		setNumItem := func(text string) {
			item := qt6.NewQTableWidgetItem2(text)
			item.SetTextAlignment(int(qt6.AlignRight | qt6.AlignVCenter))
			if p.IsError {
				item.SetForeground(qt6.NewQBrush3(errorColor))
			}
			table.SetItem(i, col, item)
			col++
		}

		// Object
		objName := objCache.GetObjName(p.ObjHash)
		if objName == "" {
			objName = fmt.Sprintf("obj#%d", p.ObjHash)
		}
		setItem(objName)

		// Elapsed
		setNumItem(fmt.Sprintf("%d", p.Elapsed))

		// Service
		serviceName := v.textCache.GetService(p.Service)
		if serviceName == "" {
			serviceName = fmt.Sprintf("service#%d", p.Service)
		}
		setItem(serviceName)

		// StartTime (EndTime - Elapsed)
		startTime := p.EndTime.Add(-time.Duration(p.Elapsed) * time.Millisecond)
		setItem(startTime.Format("15:04:05.000"))

		// EndTime
		setItem(p.EndTime.Format("15:04:05.000"))

		// CPU
		setNumItem(fmt.Sprintf("%d", p.CPU))

		// SQL Count
		setNumItem(fmt.Sprintf("%d", p.SQLCount))

		// SQL Time
		setNumItem(fmt.Sprintf("%d", p.SQLTime))

		// API Count
		setNumItem(fmt.Sprintf("%d", p.APICallCount))

		// API Time
		setNumItem(fmt.Sprintf("%d", p.APICallTime))

		// KBytes
		setNumItem(fmt.Sprintf("%d", p.KBytes))

		// IP
		setItem(formatIPAddr(p.IPAddr))

		// Login
		loginText := ""
		if p.Login != 0 {
			loginText = v.textCache.GetLogin(p.Login)
		}
		setItem(loginText)

		// Desc
		descText := ""
		if p.Desc != 0 {
			descText = v.textCache.GetDesc(p.Desc)
		}
		setItem(descText)

		// Error
		errorText := ""
		if p.Error != 0 {
			errorText = v.textCache.GetError(p.Error)
			if errorText == "" {
				errorText = "Error"
			}
		}
		setItem(errorText)

		// Dump
		dumpText := ""
		if p.HasDump != 0 {
			dumpText = "Y"
		}
		setItem(dumpText)

		// Txid
		setItem(protocol.Hexa32ToString32(p.TxID))

		// Gxid
		gxidText := ""
		if p.GxID != 0 {
			gxidText = protocol.Hexa32ToString32(p.GxID)
		}
		setItem(gxidText)
	}

	// Resize columns to contents
	table.ResizeColumnsToContents()
	header := table.HorizontalHeader()
	header.SetStretchLastSection(true)

	layout.AddWidget(table.QWidget)

	// Double-click to open profile
	table.OnCellDoubleClicked(func(row int, column int) {
		if row >= 0 && row < len(points) {
			p := points[row]
			startTimeMs := p.EndTime.UnixMilli() - int64(p.Elapsed)
			v.showProfileDialog(p.TxID, startTimeMs)
		}
	})

	dialog.Show()
}

// showProfileDialog opens a profile dialog for the given transaction
func (v *View) showProfileDialog(txID int64, startTimeMs int64) {
	// Find a connected server proxy for profile fetching
	var proxy *net.Proxy
	servers := server.GetManager().GetConnectedServers()
	for _, srv := range servers {
		session := srv.Session()
		if session != nil && session.Proxy() != nil {
			proxy = session.Proxy()
			break
		}
	}
	if proxy == nil {
		return
	}

	// Create profile dialog
	dialog := qt6.NewQDialog2()
	dialog.SetWindowTitle(fmt.Sprintf("Profile - %s", formatInt64Hex(txID)))
	dialog.Resize(800, 600)

	layout := qt6.NewQVBoxLayout(dialog.QWidget)

	profileView := NewProfileView(nil, proxy)
	layout.AddWidget(profileView.QWidget())

	// Load profile before showing dialog
	profileView.LoadProfileWithTime(txID, startTimeMs)

	dialog.Exec()
}

func formatInt64Hex(v int64) string {
	return "0x" + formatHex(uint64(v))
}

func formatHex(v uint64) string {
	const hexDigits = "0123456789ABCDEF"
	if v == 0 {
		return "0"
	}
	result := make([]byte, 0, 16)
	for v > 0 {
		result = append([]byte{hexDigits[v%16]}, result...)
		v /= 16
	}
	return string(result)
}

