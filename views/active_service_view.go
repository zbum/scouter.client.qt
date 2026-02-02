package views

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/cache"
	"scouter.client.qt/protocol"
	"scouter.client.qt/protocol/io"
	"scouter.client.qt/protocol/pack"
	"scouter.client.qt/qtutil"
	"scouter.client.qt/server"
)

// ActiveService represents an active service/transaction
type ActiveService struct {
	ObjHash     int32
	ThreadID    int64
	Service     int32  // Service hash
	ServiceName string // resolved name (from server for per-agent mode)
	Elapsed     int32  // ms
	CPUTime     int64  // ns
	SQLCount    int32
	APICount    int32
	StartTime   int64
	Name        string // thread name
	State       string // thread state
	IP          string // remote IP
	TxID        string // transaction ID (hex)
}

// ActiveServiceView represents the active service list view
type ActiveServiceView struct {
	dock *qt6.QDockWidget

	// UI components
	tableWidget *qt6.QTableWidget
	filterEdit  *qt6.QLineEdit
	countLabel  *qt6.QLabel
	tableItems  [][]*qt6.QTableWidgetItem // retained references to prevent GC

	// Data
	services   []*ActiveService
	servicesMu sync.RWMutex
	fetching   bool // true while a background fetch is in progress
	dirty      bool // true when new data is available for UI refresh

	// Cache
	textCache   *cache.TextCache
	objectCache *cache.ObjectCache

	// Timer
	timer       *qt6.QTimer
	autoRefresh bool

	// Per-agent mode fields
	objHash  int32  // 0 = all agents, non-zero = specific agent
	objType  string
	serverId int
}

