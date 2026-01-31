package views

import (
	"fmt"
	"time"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/chart"
	"scouter.client.qt/model"
	"scouter.client.qt/protocol"
	"scouter.client.qt/protocol/io"
	"scouter.client.qt/protocol/pack"
	"scouter.client.qt/server"
)

// HostView represents the host monitoring view
type HostView struct {
	dock            *qt6.QDockWidget
	objectNameBytes []byte
	tabWidget       *qt6.QTabWidget

	// Per-host tabs
	hostTabs map[int32]*HostTab

	// Update timer
	timer *qt6.QTimer

	// Counter subscriptions
	subscriptionIDs []int
}

// HostTab represents monitoring widgets for a single host
type HostTab struct {
	objHash   int32
	objName   string
	container *qt6.QWidget

	// Charts
	cpuChart     *chart.Widget
	memoryChart  *chart.Widget
	networkChart *chart.Widget

	// Disk table
	diskTable *qt6.QTableWidget

	// Data storage for rate calculation
	lastNetRxBytes   float64
	lastNetTxBytes   float64
	lastNetTimestamp time.Time

	// CPU components (percentages)
	cpuUser   float64
	cpuSystem float64
	cpuIOWait float64

	// Memory info (bytes)
	memTotal float64
	memUsed  float64
}

// NewHostView creates a new host monitoring view
func NewHostView(mainWindow *qt6.QMainWindow) *HostView {
	v := &HostView{
		hostTabs: make(map[int32]*HostTab),
	}

	// Create dock widget
	v.dock = qt6.NewQDockWidget2("Host Monitoring")
	v.objectNameBytes = []byte("hostMonitoringDock")
	objectNameView := qt6.NewQAnyStringView2(v.objectNameBytes)
	v.dock.SetObjectName(*objectNameView)
	v.dock.SetAllowedAreas(qt6.AllDockWidgetAreas)

	// Create tab widget
	v.tabWidget = qt6.NewQTabWidget2()
	v.tabWidget.SetTabsClosable(true)
	v.tabWidget.OnTabCloseRequested(func(index int) {
		v.closeTab(index)
	})

	v.dock.SetWidget(v.tabWidget.QWidget)

	// Add to main window
	mainWindow.AddDockWidget(qt6.RightDockWidgetArea, v.dock)

	// Setup update timer (every 2 seconds)
	v.timer = qt6.NewQTimer()
	v.timer.OnTimeout(func() {
		v.update()
	})
	v.timer.Start(2000)

	// Subscribe to counters
	v.subscribeCounters()

	return v
}

// Dock returns the dock widget
func (v *HostView) Dock() *qt6.QDockWidget {
	return v.dock
}

// AddHost adds a host to monitor
func (v *HostView) AddHost(objHash int32, objName string) {
	if _, exists := v.hostTabs[objHash]; exists {
		return
	}

	tab := v.createHostTab(objHash, objName)
	v.hostTabs[objHash] = tab

	v.tabWidget.AddTab(tab.container, objName)
}

// RemoveHost removes a host from monitoring
func (v *HostView) RemoveHost(objHash int32) {
	tab, exists := v.hostTabs[objHash]
	if !exists {
		return
	}

	// Find and remove tab
	for i := 0; i < v.tabWidget.Count(); i++ {
		if v.tabWidget.Widget(i) == tab.container {
			v.tabWidget.RemoveTab(i)
			break
		}
	}

	delete(v.hostTabs, objHash)
}

// closeTab handles tab close button
func (v *HostView) closeTab(index int) {
	widget := v.tabWidget.Widget(index)

	// Find and remove corresponding host tab
	for objHash, tab := range v.hostTabs {
		if tab.container == widget {
			delete(v.hostTabs, objHash)
			break
		}
	}

	v.tabWidget.RemoveTab(index)
}

