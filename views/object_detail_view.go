package views

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/cache"
	"scouter.client.qt/protocol"
	"scouter.client.qt/protocol/pack"
	"scouter.client.qt/server"
)

// ObjectDetailView displays detailed information about an agent object
type ObjectDetailView struct {
	dock          *qt6.QDockWidget
	widget        *qt6.QWidget
	tabWidget     *qt6.QTabWidget
	objectCache   *cache.ObjectCache

	// Current object
	objHash  int32
	objPack  *pack.ObjectPack
	serverId int

	// Properties tab
	propsTable *qt6.QTableWidget

	// Environment tab
	envTable    *qt6.QTableWidget
	envFilter   *qt6.QLineEdit

	// Thread list tab
	threadTable   *qt6.QTableWidget
	threadRefresh *qt6.QPushButton

	// Thread dump tab
	threadDumpText *qt6.QTextEdit

	// Refresh timer
	refreshTimer *qt6.QTimer
}

// NewObjectDetailView creates a new object detail view
func NewObjectDetailView(parent *qt6.QWidget) *ObjectDetailView {
	v := &ObjectDetailView{
		objectCache: cache.GetObjectCache(),
	}

	// Create dock widget
	v.dock = qt6.NewQDockWidget2("Object Details")
	v.widget = qt6.NewQWidget(parent)
	v.dock.SetWidget(v.widget)

	// Main layout
	layout := qt6.NewQVBoxLayout(v.widget)
	layout.SetContentsMargins(4, 4, 4, 4)

	// Create tab widget
	v.tabWidget = qt6.NewQTabWidget2()
	layout.AddWidget(v.tabWidget.QWidget)

	// Create tabs
	v.createPropertiesTab()
	v.createEnvironmentTab()
	v.createThreadListTab()
	v.createThreadDumpTab()

	// Setup refresh timer
	v.refreshTimer = qt6.NewQTimer()
	v.refreshTimer.OnTimeout(func() {
		v.refreshThreadList()
	})

	v.dock.SetMinimumWidth(400)
	v.dock.SetMinimumHeight(300)

	return v
}

// createPropertiesTab creates the properties display tab
func (v *ObjectDetailView) createPropertiesTab() {
	widget := qt6.NewQWidget2()
	layout := qt6.NewQVBoxLayout(widget)

	// Properties table
	v.propsTable = qt6.NewQTableWidget2()
	v.propsTable.SetColumnCount(2)
	v.propsTable.SetHorizontalHeaderLabels([]string{"Property", "Value"})
	v.propsTable.SetAlternatingRowColors(true)
	v.propsTable.SetEditTriggers(qt6.QAbstractItemView__NoEditTriggers)
	v.propsTable.SetSelectionBehavior(qt6.QAbstractItemView__SelectRows)

	header := v.propsTable.HorizontalHeader()
	header.SetSectionResizeMode2(0, qt6.QHeaderView__ResizeToContents)
	header.SetSectionResizeMode2(1, qt6.QHeaderView__Stretch)

	layout.AddWidget(v.propsTable.QWidget)

	v.tabWidget.AddTab(widget, "Properties")
}