// NewActiveServiceView creates a new active service view
func NewActiveServiceView(mainWindow *qt6.QMainWindow) *ActiveServiceView {
	v := &ActiveServiceView{
		textCache:   cache.GetTextCache(),
		objectCache: cache.GetObjectCache(),
	}

	// Create dock widget
	v.dock = qt6.NewQDockWidget2("Active Service")
	qtutil.SetObjectName(v.dock.QWidget.QObject, "activeServiceDock")
	v.dock.SetAllowedAreas(qt6.AllDockWidgetAreas)

	// Create container
	container := qt6.NewQWidget2()
	layout := qt6.NewQVBoxLayout(container)
	layout.SetContentsMargins(4, 4, 4, 4)
	layout.SetSpacing(4)

	// Control bar
	controlBar := qt6.NewQHBoxLayout2()

	controlFont := qt6.NewQFont()
	controlFont.SetPointSize(10)

	// Filter
	filterLabel := qt6.NewQLabel3("Filter:")
	filterLabel.SetFont(controlFont)
	controlBar.AddWidget(filterLabel.QWidget)

	v.filterEdit = qt6.NewQLineEdit2()
	v.filterEdit.SetPlaceholderText("Service name filter...")
	v.filterEdit.SetFont(controlFont)
	v.filterEdit.OnTextChanged(func(text string) {
		v.refreshTable()
	})
	controlBar.AddWidget(v.filterEdit.QWidget)

	// Refresh button
	refreshBtn := qt6.NewQPushButton3("Refresh")
	refreshBtn.SetFont(controlFont)
	refreshBtn.OnClicked(func() {
		v.startFetch()
	})
	controlBar.AddWidget(refreshBtn.QWidget)

	// Auto-refresh toggle button
	v.autoRefresh = true
	autoRefreshBtn := qt6.NewQPushButton3("Auto")
	autoRefreshBtn.SetFont(controlFont)
	autoRefreshBtn.SetCheckable(true)
	autoRefreshBtn.SetChecked(true)
	autoRefreshBtn.OnToggled(func(checked bool) {
		v.autoRefresh = checked
		if checked {
			v.timer.Start(2000)
		} else {
			v.timer.Stop()
		}
	})
	controlBar.AddWidget(autoRefreshBtn.QWidget)

	controlBar.AddStretch()

	// Count label
	v.countLabel = qt6.NewQLabel3("0 active")
	v.countLabel.SetFont(controlFont)
	controlBar.AddWidget(v.countLabel.QWidget)

	layout.AddLayout(controlBar.QLayout)

	// Table
	v.tableWidget = qt6.NewQTableWidget2()
	v.tableWidget.SetColumnCount(7)
	v.tableWidget.SetHorizontalHeaderLabels([]string{
		"Object", "Thread", "Service", "Elapsed", "CPU", "SQL", "API",
	})
	v.tableWidget.SetSelectionBehavior(qt6.QAbstractItemView__SelectRows)
	v.tableWidget.SetSelectionMode(qt6.QAbstractItemView__SingleSelection)
	v.tableWidget.SetAlternatingRowColors(true)
	v.tableWidget.SetEditTriggers(qt6.QAbstractItemView__NoEditTriggers)
	v.tableWidget.SetSortingEnabled(false)

	// Set column widths
	header := v.tableWidget.HorizontalHeader()
	header.SetSectionResizeMode2(0, qt6.QHeaderView__Interactive) // Object
	header.SetSectionResizeMode2(1, qt6.QHeaderView__Interactive) // Thread
	header.SetSectionResizeMode2(2, qt6.QHeaderView__Stretch)     // Service
	header.SetSectionResizeMode2(3, qt6.QHeaderView__Interactive) // Elapsed
	header.SetSectionResizeMode2(4, qt6.QHeaderView__Interactive) // CPU
	header.SetSectionResizeMode2(5, qt6.QHeaderView__Interactive) // SQL
	header.SetSectionResizeMode2(6, qt6.QHeaderView__Interactive) // API
	header.ResizeSection(0, 120)                                  // Object
	header.ResizeSection(1, 80)                                   // Thread
	header.ResizeSection(3, 80)                                   // Elapsed
	header.ResizeSection(4, 80)                                   // CPU
	header.ResizeSection(5, 60)                                   // SQL
	header.ResizeSection(6, 60)                                   // API

	// Context menu
	v.tableWidget.SetContextMenuPolicy(qt6.CustomContextMenu)
	v.tableWidget.OnCustomContextMenuRequested(func(pos *qt6.QPoint) {
		v.showContextMenu(pos)
	})

	// Double-click for thread dump
	v.tableWidget.OnCellDoubleClicked(func(row, column int) {
		v.handleDoubleClick(row)
	})

	layout.AddWidget(v.tableWidget.QWidget)

	v.dock.SetWidget(container)

	// Add to main window
	mainWindow.AddDockWidget(qt6.BottomDockWidgetArea, v.dock)

	// Setup timer for auto-refresh (2 seconds)
	v.timer = qt6.NewQTimer()
	v.timer.OnTimeout(func() {
		v.tick()
	})
	v.timer.Start(2000)

	// Initial fetch
	v.startFetch()

	return v
}