// createHostTab creates monitoring widgets for a host
func (v *HostView) createHostTab(objHash int32, objName string) *HostTab {
	tab := &HostTab{
		objHash:          objHash,
		objName:          objName,
		container:        qt6.NewQWidget2(),
		lastNetTimestamp: time.Now(),
	}

	layout := qt6.NewQVBoxLayout(tab.container)
	layout.SetContentsMargins(4, 4, 4, 4)
	layout.SetSpacing(4)

	// CPU Chart
	cpuConfig := chart.Config{
		MaxPoints:   60,
		MaxValue:    100,
		TimeRange:   60,
		Title:       "CPU Usage (%)",
		MinWidth:    100,
		MinHeight:   80,
		ShowMarkers: false,
		ShowLines:   true,
	}
	tab.cpuChart = chart.NewWithConfig(nil, cpuConfig)
	layout.AddWidget(tab.cpuChart.QWidget())

	// Memory Chart
	memConfig := chart.Config{
		MaxPoints:   60,
		MaxValue:    100,
		TimeRange:   60,
		Title:       "Memory Usage (%)",
		MinWidth:    100,
		MinHeight:   80,
		ShowMarkers: false,
		ShowLines:   true,
	}
	tab.memoryChart = chart.NewWithConfig(nil, memConfig)
	layout.AddWidget(tab.memoryChart.QWidget())

	// Network Chart
	netConfig := chart.Config{
		MaxPoints:   60,
		MaxValue:    10000,
		TimeRange:   60,
		Title:       "Network I/O (KB/s)",
		MinWidth:    100,
		MinHeight:   80,
		ShowMarkers: false,
		ShowLines:   true,
	}
	tab.networkChart = chart.NewWithConfig(nil, netConfig)
	layout.AddWidget(tab.networkChart.QWidget())

	// Disk Usage Table
	diskLabel := qt6.NewQLabel3("Disk Usage")
	layout.AddWidget(diskLabel.QWidget)

	tab.diskTable = qt6.NewQTableWidget2()
	tab.diskTable.SetColumnCount(5)
	tab.diskTable.SetHorizontalHeaderLabels([]string{"Mount", "Total", "Used", "Free", "Usage %"})
	tab.diskTable.HorizontalHeader().SetStretchLastSection(true)
	tab.diskTable.SetEditTriggers(qt6.QAbstractItemView__NoEditTriggers)
	tab.diskTable.SetSelectionBehavior(qt6.QAbstractItemView__SelectRows)
	tab.diskTable.SetMaximumHeight(120)
	layout.AddWidget(tab.diskTable.QWidget)

	return tab
}

// subscribeCounters subscribes to host counters
func (v *HostView) subscribeCounters() {
	engine := model.GetCounterEngine()

	// CPU counters
	subID1 := engine.Subscribe("Host.Cpu.User", 0, v.onCounterUpdate)
	subID2 := engine.Subscribe("Host.Cpu.Sys", 0, v.onCounterUpdate)
	subID3 := engine.Subscribe("Host.Cpu.Wait", 0, v.onCounterUpdate)

	// Memory counters
	subID4 := engine.Subscribe("Host.Mem.Total", 0, v.onCounterUpdate)
	subID5 := engine.Subscribe("Host.Mem.Used", 0, v.onCounterUpdate)

	// Network counters
	subID6 := engine.Subscribe("Host.Net.RxBytes", 0, v.onCounterUpdate)
	subID7 := engine.Subscribe("Host.Net.TxBytes", 0, v.onCounterUpdate)

	v.subscriptionIDs = []int{subID1, subID2, subID3, subID4, subID5, subID6, subID7}
}

// unsubscribeCounters unsubscribes from all counters
func (v *HostView) unsubscribeCounters() {
	engine := model.GetCounterEngine()
	for _, id := range v.subscriptionIDs {
		engine.Unsubscribe(id)
	}
	v.subscriptionIDs = nil
}

// onCounterUpdate handles counter updates
func (v *HostView) onCounterUpdate(objHash int32, counter string, value float64, timestamp time.Time) {
	tab, exists := v.hostTabs[objHash]
	if !exists {
		return
	}

	switch counter {
	case "Host.Cpu.User":
		tab.cpuUser = value
	case "Host.Cpu.Sys":
		tab.cpuSystem = value
	case "Host.Cpu.Wait":
		tab.cpuIOWait = value
	case "Host.Mem.Total":
		tab.memTotal = value
	case "Host.Mem.Used":
		tab.memUsed = value
	case "Host.Net.RxBytes":
		v.updateNetworkRate(tab, "rx", value, timestamp)
	case "Host.Net.TxBytes":
		v.updateNetworkRate(tab, "tx", value, timestamp)
	}
}

// updateNetworkRate calculates network I/O rate
func (v *HostView) updateNetworkRate(tab *HostTab, direction string, bytes float64, timestamp time.Time) {
	if tab.lastNetTimestamp.IsZero() {
		if direction == "rx" {
			tab.lastNetRxBytes = bytes
		} else {
			tab.lastNetTxBytes = bytes
		}
		tab.lastNetTimestamp = timestamp
		return
	}

	duration := timestamp.Sub(tab.lastNetTimestamp).Seconds()
	if duration <= 0 {
		return
	}

	var rate float64
	if direction == "rx" {
		rate = (bytes - tab.lastNetRxBytes) / duration / 1024 // KB/s
		tab.lastNetRxBytes = bytes
	} else {
		rate = (bytes - tab.lastNetTxBytes) / duration / 1024 // KB/s
		tab.lastNetTxBytes = bytes
	}

	tab.lastNetTimestamp = timestamp

	if rate < 0 {
		rate = 0
	}

	// Add to chart
	tab.networkChart.AddPoint(int64(rate))
}