// createEnvironmentTab creates the JVM environment tab
func (v *ObjectDetailView) createEnvironmentTab() {
	widget := qt6.NewQWidget2()
	layout := qt6.NewQVBoxLayout(widget)

	// Filter toolbar
	filterLayout := qt6.NewQHBoxLayout2()

	filterLabel := qt6.NewQLabel3("Filter:")
	filterLayout.AddWidget(filterLabel.QWidget)

	v.envFilter = qt6.NewQLineEdit2()
	v.envFilter.SetPlaceholderText("Type to filter...")
	v.envFilter.OnTextChanged(func(text string) {
		v.filterEnvironment(text)
	})
	filterLayout.AddWidget(v.envFilter.QWidget)

	refreshBtn := qt6.NewQPushButton3("Refresh")
	refreshBtn.OnClicked(func() {
		v.loadEnvironment()
	})
	filterLayout.AddWidget(refreshBtn.QWidget)

	layout.AddLayout(filterLayout.QLayout)

	// Environment table
	v.envTable = qt6.NewQTableWidget2()
	v.envTable.SetColumnCount(2)
	v.envTable.SetHorizontalHeaderLabels([]string{"Name", "Value"})
	v.envTable.SetAlternatingRowColors(true)
	v.envTable.SetEditTriggers(qt6.QAbstractItemView__NoEditTriggers)
	v.envTable.SetSelectionBehavior(qt6.QAbstractItemView__SelectRows)

	header := v.envTable.HorizontalHeader()
	header.SetSectionResizeMode2(0, qt6.QHeaderView__Interactive)
	header.SetSectionResizeMode2(1, qt6.QHeaderView__Stretch)
	header.ResizeSection(0, 200)

	// Enable context menu
	v.envTable.SetContextMenuPolicy(qt6.CustomContextMenu)
	v.envTable.OnCustomContextMenuRequested(func(pos *qt6.QPoint) {
		v.showEnvContextMenu(pos)
	})

	layout.AddWidget(v.envTable.QWidget)

	v.tabWidget.AddTab(widget, "Environment")
}

// createThreadListTab creates the thread list tab
func (v *ObjectDetailView) createThreadListTab() {
	widget := qt6.NewQWidget2()
	layout := qt6.NewQVBoxLayout(widget)

	// Toolbar
	toolbar := qt6.NewQHBoxLayout2()

	v.threadRefresh = qt6.NewQPushButton3("Refresh")
	v.threadRefresh.OnClicked(func() {
		v.refreshThreadList()
	})
	toolbar.AddWidget(v.threadRefresh.QWidget)

	autoRefresh := qt6.NewQCheckBox3("Auto-refresh (5s)")
	autoRefresh.OnStateChanged(func(state int) {
		if state == int(qt6.Checked) {
			v.refreshTimer.Start(5000)
		} else {
			v.refreshTimer.Stop()
		}
	})
	toolbar.AddWidget(autoRefresh.QWidget)

	toolbar.AddStretch()

	layout.AddLayout(toolbar.QLayout)

	// Thread table
	v.threadTable = qt6.NewQTableWidget2()
	v.threadTable.SetColumnCount(5)
	v.threadTable.SetHorizontalHeaderLabels([]string{"ID", "Name", "State", "CPU%", "Service"})
	v.threadTable.SetAlternatingRowColors(true)
	v.threadTable.SetEditTriggers(qt6.QAbstractItemView__NoEditTriggers)
	v.threadTable.SetSelectionBehavior(qt6.QAbstractItemView__SelectRows)

	header := v.threadTable.HorizontalHeader()
	header.SetSectionResizeMode2(0, qt6.QHeaderView__ResizeToContents)
	header.SetSectionResizeMode2(1, qt6.QHeaderView__Stretch)
	header.SetSectionResizeMode2(2, qt6.QHeaderView__ResizeToContents)
	header.SetSectionResizeMode2(3, qt6.QHeaderView__ResizeToContents)
	header.SetSectionResizeMode2(4, qt6.QHeaderView__Stretch)

	// Context menu for thread operations
	v.threadTable.SetContextMenuPolicy(qt6.CustomContextMenu)
	v.threadTable.OnCustomContextMenuRequested(func(pos *qt6.QPoint) {
		v.showThreadContextMenu(pos)
	})

	layout.AddWidget(v.threadTable.QWidget)

	v.tabWidget.AddTab(widget, "Threads")
}

// createThreadDumpTab creates the thread dump tab
func (v *ObjectDetailView) createThreadDumpTab() {
	widget := qt6.NewQWidget2()
	layout := qt6.NewQVBoxLayout(widget)

	// Toolbar
	toolbar := qt6.NewQHBoxLayout2()

	dumpBtn := qt6.NewQPushButton3("Get Thread Dump")
	dumpBtn.OnClicked(func() {
		v.getThreadDump()
	})
	toolbar.AddWidget(dumpBtn.QWidget)

	copyBtn := qt6.NewQPushButton3("Copy")
	copyBtn.OnClicked(func() {
		clipboard := qt6.QGuiApplication_Clipboard()
		clipboard.SetText(v.threadDumpText.ToPlainText())
	})
	toolbar.AddWidget(copyBtn.QWidget)

	toolbar.AddStretch()

	layout.AddLayout(toolbar.QLayout)

	// Thread dump text area
	v.threadDumpText = qt6.NewQTextEdit2()
	v.threadDumpText.SetReadOnly(true)
	v.threadDumpText.SetFontFamily("monospace")
	v.threadDumpText.SetLineWrapMode(qt6.QTextEdit__NoWrap)

	layout.AddWidget(v.threadDumpText.QWidget)

	v.tabWidget.AddTab(widget, "Thread Dump")
}