// NewActiveServiceViewForAgent creates an active service view for a specific agent
func NewActiveServiceViewForAgent(mainWindow *qt6.QMainWindow, objHash int32, objType string, serverId int) *ActiveServiceView {
	v := &ActiveServiceView{
		textCache:   cache.GetTextCache(),
		objectCache: cache.GetObjectCache(),
		objHash:     objHash,
		objType:     objType,
		serverId:    serverId,
	}

	objName := v.objectCache.GetName(objHash)
	if objName == "" {
		objName = fmt.Sprintf("obj#%d", objHash)
	}
	title := fmt.Sprintf("Active Service - %s", objName)

	v.dock = qt6.NewQDockWidget2(title)
	qtutil.SetObjectName(v.dock.QWidget.QObject, fmt.Sprintf("activeServiceDock_%d", objHash))
	v.dock.SetAllowedAreas(qt6.AllDockWidgetAreas)

	container := qt6.NewQWidget2()
	layout := qt6.NewQVBoxLayout(container)
	layout.SetContentsMargins(4, 4, 4, 4)
	layout.SetSpacing(4)

	controlBar := qt6.NewQHBoxLayout2()

	controlFont := qt6.NewQFont()
	controlFont.SetPointSize(10)

	filterLabel := qt6.NewQLabel3("Filter:")
	filterLabel.SetFont(controlFont)
	controlBar.AddWidget(filterLabel.QWidget)

	v.filterEdit = qt6.NewQLineEdit2()
	v.filterEdit.SetPlaceholderText("Service name filter...")
	v.filterEdit.SetFont(controlFont)
	v.filterEdit.OnTextChanged(func(text string) {
		v.refreshTable()
	})
	controlBar.AddWidget(v.filterEdit.QWidget)

	refreshBtn := qt6.NewQPushButton3("Refresh")
	refreshBtn.SetFont(controlFont)
	refreshBtn.OnClicked(func() {
		v.startFetch()
	})
	controlBar.AddWidget(refreshBtn.QWidget)

	// Auto-refresh toggle button
	v.autoRefresh = true
	autoRefreshBtn := qt6.NewQPushButton3("Auto")
	autoRefreshBtn.SetFont(controlFont)
	autoRefreshBtn.SetCheckable(true)
	autoRefreshBtn.SetChecked(true)
	autoRefreshBtn.OnToggled(func(checked bool) {
		v.autoRefresh = checked
		if checked {
			v.timer.Start(2000)
		} else {
			v.timer.Stop()
		}
	})
	controlBar.AddWidget(autoRefreshBtn.QWidget)

	controlBar.AddStretch()

	v.countLabel = qt6.NewQLabel3("0 active")
	v.countLabel.SetFont(controlFont)
	controlBar.AddWidget(v.countLabel.QWidget)

	layout.AddLayout(controlBar.QLayout)

	v.tableWidget = qt6.NewQTableWidget2()
	v.tableWidget.SetColumnCount(7)
	v.tableWidget.SetHorizontalHeaderLabels([]string{
		"Name", "Service", "Elapsed", "IP", "State", "Thread", "CPU",
	})
	v.tableWidget.SetSelectionBehavior(qt6.QAbstractItemView__SelectRows)
	v.tableWidget.SetSelectionMode(qt6.QAbstractItemView__SingleSelection)
	v.tableWidget.SetAlternatingRowColors(true)
	v.tableWidget.SetEditTriggers(qt6.QAbstractItemView__NoEditTriggers)
	v.tableWidget.SetSortingEnabled(false)

	header := v.tableWidget.HorizontalHeader()
	header.SetSectionResizeMode2(0, qt6.QHeaderView__Interactive) // Name
	header.SetSectionResizeMode2(1, qt6.QHeaderView__Stretch)     // Service
	header.SetSectionResizeMode2(2, qt6.QHeaderView__Interactive) // Elapsed
	header.SetSectionResizeMode2(3, qt6.QHeaderView__Interactive) // IP
	header.SetSectionResizeMode2(4, qt6.QHeaderView__Interactive) // State
	header.SetSectionResizeMode2(5, qt6.QHeaderView__Interactive) // Thread
	header.SetSectionResizeMode2(6, qt6.QHeaderView__Interactive) // CPU
	header.ResizeSection(0, 120) // Name
	header.ResizeSection(2, 80)  // Elapsed
	header.ResizeSection(3, 100) // IP
	header.ResizeSection(4, 80)  // State
	header.ResizeSection(5, 80)  // Thread
	header.ResizeSection(6, 80)  // CPU

	v.tableWidget.SetContextMenuPolicy(qt6.CustomContextMenu)
	v.tableWidget.OnCustomContextMenuRequested(func(pos *qt6.QPoint) {
		v.showContextMenu(pos)
	})

	v.tableWidget.OnCellDoubleClicked(func(row, column int) {
		v.handleDoubleClick(row)
	})

	layout.AddWidget(v.tableWidget.QWidget)

	v.dock.SetWidget(container)

	mainWindow.AddDockWidget(qt6.BottomDockWidgetArea, v.dock)

	v.timer = qt6.NewQTimer()
	v.timer.OnTimeout(func() {
		v.tick()
	})
	v.timer.Start(2000)

	// Initial fetch
	v.startFetch()

	v.dock.OnVisibilityChanged(func(visible bool) {
		if !visible {
			if v.timer != nil {
				v.timer.Stop()
			}
		}
	})

	v.startFetch()

	return v
}

