package views

import (
	"fmt"
	"sync"

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
	ObjHash   int32
	ThreadID  int64
	Service   int32  // Service hash
	Elapsed   int32  // ms
	CPUTime   int64  // ns
	SQLCount  int32
	APICount  int32
	StartTime int64
}

// ActiveServiceView represents the active service list view
type ActiveServiceView struct {
	dock *qt6.QDockWidget

	// UI components
	tableWidget *qt6.QTableWidget
	filterEdit  *qt6.QLineEdit
	countLabel  *qt6.QLabel

	// Data
	services   []*ActiveService
	servicesMu sync.RWMutex

	// Cache
	textCache   *cache.TextCache
	objectCache *cache.ObjectCache

	// Timer
	timer *qt6.QTimer
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

	// Filter
	filterLabel := qt6.NewQLabel3("Filter:")
	controlBar.AddWidget(filterLabel.QWidget)

	v.filterEdit = qt6.NewQLineEdit2()
	v.filterEdit.SetPlaceholderText("Service name filter...")
	v.filterEdit.OnTextChanged(func(text string) {
		v.refreshTable()
	})
	controlBar.AddWidget(v.filterEdit.QWidget)

	// Refresh button
	refreshBtn := qt6.NewQPushButton3("Refresh")
	refreshBtn.OnClicked(func() {
		v.fetchActiveServices()
	})
	controlBar.AddWidget(refreshBtn.QWidget)

	controlBar.AddStretch()

	// Count label
	v.countLabel = qt6.NewQLabel3("0 active")
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
	v.tableWidget.SetSortingEnabled(true)

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
		v.fetchActiveServices()
	})
	v.timer.Start(2000)

	// Initial fetch
	v.fetchActiveServices()

	return v
}

// Dock returns the dock widget
func (v *ActiveServiceView) Dock() *qt6.QDockWidget {
	return v.dock
}

// fetchActiveServices fetches active services from all connected servers
func (v *ActiveServiceView) fetchActiveServices() {
	servers := server.GetManager().GetConnectedServers()
	if len(servers) == 0 {
		return
	}

	var allServices []*ActiveService

	for _, srv := range servers {
		session := srv.Session()
		if session == nil {
			continue
		}

		services := v.fetchFromSession(session)
		allServices = append(allServices, services...)
	}

	v.servicesMu.Lock()
	v.services = allServices
	v.servicesMu.Unlock()

	// Refresh table
	v.refreshTable()
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

// refreshTable refreshes the table view
func (v *ActiveServiceView) refreshTable() {
	v.servicesMu.RLock()
	defer v.servicesMu.RUnlock()

	filter := v.filterEdit.Text()

	// Clear existing rows
	v.tableWidget.SetRowCount(0)

	visibleCount := 0
	for _, svc := range v.services {
		serviceName := v.textCache.GetService(svc.Service)
		if serviceName == "" {
			serviceName = fmt.Sprintf("service#%d", svc.Service)
		}

		// Apply filter
		if filter != "" && !containsIgnoreCase(serviceName, filter) {
			continue
		}

		v.addServiceToTable(svc, serviceName, visibleCount)
		visibleCount++
	}

	// Update count label
	if visibleCount == len(v.services) {
		v.countLabel.SetText(fmt.Sprintf("%d active", len(v.services)))
	} else {
		v.countLabel.SetText(fmt.Sprintf("%d/%d active", visibleCount, len(v.services)))
	}
}

// addServiceToTable adds a service to the table
func (v *ActiveServiceView) addServiceToTable(svc *ActiveService, serviceName string, row int) {
	v.tableWidget.InsertRow(row)

	// Object name
	objName := v.objectCache.GetName(svc.ObjHash)
	if objName == "" {
		objName = fmt.Sprintf("obj#%d", svc.ObjHash)
	}
	objItem := qt6.NewQTableWidgetItem2(objName)
	v.tableWidget.SetItem(row, 0, objItem)

	// Thread ID
	threadItem := qt6.NewQTableWidgetItem2(fmt.Sprintf("%d", svc.ThreadID))
	v.tableWidget.SetItem(row, 1, threadItem)

	// Service name
	serviceItem := qt6.NewQTableWidgetItem2(serviceName)
	v.tableWidget.SetItem(row, 2, serviceItem)

	// Elapsed time with color coding
	elapsedItem := qt6.NewQTableWidgetItem2(formatElapsedMs(svc.Elapsed))
	elapsedItem.SetForeground(qt6.NewQBrush3(v.getElapsedColor(svc.Elapsed)))
	v.tableWidget.SetItem(row, 3, elapsedItem)

	// CPU time (convert ns to ms)
	cpuMs := svc.CPUTime / 1000000
	cpuItem := qt6.NewQTableWidgetItem2(fmt.Sprintf("%d ms", cpuMs))
	v.tableWidget.SetItem(row, 4, cpuItem)

	// SQL count
	sqlItem := qt6.NewQTableWidgetItem2(fmt.Sprintf("%d", svc.SQLCount))
	v.tableWidget.SetItem(row, 5, sqlItem)

	// API count
	apiItem := qt6.NewQTableWidgetItem2(fmt.Sprintf("%d", svc.APICount))
	v.tableWidget.SetItem(row, 6, apiItem)
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

	// Thread dump action
	dumpAction := qt6.NewQAction2("Thread Dump")
	dumpAction.OnTriggered(func() {
		v.requestThreadDump(row)
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

// handleDoubleClick handles double-click to show thread dump
func (v *ActiveServiceView) handleDoubleClick(row int) {
	v.requestThreadDump(row)
}

// requestThreadDump requests thread dump for a service
func (v *ActiveServiceView) requestThreadDump(row int) {
	v.servicesMu.RLock()
	if row < 0 || row >= len(v.services) {
		v.servicesMu.RUnlock()
		return
	}
	svc := v.services[row]
	v.servicesMu.RUnlock()

	// Find the server and request thread dump
	servers := server.GetManager().GetConnectedServers()
	for _, srv := range servers {
		session := srv.Session()
		if session == nil {
			continue
		}

		param := pack.NewMapPack()
		param.PutDecimal(protocol.ParamObjHash, svc.ObjHash)
		param.PutDecimalLong("threadId", svc.ThreadID)

		resp, err := session.Request(protocol.CMD_THREAD_DUMP, param)
		if err != nil {
			continue
		}

		// Show thread dump in dialog
		v.showThreadDump(svc, resp)
		return
	}
}

// showThreadDump shows thread dump in a dialog
func (v *ActiveServiceView) showThreadDump(svc *ActiveService, resp *pack.MapPack) {
	dialog := qt6.NewQDialog2()
	dialog.SetWindowTitle(fmt.Sprintf("Thread Dump - Thread %d", svc.ThreadID))
	dialog.Resize(700, 500)

	layout := qt6.NewQVBoxLayout(dialog.QWidget)

	textEdit := qt6.NewQTextEdit2()
	textEdit.SetReadOnly(true)
	textEdit.SetFontFamily("Consolas")

	dump := resp.GetText("dump")
	if dump == "" {
		dump = "No thread dump available"
	}
	textEdit.SetPlainText(dump)
	layout.AddWidget(textEdit.QWidget)

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