// update fetches and updates all host data
func (v *HostView) update() {
	servers := server.GetManager().GetConnectedServers()
	if len(servers) == 0 {
		return
	}

	// Update CPU and Memory from charts (already handled by counter subscriptions)
	v.updateCPUCharts()
	v.updateMemoryCharts()

	// Update disk usage for each host
	for _, srv := range servers {
		session := srv.Session()
		if session == nil {
			continue
		}

		for objHash := range v.hostTabs {
			v.updateDiskUsage(session, objHash)
		}
	}
}

// updateCPUCharts updates CPU usage charts
func (v *HostView) updateCPUCharts() {
	for _, tab := range v.hostTabs {
		// Total CPU usage (user + system + iowait)
		totalCPU := tab.cpuUser + tab.cpuSystem + tab.cpuIOWait
		if totalCPU > 100 {
			totalCPU = 100
		}
		tab.cpuChart.AddPoint(int64(totalCPU))

		// Update title with current values
		title := fmt.Sprintf("CPU (U:%.1f%% S:%.1f%% W:%.1f%%)", tab.cpuUser, tab.cpuSystem, tab.cpuIOWait)
		tab.cpuChart.SetTitle(title)
	}
}

// updateMemoryCharts updates memory usage charts
func (v *HostView) updateMemoryCharts() {
	for _, tab := range v.hostTabs {
		if tab.memTotal <= 0 {
			continue
		}

		usedPercent := (tab.memUsed / tab.memTotal) * 100
		tab.memoryChart.AddPoint(int64(usedPercent))

		// Update title with current values
		title := fmt.Sprintf("Memory (%.1f / %.1f GB)",
			tab.memUsed/1024/1024/1024,
			tab.memTotal/1024/1024/1024)
		tab.memoryChart.SetTitle(title)
	}
}

// updateDiskUsage fetches and updates disk usage information
func (v *HostView) updateDiskUsage(session interface{}, objHash int32) {
	type sessionInterface interface {
		Request(cmd string, param *pack.MapPack) (*pack.MapPack, error)
	}

	s, ok := session.(sessionInterface)
	if !ok {
		return
	}

	tab, exists := v.hostTabs[objHash]
	if !exists {
		return
	}

	param := pack.NewMapPack()
	param.PutDecimal(protocol.ParamObjHash, objHash)

	resp, err := s.Request(protocol.CMD_HOST_DISK_USAGE, param)
	if err != nil {
		return
	}

	// Parse disk usage response
	v.parseDiskUsage(tab, resp)
}

// parseDiskUsage parses disk usage response and updates table
func (v *HostView) parseDiskUsage(tab *HostTab, resp *pack.MapPack) {
	diskList := resp.GetListValue("disks")
	if diskList == nil {
		return
	}

	count := diskList.Size()
	tab.diskTable.SetRowCount(count)

	for i := 0; i < count; i++ {
		diskVal := diskList.Get(i)
		diskMap, ok := diskVal.(*io.MapValue)
		if !ok {
			continue
		}

		mount := diskMap.GetText("mount")
		total := diskMap.GetDecimal("total")
		used := diskMap.GetDecimal("used")
		free := diskMap.GetDecimal("free")

		var usagePercent float64
		if total > 0 {
			usagePercent = (float64(used) / float64(total)) * 100
		}

		// Set table cells
		tab.diskTable.SetItem(i, 0, qt6.NewQTableWidgetItem2(mount))
		tab.diskTable.SetItem(i, 1, qt6.NewQTableWidgetItem2(formatBytesSize(int64(total))))
		tab.diskTable.SetItem(i, 2, qt6.NewQTableWidgetItem2(formatBytesSize(int64(used))))
		tab.diskTable.SetItem(i, 3, qt6.NewQTableWidgetItem2(formatBytesSize(int64(free))))

		percentCell := qt6.NewQTableWidgetItem2(fmt.Sprintf("%.1f%%", usagePercent))

		// Color-code by threshold
		color := v.getThresholdColor(usagePercent)
		percentCell.SetBackground(qt6.NewQBrush3(color))

		tab.diskTable.SetItem(i, 4, percentCell)
	}
}

// getThresholdColor returns color based on usage threshold
func (v *HostView) getThresholdColor(percent float64) *qt6.QColor {
	if percent < 70 {
		return qt6.NewQColor3(144, 238, 144) // Light green
	} else if percent < 90 {
		return qt6.NewQColor3(255, 255, 153) // Light yellow
	} else {
		return qt6.NewQColor3(255, 153, 153) // Light red
	}
}

// formatBytesSize formats bytes to human-readable string
func formatBytesSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Close cleans up resources
func (v *HostView) Close() {
	if v.timer != nil {
		v.timer.Stop()
	}
	v.unsubscribeCounters()
}