// Dock returns the dock widget
func (v *ActiveServiceView) Dock() *qt6.QDockWidget {
	return v.dock
}

// tick is called by the timer on the main thread.
// It refreshes the UI if new data is ready, then kicks off the next background fetch.
func (v *ActiveServiceView) tick() {
	if v.dirty {
		v.dirty = false
		v.refreshTable()
	}
	v.startFetch()
}

// startFetch launches a background goroutine to fetch data (if not already fetching)
func (v *ActiveServiceView) startFetch() {
	v.servicesMu.Lock()
	if v.fetching {
		v.servicesMu.Unlock()
		return
	}
	v.fetching = true
	v.servicesMu.Unlock()

	go v.doFetch()
}

// doFetch fetches active services in the background and stores the result
func (v *ActiveServiceView) doFetch() {
	defer func() {
		v.servicesMu.Lock()
		v.fetching = false
		v.servicesMu.Unlock()
	}()

	start := time.Now()
	servers := server.GetManager().GetConnectedServers()
	if len(servers) == 0 {
		log.Printf("[ActiveService] no connected servers")
		return
	}

	var allServices []*ActiveService

	if v.objHash != 0 {
		// Per-agent mode: use OBJECT_ACTIVE_SERVICE_LIST
		for _, srv := range servers {
			if v.serverId != 0 && srv.ID != v.serverId {
				continue
			}
			session := srv.Session()
			if session == nil {
				continue
			}
			log.Printf("[ActiveService] fetching from server %d for objHash=%d", srv.ID, v.objHash)
			services := v.fetchAgentActiveServices(session)
			log.Printf("[ActiveService] got %d services from server %d (took %v)", len(services), srv.ID, time.Since(start))
			allServices = append(allServices, services...)
		}
	} else {
		for _, srv := range servers {
			session := srv.Session()
			if session == nil {
				continue
			}
			services := v.fetchFromSession(session)
			allServices = append(allServices, services...)
		}
	}

	log.Printf("[ActiveService] total %d services (took %v)", len(allServices), time.Since(start))

	// Keep previous data if the new fetch returned nothing
	if len(allServices) == 0 {
		log.Printf("[ActiveService] keeping previous data (empty response)")
		return
	}

	v.servicesMu.Lock()
	v.services = allServices
	v.dirty = true
	v.servicesMu.Unlock()
}

// fetchFromSession fetches active services from a single session
func (v *ActiveServiceView) fetchFromSession(session interface{}) []*ActiveService {
	type sessionInterface interface {
		RequestStream(cmd string, param *pack.MapPack, callback func(pack.Pack) bool) error
	}

	s, ok := session.(sessionInterface)
	if !ok {
		return nil
	}

	var services []*ActiveService

	param := pack.NewMapPack()
	s.RequestStream(protocol.CMD_ACTIVE_SERVICE_LIST, param, func(p pack.Pack) bool {
		if mp, ok := p.(*pack.MapPack); ok {
			// Parse response - expected format is MapPack with list of active services
			serviceList := mp.GetListValue("services")
			if serviceList == nil {
				return true
			}

			for i := 0; i < serviceList.Size(); i++ {
				val := serviceList.Get(i)
				if svcMap, ok := val.(*io.MapValue); ok {
					svc := &ActiveService{
						ObjHash:   svcMap.GetDecimal("objHash"),
						ThreadID:  svcMap.GetDecimalLong("threadId"),
						Service:   svcMap.GetDecimal("service"),
						Elapsed:   svcMap.GetDecimal("elapsed"),
						CPUTime:   svcMap.GetDecimalLong("cpuTime"),
						SQLCount:  svcMap.GetDecimal("sqlCount"),
						APICount:  svcMap.GetDecimal("apiCount"),
						StartTime: svcMap.GetDecimalLong("startTime"),
					}
					services = append(services, svc)
				}
			}
		}
		return true
	})

	return services
}

