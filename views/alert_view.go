package views

import (
	"fmt"
	"sync"
	"time"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/protocol"
	"scouter.client.qt/protocol/pack"
	"scouter.client.qt/qtutil"
	"scouter.client.qt/server"
)

const (
	maxAlerts = 500 // Maximum number of alerts to keep in memory
)

// AlertView represents the Alert dock widget for real-time alerts
type AlertView struct {
	dock *qt6.QDockWidget

	// UI components
	tableWidget   *qt6.QTableWidget
	filterCombo   *qt6.QComboBox
	soundCheckbox *qt6.QCheckBox
	clearButton   *qt6.QPushButton
	countLabel    *qt6.QLabel

	// Alert data
	alerts   []*pack.AlertPack
	alertsMu sync.RWMutex

	// Streaming control
	stopChan    chan struct{}
	isStreaming bool
	streamMu    sync.Mutex

	// Settings
	soundEnabled bool
	filterLevel  byte // 0xFF = show all, otherwise show >= filterLevel
}

// NewAlertView creates a new Alert view
func NewAlertView(mainWindow *qt6.QMainWindow) *AlertView {
	v := &AlertView{
		filterLevel: 0xFF, // Show all by default
		stopChan:    make(chan struct{}),
	}

	// Create dock widget
	v.dock = qt6.NewQDockWidget2("Alerts")
	qtutil.SetObjectName(v.dock.QWidget.QObject, "alertDock")
	v.dock.SetAllowedAreas(qt6.TopDockWidgetArea | qt6.BottomDockWidgetArea)

	// Create container
	container := qt6.NewQWidget2()
	layout := qt6.NewQVBoxLayout(container)
	layout.SetContentsMargins(4, 4, 4, 4)
	layout.SetSpacing(4)

	// Control bar
	controlBar := qt6.NewQHBoxLayout2()

	// Filter by level
	filterLabel := qt6.NewQLabel3("Level:")
	controlBar.AddWidget(filterLabel.QWidget)

	v.filterCombo = qt6.NewQComboBox2()
	v.filterCombo.AddItems([]string{"All", "Fatal", "Error", "Warn", "Info"})
	v.filterCombo.SetCurrentIndex(0)
	v.filterCombo.OnCurrentIndexChanged(func(index int) {
		v.handleFilterChanged(index)
	})
	controlBar.AddWidget(v.filterCombo.QWidget)

	controlBar.AddSpacing(10)

	// Sound notification
	v.soundCheckbox = qt6.NewQCheckBox3("Sound")
	v.soundCheckbox.SetChecked(false)
	v.soundCheckbox.OnToggled(func(checked bool) {
		v.soundEnabled = checked
	})
	controlBar.AddWidget(v.soundCheckbox.QWidget)

	controlBar.AddStretch()

	// Alert count
	v.countLabel = qt6.NewQLabel3("0 alerts")
	controlBar.AddWidget(v.countLabel.QWidget)

	controlBar.AddSpacing(10)

	// Clear button
	v.clearButton = qt6.NewQPushButton3("Clear")
	v.clearButton.OnClicked(func() {
		v.clearAlerts()
	})
	controlBar.AddWidget(v.clearButton.QWidget)

	layout.AddLayout(controlBar.QLayout)

	// Table widget
	v.tableWidget = qt6.NewQTableWidget2()
	v.tableWidget.SetColumnCount(5)
	v.tableWidget.SetHorizontalHeaderLabels([]string{"Time", "Level", "Title", "Message", "Object"})
	v.tableWidget.SetSelectionBehavior(qt6.QAbstractItemView__SelectRows)
	v.tableWidget.SetSelectionMode(qt6.QAbstractItemView__SingleSelection)
	v.tableWidget.SetAlternatingRowColors(true)
	v.tableWidget.SetEditTriggers(qt6.QAbstractItemView__NoEditTriggers)

	// Set column widths
	header := v.tableWidget.HorizontalHeader()
	header.SetSectionResizeMode2(0, qt6.QHeaderView__Interactive) // Time
	header.SetSectionResizeMode2(1, qt6.QHeaderView__Interactive) // Level
	header.SetSectionResizeMode2(2, qt6.QHeaderView__Interactive) // Title
	header.SetSectionResizeMode2(3, qt6.QHeaderView__Stretch)     // Message
	header.SetSectionResizeMode2(4, qt6.QHeaderView__Interactive) // Object
	header.ResizeSection(0, 140)                                  // Time
	header.ResizeSection(1, 60)                                   // Level
	header.ResizeSection(2, 150)                                  // Title
	header.ResizeSection(4, 100)                                  // Object

	// Double-click to show details
	v.tableWidget.OnCellDoubleClicked(func(row, column int) {
		v.handleDoubleClick(row)
	})

	// Context menu
	v.tableWidget.SetContextMenuPolicy(qt6.CustomContextMenu)
	v.tableWidget.OnCustomContextMenuRequested(func(pos *qt6.QPoint) {
		v.showContextMenu(pos)
	})

	layout.AddWidget(v.tableWidget.QWidget)

	v.dock.SetWidget(container)

	// Add to main window
	mainWindow.AddDockWidget(qt6.BottomDockWidgetArea, v.dock)

	// Start streaming from connected servers
	go v.startStreaming()

	return v
}