// QDockWidget returns the dock widget
func (v *ObjectDetailView) QDockWidget() *qt6.QDockWidget {
	return v.dock
}

// SetObject sets the object to display
func (v *ObjectDetailView) SetObject(objHash int32, serverId int) {
	v.objHash = objHash
	v.serverId = serverId

	// Get object from cache
	v.objPack = v.objectCache.Get(objHash)
	if v.objPack == nil {
		// Try to refresh cache
		v.objectCache.Refresh()
		v.objPack = v.objectCache.Get(objHash)
	}

	// Update all tabs
	v.updatePropertiesTab()
	v.loadEnvironment()
	v.refreshThreadList()
	v.threadDumpText.Clear()

	// Update dock title
	if v.objPack != nil {
		v.dock.SetWindowTitle(fmt.Sprintf("Object: %s", v.objPack.ExtractShortName()))
	} else {
		v.dock.SetWindowTitle(fmt.Sprintf("Object: #%d", objHash))
	}
}

// updatePropertiesTab updates the properties display
func (v *ObjectDetailView) updatePropertiesTab() {
	v.propsTable.SetRowCount(0)

	if v.objPack == nil {
		return
	}

	// Collect properties
	props := []struct {
		name  string
		value string
	}{
		{"Object Name", v.objPack.ObjName},
		{"Object Type", v.objPack.ObjType},
		{"Object Hash", fmt.Sprintf("%d (0x%08X)", v.objPack.ObjHash, uint32(v.objPack.ObjHash))},
		{"Address", v.objPack.Address},
		{"Version", v.objPack.Version},
		{"Alive", fmt.Sprintf("%v", v.objPack.Alive)},
		{"Last WakeUp", formatTimestamp(v.objPack.WakeUp)},
		{"Short Name", v.objPack.ExtractShortName()},
		{"Family", v.objPack.ExtractObjFamily()},
	}

	// Add custom tags
	if v.objPack.Tags != nil {
		tagKeys := v.objPack.Tags.Keys()
		sort.Strings(tagKeys)
		for _, key := range tagKeys {
			value := v.objPack.Tags.GetText(key)
			props = append(props, struct {
				name  string
				value string
			}{"[Tag] " + key, value})
		}
	}

	v.propsTable.SetRowCount(len(props))

	for i, prop := range props {
		nameItem := qt6.NewQTableWidgetItem2(prop.name)
		valueItem := qt6.NewQTableWidgetItem2(prop.value)

		// Color alive status
		if prop.name == "Alive" {
			if v.objPack.Alive {
				valueItem.SetForeground(qt6.NewQBrush3(qt6.NewQColor3(50, 180, 80)))
			} else {
				valueItem.SetForeground(qt6.NewQBrush3(qt6.NewQColor3(220, 50, 50)))
			}
		}

		v.propsTable.SetItem(i, 0, nameItem)
		v.propsTable.SetItem(i, 1, valueItem)
	}
}