// fetchAgentActiveServices fetches active services for a specific agent using OBJECT_ACTIVE_SERVICE_LIST.
// The server returns MapPack(s) with parallel ListValue arrays matching the Java agent response format.
func (v *ActiveServiceView) fetchAgentActiveServices(session interface{}) []*ActiveService {
	type sessionInterface interface {
		RequestStream(cmd string, param *pack.MapPack, callback func(pack.Pack) bool) error
	}

	s, ok := session.(sessionInterface)
	if !ok {
		return nil
	}

	var services []*ActiveService

	param := pack.NewMapPack()
	param.PutDecimal(protocol.ParamObjHash, v.objHash)
	param.PutText(protocol.ParamObjType, v.objType)

	log.Printf("[ActiveService] RequestStream CMD_OBJECT_ACTIVE_SERVICE_LIST objHash=%d objType=%s", v.objHash, v.objType)
	err := s.RequestStream(protocol.CMD_OBJECT_ACTIVE_SERVICE_LIST, param, func(p pack.Pack) bool {
		log.Printf("[ActiveService] callback: pack type=%T", p)
		mp, ok := p.(*pack.MapPack)
		if !ok {
			log.Printf("[ActiveService] callback: not a MapPack, skipping")
			return true
		}

		log.Printf("[ActiveService] callback: MapPack keys=%v", mp.Keys())

		// Server returns MapPack with parallel ListValue arrays
		idLv := mp.GetListValue("id")
		nameLv := mp.GetListValue("name")
		statLv := mp.GetListValue("stat")
		elapsedLv := mp.GetListValue("elapsed")
		serviceLv := mp.GetListValue("service")
		cpuLv := mp.GetListValue("cpu")
		txidLv := mp.GetListValue("txid")
		ipLv := mp.GetListValue("ip")
		sqlLv := mp.GetListValue("sql")
		subcallLv := mp.GetListValue("subcall")

		if idLv == nil {
			return true
		}

		objHash := mp.GetDecimal("objHash")
		if objHash == 0 {
			objHash = v.objHash
		}

		size := idLv.Size()
		for i := 0; i < size; i++ {
			svc := &ActiveService{
				ObjHash:  objHash,
				ThreadID: idLv.GetInt64(i),
				Elapsed:  int32(elapsedLv.GetInt64(i)),
			}
			if nameLv != nil {
				svc.Name = nameLv.GetString(i)
			}
			if statLv != nil {
				svc.State = statLv.GetString(i)
			}
			if serviceLv != nil {
				svc.ServiceName = serviceLv.GetString(i)
			}
			if cpuLv != nil {
				svc.CPUTime = cpuLv.GetInt64(i)
			}
			if txidLv != nil {
				svc.TxID = txidLv.GetString(i)
			}
			if ipLv != nil {
				svc.IP = ipLv.GetString(i)
			}
			if sqlLv != nil {
				svc.SQLCount = int32(sqlLv.GetInt64(i))
			}
			if subcallLv != nil {
				svc.APICount = int32(subcallLv.GetInt64(i))
			}
			services = append(services, svc)
		}
		return true
	})
	if err != nil {
		log.Printf("[ActiveService] RequestStream error: %v", err)
	}
	log.Printf("[ActiveService] RequestStream done, got %d services", len(services))

	return services
}

// refreshTable refreshes the table view
func (v *ActiveServiceView) refreshTable() {
	v.servicesMu.RLock()
	defer v.servicesMu.RUnlock()

	filter := v.filterEdit.Text()

	// Pre-calculate visible services to avoid insert/remove during table update
	type visibleSvc struct {
		svc         *ActiveService
		serviceName string
	}
	var visible []visibleSvc
	for _, svc := range v.services {
		var serviceName string
		if v.objHash != 0 && svc.ServiceName != "" {
			serviceName = svc.ServiceName
		} else {
			serviceName = v.textCache.GetService(svc.Service)
			if serviceName == "" {
				serviceName = fmt.Sprintf("service#%d", svc.Service)
			}
		}

		if filter != "" && !containsIgnoreCase(serviceName, filter) {
			continue
		}
		visible = append(visible, visibleSvc{svc, serviceName})
	}

	log.Printf("[ActiveService] refreshTable: objHash=%d, %d services, %d visible, currentRows=%d",
		v.objHash, len(v.services), len(visible), v.tableWidget.RowCount())

	// Adjust row count without clearing existing items
	currentRows := v.tableWidget.RowCount()
	newRows := len(visible)
	if newRows > currentRows {
		for i := currentRows; i < newRows; i++ {
			v.tableWidget.InsertRow(i)
		}
	} else if newRows < currentRows {
		for i := currentRows - 1; i >= newRows; i-- {
			v.tableWidget.RemoveRow(i)
		}
		// Trim retained references
		if newRows < len(v.tableItems) {
			v.tableItems = v.tableItems[:newRows]
		}
	}

	for i, vs := range visible {
		v.setServiceRow(vs.svc, vs.serviceName, i)
	}

	// Update count label
	if len(visible) == len(v.services) {
		v.countLabel.SetText(fmt.Sprintf("%d active", len(v.services)))
	} else {
		v.countLabel.SetText(fmt.Sprintf("%d/%d active", len(visible), len(v.services)))
	}
}