// Dock returns the dock widget
func (v *AlertView) Dock() *qt6.QDockWidget {
	return v.dock
}

// handleFilterChanged handles filter level change
func (v *AlertView) handleFilterChanged(index int) {
	switch index {
	case 0: // All
		v.filterLevel = 0xFF
	case 1: // Fatal
		v.filterLevel = pack.AlertLevelFatal
	case 2: // Error
		v.filterLevel = pack.AlertLevelError
	case 3: // Warn
		v.filterLevel = pack.AlertLevelWarn
	case 4: // Info
		v.filterLevel = pack.AlertLevelInfo
	default:
		v.filterLevel = 0xFF
	}
	v.refreshTable()
}

// clearAlerts clears all alerts
func (v *AlertView) clearAlerts() {
	v.alertsMu.Lock()
	v.alerts = nil
	v.alertsMu.Unlock()
	v.refreshTable()
}

// addAlert adds a new alert
func (v *AlertView) addAlert(alert *pack.AlertPack) {
	v.alertsMu.Lock()
	// Add to beginning (newest first)
	v.alerts = append([]*pack.AlertPack{alert}, v.alerts...)

	// Trim if exceeds max
	if len(v.alerts) > maxAlerts {
		v.alerts = v.alerts[:maxAlerts]
	}
	v.alertsMu.Unlock()

	// Refresh table (already on callback thread, Qt handles thread safety)
	v.refreshTable()

	// Play sound if enabled and alert is error or fatal
	if v.soundEnabled && alert.IsError() {
		qt6.QApplication_Beep()
	}
}

// refreshTable refreshes the table view
func (v *AlertView) refreshTable() {
	v.alertsMu.RLock()
	defer v.alertsMu.RUnlock()

	// Clear existing rows
	v.tableWidget.SetRowCount(0)

	// Filter and add alerts
	visibleCount := 0
	for _, alert := range v.alerts {
		// Apply filter
		if v.filterLevel != 0xFF {
			if v.filterLevel == pack.AlertLevelFatal && alert.Level != pack.AlertLevelFatal {
				continue
			}
			if v.filterLevel == pack.AlertLevelError && alert.Level < pack.AlertLevelError {
				continue
			}
			if v.filterLevel == pack.AlertLevelWarn && alert.Level < pack.AlertLevelWarn {
				continue
			}
			if v.filterLevel == pack.AlertLevelInfo && alert.Level < pack.AlertLevelInfo {
				continue
			}
		}

		v.addAlertToTable(alert, visibleCount)
		visibleCount++
	}

	// Update count label
	if visibleCount == len(v.alerts) {
		v.countLabel.SetText(fmt.Sprintf("%d alerts", len(v.alerts)))
	} else {
		v.countLabel.SetText(fmt.Sprintf("%d/%d alerts", visibleCount, len(v.alerts)))
	}
}

// addAlertToTable adds an alert to the table
func (v *AlertView) addAlertToTable(alert *pack.AlertPack, row int) {
	v.tableWidget.InsertRow(row)

	// Time
	timeStr := formatAlertTime(alert.Time)
	timeItem := qt6.NewQTableWidgetItem2(timeStr)
	v.tableWidget.SetItem(row, 0, timeItem)

	// Level
	levelStr := alert.LevelName()
	levelItem := qt6.NewQTableWidgetItem2(levelStr)
	levelItem.SetForeground(qt6.NewQBrush3(getLevelColor(alert.Level)))
	v.tableWidget.SetItem(row, 1, levelItem)

	// Title
	titleItem := qt6.NewQTableWidgetItem2(alert.Title)
	v.tableWidget.SetItem(row, 2, titleItem)

	// Message
	msgItem := qt6.NewQTableWidgetItem2(alert.Message)
	v.tableWidget.SetItem(row, 3, msgItem)

	// Object
	objItem := qt6.NewQTableWidgetItem2(alert.ObjType)
	v.tableWidget.SetItem(row, 4, objItem)
}

// handleDoubleClick handles double-click on a row
func (v *AlertView) handleDoubleClick(row int) {
	v.alertsMu.RLock()
	if row < 0 || row >= len(v.alerts) {
		v.alertsMu.RUnlock()
		return
	}
	alert := v.alerts[row]
	v.alertsMu.RUnlock()

	v.showAlertDetails(alert)
}

