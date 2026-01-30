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

	// Info panel
	infoLabel  *qt6.QLabel
	profileBtn *qt6.QPushButton

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

	// Last selected txID for profile
	lastSelectedTxID int64

	// Callbacks
	onXLogSelected func(xlog *pack.XLogPack)
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
	v := &View{
		textCache:    cache.GetTextCache(),
		groupName:    groupName,
		objType:      objType,
		serverParams: make(map[int]*pack.MapPack),
		active:       true,
	}

	title := fmt.Sprintf("%s - XLog", groupName)
	objectName := fmt.Sprintf("xlogDock_%s", groupName)
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
	rangeCombo.SetCurrentIndex(1) // 1m default
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
	v.chart.SetOnPointSelected(func(txID int64) {
		v.handlePointSelected(txID)
	})
	v.chart.SetOnRangeSelected(func(txIDs []int64) {
		v.handleRangeSelected(txIDs)
	})
	layout.AddWidget(v.chart.QWidget())

	// Info panel with profile button
	infoBar := qt6.NewQHBoxLayout2()

	v.infoLabel = qt6.NewQLabel2()
	v.infoLabel.SetWordWrap(true)
	v.infoLabel.SetMinimumHeight(40)
	v.infoLabel.SetStyleSheet("background-color: #f0f0f0; padding: 4px; border: 1px solid #ccc;")
	infoBar.AddWidget(v.infoLabel.QWidget)

	v.profileBtn = qt6.NewQPushButton3("Profile")
	v.profileBtn.SetEnabled(false)
	v.profileBtn.OnClicked(func() {
		if v.lastSelectedTxID != 0 {
			v.showProfileDialog(v.lastSelectedTxID)
		}
	})
	infoBar.AddWidget(v.profileBtn.QWidget)

	layout.AddLayout(infoBar.QLayout)

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
					v.AddXLog(pk)
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

// AddXLog adds an XLog to the chart
func (v *View) AddXLog(xlog *pack.XLogPack) {
	if v.paused {
		return
	}

	point := XLogPoint{
		EndTime: time.UnixMilli(xlog.EndTime),
		Elapsed: xlog.Elapsed,
		TxID:    xlog.TxID,
		Service: xlog.Service,
		ObjHash: xlog.ObjHash,
		IsError: xlog.IsError(),
	}
	v.chart.AddPoint(point)
}

// SetOnXLogSelected sets callback for XLog selection
func (v *View) SetOnXLogSelected(callback func(xlog *pack.XLogPack)) {
	v.onXLogSelected = callback
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

func (v *View) handlePointSelected(txID int64) {
	v.lastSelectedTxID = txID
	v.profileBtn.SetEnabled(true)

	// Try to fetch full XLog info from any connected server
	info := "TxID: " + formatInt64Hex(txID)

	servers := server.GetManager().GetConnectedServers()
	for _, srv := range servers {
		session := srv.Session()
		if session == nil {
			continue
		}
		proxy := session.Proxy()
		if proxy == nil {
			continue
		}

		xlogPack, err := proxy.GetXLogByTxID(txID)
		if err != nil || xlogPack == nil {
			continue
		}

		// Resolve service name
		serviceName := v.textCache.GetService(xlogPack.Service)
		if serviceName == "" {
			serviceName = fmt.Sprintf("service#%d", xlogPack.Service)
		}

		// Resolve object name
		objName := cache.GetObjectCache().GetObjName(xlogPack.ObjHash)
		if objName == "" {
			objName = fmt.Sprintf("obj#%d", xlogPack.ObjHash)
		}

		info = fmt.Sprintf("TxID: %s | Service: %s | Elapsed: %dms | Object: %s",
			formatInt64Hex(txID), serviceName, xlogPack.Elapsed, objName)

		if v.onXLogSelected != nil {
			v.onXLogSelected(xlogPack)
		}
		break
	}

	v.infoLabel.SetText(info)
}

func (v *View) handleRangeSelected(txIDs []int64) {
	v.lastSelectedTxID = 0
	v.profileBtn.SetEnabled(false)
	count := len(txIDs)
	v.infoLabel.SetText(formatInt(count) + " transactions selected")
}

// showProfileDialog opens a profile dialog for the given transaction
func (v *View) showProfileDialog(txID int64) {
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

	dialog.Show()

	// Load profile
	profileView.LoadProfile(txID)
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

func formatInt(v int) string {
	if v == 0 {
		return "0"
	}
	result := make([]byte, 0, 10)
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		result = append([]byte{byte('0' + v%10)}, result...)
		v /= 10
	}
	if neg {
		result = append([]byte{'-'}, result...)
	}
	return string(result)
}