// ensureItem returns the existing QTableWidgetItem at (row, col) or creates one.
// Retains a Go-side reference to prevent garbage collection.
func (v *ActiveServiceView) ensureItem(row, col int) *qt6.QTableWidgetItem {
	// Grow tableItems if needed
	for len(v.tableItems) <= row {
		v.tableItems = append(v.tableItems, make([]*qt6.QTableWidgetItem, 7))
	}
	item := v.tableItems[row][col]
	if item == nil {
		item = qt6.NewQTableWidgetItem2("")
		v.tableItems[row][col] = item
		v.tableWidget.SetItem(row, col, item)
	}
	return item
}

// setServiceRow sets the data for a specific row (row must already exist via SetRowCount)
func (v *ActiveServiceView) setServiceRow(svc *ActiveService, serviceName string, row int) {
	if v.objHash != 0 {
		// Per-agent mode: Name, Service, Elapsed, IP, State, Thread, CPU
		v.ensureItem(row, 0).SetText(svc.Name)
		v.ensureItem(row, 1).SetText(serviceName)

		elapsedItem := v.ensureItem(row, 2)
		elapsedItem.SetText(formatElapsedMs(svc.Elapsed))
		elapsedItem.SetForeground(qt6.NewQBrush3(v.getElapsedColor(svc.Elapsed)))

		v.ensureItem(row, 3).SetText(svc.IP)
		v.ensureItem(row, 4).SetText(svc.State)
		v.ensureItem(row, 5).SetText(fmt.Sprintf("%d", svc.ThreadID))

		cpuMs := svc.CPUTime / 1000000
		v.ensureItem(row, 6).SetText(fmt.Sprintf("%d ms", cpuMs))
	} else {
		// All-agents mode: Object, Thread, Service, Elapsed, CPU, SQL, API
		objName := v.objectCache.GetName(svc.ObjHash)
		if objName == "" {
			objName = fmt.Sprintf("obj#%d", svc.ObjHash)
		}
		v.ensureItem(row, 0).SetText(objName)
		v.ensureItem(row, 1).SetText(fmt.Sprintf("%d", svc.ThreadID))
		v.ensureItem(row, 2).SetText(serviceName)

		elapsedItem := v.ensureItem(row, 3)
		elapsedItem.SetText(formatElapsedMs(svc.Elapsed))
		elapsedItem.SetForeground(qt6.NewQBrush3(v.getElapsedColor(svc.Elapsed)))

		cpuMs := svc.CPUTime / 1000000
		v.ensureItem(row, 4).SetText(fmt.Sprintf("%d ms", cpuMs))
		v.ensureItem(row, 5).SetText(fmt.Sprintf("%d", svc.SQLCount))
		v.ensureItem(row, 6).SetText(fmt.Sprintf("%d", svc.APICount))
	}
}

// getElapsedColor returns color based on elapsed time
func (v *ActiveServiceView) getElapsedColor(elapsed int32) *qt6.QColor {
	if elapsed < 3000 { // < 3s
		return qt6.NewQColor3(0, 150, 0) // Green
	} else if elapsed < 8000 { // < 8s
		return qt6.NewQColor3(200, 150, 0) // Yellow/Orange
	} else {
		return qt6.NewQColor3(200, 0, 0) // Red
	}
}