// loadEnvironment loads the JVM environment from server
func (v *ObjectDetailView) loadEnvironment() {
	v.envTable.SetRowCount(0)

	if v.objHash == 0 {
		return
	}

	// Get server
	srv := server.GetManager().GetServer(v.serverId)
	if srv == nil {
		return
	}

	session := srv.Session()
	if session == nil {
		return
	}

	// Request environment data
	param := pack.NewMapPack()
	param.PutDecimal(protocol.ParamObjHash, v.objHash)

	resp, err := session.Request("OBJECT_ENV", param)
	if err != nil || resp == nil {
		return
	}

	// Parse environment entries
	envMap := make(map[string]string)

	// Try to get from response
	if entries := resp.GetText("env"); entries != "" {
		lines := strings.Split(entries, "\n")
		for _, line := range lines {
			if idx := strings.Index(line, "="); idx > 0 {
				key := strings.TrimSpace(line[:idx])
				value := strings.TrimSpace(line[idx+1:])
				envMap[key] = value
			}
		}
	}

	// Also check for map-style response
	keys := resp.Keys()
	for _, key := range keys {
		if key != "env" {
			envMap[key] = resp.GetText(key)
		}
	}

	// Sort keys
	sortedKeys := make([]string, 0, len(envMap))
	for k := range envMap {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	// Populate table
	v.envTable.SetRowCount(len(sortedKeys))
	for i, key := range sortedKeys {
		nameItem := qt6.NewQTableWidgetItem2(key)
		valueItem := qt6.NewQTableWidgetItem2(envMap[key])
		v.envTable.SetItem(i, 0, nameItem)
		v.envTable.SetItem(i, 1, valueItem)
	}
}

// filterEnvironment filters the environment table
func (v *ObjectDetailView) filterEnvironment(filter string) {
	filter = strings.ToLower(filter)

	for row := 0; row < v.envTable.RowCount(); row++ {
		nameItem := v.envTable.Item(row, 0)
		valueItem := v.envTable.Item(row, 1)

		if nameItem == nil || valueItem == nil {
			continue
		}

		name := strings.ToLower(nameItem.Text())
		value := strings.ToLower(valueItem.Text())

		show := filter == "" || strings.Contains(name, filter) || strings.Contains(value, filter)
		v.envTable.SetRowHidden(row, !show)
	}
}

// refreshThreadList refreshes the thread list
func (v *ObjectDetailView) refreshThreadList() {
	v.threadTable.SetRowCount(0)

	if v.objHash == 0 {
		return
	}

	srv := server.GetManager().GetServer(v.serverId)
	if srv == nil {
		return
	}

	session := srv.Session()
	if session == nil {
		return
	}

	// Request thread list
	param := pack.NewMapPack()
	param.PutDecimal(protocol.ParamObjHash, v.objHash)

	resp, err := session.Request(protocol.CMD_ACTIVE_THREAD_LIST, param)
	if err != nil || resp == nil {
		return
	}

	// Parse thread list from response
	// Expected format: MapPack with arrays for id, name, state, cpu, service
	ids := resp.GetListValue("id")
	names := resp.GetListValue("name")
	states := resp.GetListValue("state")
	cpus := resp.GetListValue("cpu")
	services := resp.GetListValue("service")

	if ids == nil {
		return
	}

	count := ids.Size()
	v.threadTable.SetRowCount(count)

	textCache := cache.GetTextCache()

	for i := 0; i < count; i++ {
		// Thread ID
		idItem := qt6.NewQTableWidgetItem2(fmt.Sprintf("%d", ids.GetInt64(i)))
		v.threadTable.SetItem(i, 0, idItem)

		// Thread Name
		nameStr := ""
		if names != nil && i < names.Size() {
			nameStr = names.GetString(i)
		}
		nameItem := qt6.NewQTableWidgetItem2(nameStr)
		v.threadTable.SetItem(i, 1, nameItem)

		// Thread State
		stateStr := ""
		if states != nil && i < states.Size() {
			stateStr = states.GetString(i)
		}
		stateItem := qt6.NewQTableWidgetItem2(stateStr)
		// Color by state
		switch strings.ToUpper(stateStr) {
		case "RUNNABLE":
			stateItem.SetForeground(qt6.NewQBrush3(qt6.NewQColor3(50, 180, 80)))
		case "BLOCKED", "WAITING", "TIMED_WAITING":
			stateItem.SetForeground(qt6.NewQBrush3(qt6.NewQColor3(200, 150, 50)))
		}
		v.threadTable.SetItem(i, 2, stateItem)

		// CPU %
		cpuStr := ""
		if cpus != nil && i < cpus.Size() {
			cpuVal := cpus.GetFloat64(i)
			cpuStr = fmt.Sprintf("%.1f%%", cpuVal)
		}
		cpuItem := qt6.NewQTableWidgetItem2(cpuStr)
		v.threadTable.SetItem(i, 3, cpuItem)

		// Service
		serviceStr := ""
		if services != nil && i < services.Size() {
			serviceHash := services.GetInt32(i)
			if serviceHash != 0 {
				serviceStr = textCache.GetService(serviceHash)
				if serviceStr == "" {
					serviceStr = fmt.Sprintf("svc#%d", serviceHash)
				}
			}
		}
		serviceItem := qt6.NewQTableWidgetItem2(serviceStr)
		v.threadTable.SetItem(i, 4, serviceItem)
	}
}

// getThreadDump retrieves and displays a full thread dump
func (v *ObjectDetailView) getThreadDump() {
	v.threadDumpText.Clear()
	v.threadDumpText.SetPlainText("Loading thread dump...")

	if v.objHash == 0 {
		v.threadDumpText.SetPlainText("No object selected")
		return
	}

	srv := server.GetManager().GetServer(v.serverId)
	if srv == nil {
		v.threadDumpText.SetPlainText("Server not connected")
		return
	}

	session := srv.Session()
	if session == nil {
		v.threadDumpText.SetPlainText("Session not available")
		return
	}

	// Request thread dump
	param := pack.NewMapPack()
	param.PutDecimal(protocol.ParamObjHash, v.objHash)

	resp, err := session.Request(protocol.CMD_THREAD_DUMP, param)
	if err != nil {
		v.threadDumpText.SetPlainText(fmt.Sprintf("Error: %v", err))
		return
	}

	if resp == nil {
		v.threadDumpText.SetPlainText("No response from server")
		return
	}

	// Get thread dump text
	dump := resp.GetText("dump")
	if dump == "" {
		dump = resp.GetText("result")
	}
	if dump == "" {
		dump = resp.GetText("threadDump")
	}

	if dump == "" {
		v.threadDumpText.SetPlainText("Thread dump is empty")
		return
	}

	// Apply syntax highlighting
	v.threadDumpText.SetHtml(v.highlightThreadDump(dump))
}

// highlightThreadDump applies syntax highlighting to thread dump
func (v *ObjectDetailView) highlightThreadDump(dump string) string {
	var result strings.Builder
	result.WriteString("<pre style=\"font-family: monospace; font-size: 11px;\">")

	lines := strings.Split(dump, "\n")
	for _, line := range lines {
		highlightedLine := v.highlightLine(line)
		result.WriteString(highlightedLine)
		result.WriteString("<br>")
	}

	result.WriteString("</pre>")
	return result.String()
}

// highlightLine applies highlighting to a single line
func (v *ObjectDetailView) highlightLine(line string) string {
	// Thread header line (e.g., "Thread-1" State: RUNNABLE)
	if strings.HasPrefix(line, "\"") && strings.Contains(line, "\"") {
		return fmt.Sprintf("<span style=\"color: #0066cc; font-weight: bold;\">%s</span>", escapeHTML(line))
	}

	// Stack trace line
	if strings.HasPrefix(strings.TrimSpace(line), "at ") {
		// Highlight package/class
		return fmt.Sprintf("<span style=\"color: #666666;\">%s</span>", escapeHTML(line))
	}

	// Waiting/Lock info
	if strings.Contains(line, "waiting on") || strings.Contains(line, "locked") {
		return fmt.Sprintf("<span style=\"color: #cc6600;\">%s</span>", escapeHTML(line))
	}

	// State keywords
	if strings.Contains(line, "RUNNABLE") {
		return strings.Replace(escapeHTML(line), "RUNNABLE",
			"<span style=\"color: #00cc00;\">RUNNABLE</span>", 1)
	}
	if strings.Contains(line, "BLOCKED") {
		return strings.Replace(escapeHTML(line), "BLOCKED",
			"<span style=\"color: #cc0000;\">BLOCKED</span>", 1)
	}
	if strings.Contains(line, "WAITING") {
		return strings.Replace(escapeHTML(line), "WAITING",
			"<span style=\"color: #cc6600;\">WAITING</span>", 1)
	}

	return escapeHTML(line)
}

// showEnvContextMenu shows context menu for environment table
func (v *ObjectDetailView) showEnvContextMenu(pos *qt6.QPoint) {
	menu := qt6.NewQMenu2()

	copyAction := qt6.NewQAction2("Copy Value")
	copyAction.OnTriggered(func() {
		row := v.envTable.CurrentRow()
		if row >= 0 {
			item := v.envTable.Item(row, 1)
			if item != nil {
				clipboard := qt6.QGuiApplication_Clipboard()
				clipboard.SetText(item.Text())
			}
		}
	})
	menu.AddAction(copyAction)

	copyAllAction := qt6.NewQAction2("Copy All")
	copyAllAction.OnTriggered(func() {
		var sb strings.Builder
		for row := 0; row < v.envTable.RowCount(); row++ {
			if !v.envTable.IsRowHidden(row) {
				nameItem := v.envTable.Item(row, 0)
				valueItem := v.envTable.Item(row, 1)
				if nameItem != nil && valueItem != nil {
					sb.WriteString(fmt.Sprintf("%s=%s\n", nameItem.Text(), valueItem.Text()))
				}
			}
		}
		clipboard := qt6.QGuiApplication_Clipboard()
		clipboard.SetText(sb.String())
	})
	menu.AddAction(copyAllAction)

	cursorPos := qt6.QCursor_Pos()
	menu.ExecWithPos(cursorPos)
}

// showThreadContextMenu shows context menu for thread table
func (v *ObjectDetailView) showThreadContextMenu(pos *qt6.QPoint) {
	menu := qt6.NewQMenu2()

	// Get selected thread
	row := v.threadTable.CurrentRow()
	if row < 0 {
		return
	}

	idItem := v.threadTable.Item(row, 0)
	if idItem == nil {
		return
	}

	// View detail action
	viewAction := qt6.NewQAction2("View Stack Trace")
	viewAction.OnTriggered(func() {
		v.viewThreadDetail(row)
	})
	menu.AddAction(viewAction)

	menu.AddSeparator()

	// Interrupt thread action
	interruptAction := qt6.NewQAction2("Interrupt Thread")
	interruptAction.OnTriggered(func() {
		v.interruptThread(row)
	})
	menu.AddAction(interruptAction)

	cursorPos := qt6.QCursor_Pos()
	menu.ExecWithPos(cursorPos)
}

// viewThreadDetail shows detailed stack trace for a thread
func (v *ObjectDetailView) viewThreadDetail(row int) {
	idItem := v.threadTable.Item(row, 0)
	if idItem == nil {
		return
	}

	// Switch to thread dump tab and get specific thread info
	v.tabWidget.SetCurrentIndex(3) // Thread Dump tab

	var threadId int64
	fmt.Sscanf(idItem.Text(), "%d", &threadId)

	// For now, get full thread dump - server may support single thread query
	v.getThreadDump()
}

// interruptThread sends interrupt request for a thread
func (v *ObjectDetailView) interruptThread(row int) {
	idItem := v.threadTable.Item(row, 0)
	if idItem == nil {
		return
	}

	var threadId int64
	fmt.Sscanf(idItem.Text(), "%d", &threadId)

	srv := server.GetManager().GetServer(v.serverId)
	if srv == nil {
		return
	}

	session := srv.Session()
	if session == nil {
		return
	}

	param := pack.NewMapPack()
	param.PutDecimal(protocol.ParamObjHash, v.objHash)
	param.PutDecimalLong("threadId", threadId)

	session.Request(protocol.CMD_THREAD_STOP, param)

	// Refresh thread list
	v.refreshThreadList()
}

// Close cleans up resources
func (v *ObjectDetailView) Close() {
	if v.refreshTimer != nil {
		v.refreshTimer.Stop()
	}
}

// formatTimestamp formats a Unix timestamp to string
func formatTimestamp(ts int64) string {
	if ts == 0 {
		return "-"
	}
	t := time.Unix(ts/1000, (ts%1000)*1000000)
	return t.Format("2006-01-02 15:04:05")
}

// escapeHTML escapes HTML special characters
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