// showAlertDetails shows a dialog with full alert details
func (v *AlertView) showAlertDetails(alert *pack.AlertPack) {
	dialog := qt6.NewQDialog2()
	dialog.SetWindowTitle("Alert Details")
	dialog.Resize(500, 300)

	layout := qt6.NewQVBoxLayout(dialog.QWidget)

	// Create text edit with alert details
	textEdit := qt6.NewQTextEdit2()
	textEdit.SetReadOnly(true)

	details := fmt.Sprintf(`Time: %s
Level: %s
Object Type: %s
Object Hash: %d
Title: %s
Message: %s
`,
		formatAlertTime(alert.Time),
		alert.LevelName(),
		alert.ObjType,
		alert.ObjHash,
		alert.Title,
		alert.Message,
	)

	textEdit.SetPlainText(details)
	layout.AddWidget(textEdit.QWidget)

	// Close button
	closeBtn := qt6.NewQPushButton3("Close")
	closeBtn.OnClicked(func() {
		dialog.Accept()
	})
	layout.AddWidget(closeBtn.QWidget)

	dialog.Exec()
}

// showContextMenu shows context menu for alert row
func (v *AlertView) showContextMenu(pos *qt6.QPoint) {
	item := v.tableWidget.ItemAt(pos)
	menu := qt6.NewQMenu2()

	if item != nil {
		row := item.Row()

		// Copy alert
		copyAction := qt6.NewQAction2("Copy Alert")
		copyAction.OnTriggered(func() {
			v.copyAlert(row)
		})
		menu.AddAction(copyAction)

		menu.AddSeparator()

		// Show details
		detailsAction := qt6.NewQAction2("Show Details")
		detailsAction.OnTriggered(func() {
			v.handleDoubleClick(row)
		})
		menu.AddAction(detailsAction)
	}

	cursorPos := qt6.QCursor_Pos()
	menu.ExecWithPos(cursorPos)
}

// copyAlert copies alert to clipboard
func (v *AlertView) copyAlert(row int) {
	v.alertsMu.RLock()
	defer v.alertsMu.RUnlock()

	if row < 0 || row >= len(v.alerts) {
		return
	}

	alert := v.alerts[row]
	text := fmt.Sprintf("[%s] %s: %s - %s",
		formatAlertTime(alert.Time),
		alert.LevelName(),
		alert.Title,
		alert.Message,
	)

	clipboard := qt6.QGuiApplication_Clipboard()
	clipboard.SetText(text)
}

// startStreaming starts streaming alerts from connected servers
func (v *AlertView) startStreaming() {
	v.streamMu.Lock()
	if v.isStreaming {
		v.streamMu.Unlock()
		return
	}
	v.isStreaming = true
	v.streamMu.Unlock()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-v.stopChan:
			return
		case <-ticker.C:
			v.subscribeToServers()
		}
	}
}

// subscribeToServers subscribes to alert streams from all connected servers
func (v *AlertView) subscribeToServers() {
	servers := server.GetManager().GetConnectedServers()

	for _, srv := range servers {
		session := srv.Session()
		if session == nil {
			continue
		}

		// Start streaming alerts
		go func(sess interface{}) {
			type sessionInterface interface {
				RequestStream(cmd string, param *pack.MapPack, callback func(pack.Pack) bool) error
			}

			s, ok := sess.(sessionInterface)
			if !ok {
				return
			}

			param := pack.NewMapPack()
			s.RequestStream(protocol.CMD_ALERT_REAL_TIME, param, func(p pack.Pack) bool {
				if alert, ok := p.(*pack.AlertPack); ok {
					v.addAlert(alert)
				}
				return true
			})
		}(session)
	}
}

// Stop stops the alert streaming
func (v *AlertView) Stop() {
	v.streamMu.Lock()
	defer v.streamMu.Unlock()

	if !v.isStreaming {
		return
	}

	close(v.stopChan)
	v.isStreaming = false
}

// formatAlertTime formats a timestamp in milliseconds
func formatAlertTime(ms int64) string {
	t := time.UnixMilli(ms)
	return t.Format("2006-01-02 15:04:05")
}

// getLevelColor returns the color for an alert level
func getLevelColor(level byte) *qt6.QColor {
	switch level {
	case pack.AlertLevelFatal:
		return qt6.NewQColor3(200, 0, 0) // Red
	case pack.AlertLevelError:
		return qt6.NewQColor3(255, 80, 80) // Light Red
	case pack.AlertLevelWarn:
		return qt6.NewQColor3(255, 140, 0) // Orange
	case pack.AlertLevelInfo:
		return qt6.NewQColor3(0, 100, 200) // Blue
	default:
		return qt6.NewQColor3(80, 80, 80) // Gray
	}
}