// showContextMenu shows context menu
func (v *ActiveServiceView) showContextMenu(pos *qt6.QPoint) {
	item := v.tableWidget.ItemAt(pos)
	if item == nil {
		return
	}

	row := item.Row()
	menu := qt6.NewQMenu2()

	// Thread detail action
	dumpAction := qt6.NewQAction2("Thread Detail")
	dumpAction.OnTriggered(func() {
		v.requestThreadDetail(row)
	})
	menu.AddAction(dumpAction)

	// Stop thread action
	stopAction := qt6.NewQAction2("Stop Thread")
	stopAction.OnTriggered(func() {
		v.requestStopThread(row)
	})
	menu.AddAction(stopAction)

	menu.AddSeparator()

	// Copy service name
	copyAction := qt6.NewQAction2("Copy Service Name")
	copyAction.OnTriggered(func() {
		v.copyServiceName(row)
	})
	menu.AddAction(copyAction)

	cursorPos := qt6.QCursor_Pos()
	menu.ExecWithPos(cursorPos)
}

// handleDoubleClick handles double-click to show thread detail
func (v *ActiveServiceView) handleDoubleClick(row int) {
	v.requestThreadDetail(row)
}

// requestThreadDetail requests thread detail (stack trace) for a service
func (v *ActiveServiceView) requestThreadDetail(row int) {
	v.servicesMu.RLock()
	if row < 0 || row >= len(v.services) {
		v.servicesMu.RUnlock()
		return
	}
	svc := v.services[row]
	v.servicesMu.RUnlock()

	// Convert hex txid to int64
	var txid int64
	if svc.TxID != "" {
		txid = protocol.Hexa32ToLong32(svc.TxID)
	}

	servers := server.GetManager().GetConnectedServers()
	for _, srv := range servers {
		if v.serverId != 0 && srv.ID != v.serverId {
			continue
		}
		session := srv.Session()
		if session == nil {
			continue
		}

		param := pack.NewMapPack()
		param.PutDecimal(protocol.ParamObjHash, svc.ObjHash)
		param.PutDecimalLong("id", svc.ThreadID)
		param.PutDecimalLong("txid", txid)

		resp, err := session.Request(protocol.CMD_OBJECT_THREAD_DETAIL, param)
		if err != nil {
			continue
		}

		v.showThreadDetail(svc, resp)
		return
	}
}

// showThreadDetail shows thread detail in a dialog with metadata table and stack trace
func (v *ActiveServiceView) showThreadDetail(svc *ActiveService, resp *pack.MapPack) {
	threadName := resp.GetText("Thread Name")
	if threadName == "" {
		threadName = fmt.Sprintf("Thread %d", svc.ThreadID)
	}

	dialog := qt6.NewQDialog2()
	dialog.SetWindowTitle(fmt.Sprintf("Thread Detail - %s", threadName))
	dialog.Resize(750, 600)

	layout := qt6.NewQVBoxLayout(dialog.QWidget)

	// Splitter: top = detail table, bottom = stack trace
	splitter := qt6.NewQSplitter3(qt6.Vertical)

	// Detail table
	detailTable := qt6.NewQTableWidget2()
	detailTable.SetColumnCount(2)
	detailTable.SetHorizontalHeaderLabels([]string{"Property", "Value"})
	detailTable.SetEditTriggers(qt6.QAbstractItemView__NoEditTriggers)
	detailTable.SetSelectionBehavior(qt6.QAbstractItemView__SelectRows)
	detailTable.SetAlternatingRowColors(true)
	detailHeader := detailTable.HorizontalHeader()
	detailHeader.SetSectionResizeMode2(0, qt6.QHeaderView__Interactive)
	detailHeader.SetSectionResizeMode2(1, qt6.QHeaderView__Stretch)
	detailHeader.ResizeSection(0, 180)

	// Collect all keys except "Stack Trace" into the table
	stackTrace := ""
	keys := resp.Keys()
	row := 0
	for _, key := range keys {
		if key == "Stack Trace" {
			stackTrace = resp.GetText(key)
			continue
		}
		val := resp.Get(key)
		if val == nil {
			continue
		}
		detailTable.InsertRow(row)
		detailTable.SetItem(row, 0, qt6.NewQTableWidgetItem2(key))
		detailTable.SetItem(row, 1, qt6.NewQTableWidgetItem2(valueToString(val)))
		row++
	}

	splitter.AddWidget(detailTable.QWidget)

	// Stack trace text
	stackEdit := qt6.NewQTextEdit2()
	stackEdit.SetReadOnly(true)
	stackEdit.SetFontFamily("Consolas")
	if stackTrace == "" {
		stackTrace = "No stack trace available"
	}
	stackEdit.SetPlainText(stackTrace)

	splitter.AddWidget(stackEdit.QWidget)
	splitter.SetSizes([]int{200, 400})

	layout.AddWidget(splitter.QWidget)

	closeBtn := qt6.NewQPushButton3("Close")
	closeBtn.OnClicked(func() {
		dialog.Accept()
	})
	layout.AddWidget(closeBtn.QWidget)

	dialog.Exec()
}

// requestStopThread requests to stop a thread
func (v *ActiveServiceView) requestStopThread(row int) {
	v.servicesMu.RLock()
	if row < 0 || row >= len(v.services) {
		v.servicesMu.RUnlock()
		return
	}
	svc := v.services[row]
	v.servicesMu.RUnlock()

	// Confirm before stopping
	result := qt6.QMessageBox_Question(
		nil,
		"Confirm Stop Thread",
		fmt.Sprintf("Are you sure you want to stop thread %d?", svc.ThreadID),
	)

	if result != qt6.QMessageBox__Yes {
		return
	}

	servers := server.GetManager().GetConnectedServers()
	for _, srv := range servers {
		session := srv.Session()
		if session == nil {
			continue
		}

		param := pack.NewMapPack()
		param.PutDecimal(protocol.ParamObjHash, svc.ObjHash)
		param.PutDecimalLong("threadId", svc.ThreadID)

		session.Request(protocol.CMD_THREAD_STOP, param)
		return
	}
}

// copyServiceName copies service name to clipboard
func (v *ActiveServiceView) copyServiceName(row int) {
	v.servicesMu.RLock()
	if row < 0 || row >= len(v.services) {
		v.servicesMu.RUnlock()
		return
	}
	svc := v.services[row]
	v.servicesMu.RUnlock()

	serviceName := v.textCache.GetService(svc.Service)
	if serviceName == "" {
		serviceName = fmt.Sprintf("service#%d", svc.Service)
	}

	clipboard := qt6.QGuiApplication_Clipboard()
	clipboard.SetText(serviceName)
}

// Stop stops the auto-refresh timer
// ObjHash returns the object hash (0 for all-agents mode)
func (v *ActiveServiceView) ObjHash() int32 {
	return v.objHash
}

func (v *ActiveServiceView) Stop() {
	if v.timer != nil {
		v.timer.Stop()
	}
}

// formatElapsedMs formats elapsed time in milliseconds
func formatElapsedMs(ms int32) string {
	if ms < 1000 {
		return fmt.Sprintf("%d ms", ms)
	} else if ms < 60000 {
		return fmt.Sprintf("%.1f s", float64(ms)/1000)
	} else {
		mins := ms / 60000
		secs := (ms % 60000) / 1000
		return fmt.Sprintf("%d:%02d", mins, secs)
	}
}

// valueToString converts an io.Value to a display string
func valueToString(val io.Value) string {
	switch v := val.(type) {
	case *io.TextValue:
		return v.Value
	case *io.DecimalValue:
		return fmt.Sprintf("%d", v.Value)
	case *io.DecimalLongValue:
		return fmt.Sprintf("%d", v.Value)
	case *io.FloatValue:
		return fmt.Sprintf("%f", v.Value)
	case *io.DoubleValue:
		return fmt.Sprintf("%f", v.Value)
	case *io.BooleanValue:
		if v.Value {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	// Simple case-insensitive contains
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			pc := substr[j]
			// Convert to lowercase
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if pc >= 'A' && pc <= 'Z' {
				pc += 32
			}
			if sc != pc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
