package xlog

import (
	"fmt"
	"strings"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/cache"
	"scouter.client.qt/net"
	"scouter.client.qt/protocol/pack"
)

// ProfileView displays XLog profile details in a tree view
type ProfileView struct {
	widget    *qt6.QWidget
	tree      *qt6.QTreeWidget
	textCache *cache.TextCache
	proxy     *net.Proxy

	// Current profile
	profile *pack.XLogProfilePack
	txID    int64
}

// NewProfileView creates a new profile detail view
func NewProfileView(parent *qt6.QWidget, proxy *net.Proxy) *ProfileView {
	v := &ProfileView{
		widget:    qt6.NewQWidget(parent),
		textCache: cache.GetTextCache(),
		proxy:     proxy,
	}

	// Layout
	layout := qt6.NewQVBoxLayout(v.widget)
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(4)

	// Toolbar
	toolbar := v.createToolbar()
	layout.AddLayout(toolbar.QLayout)

	// Tree view
	v.tree = qt6.NewQTreeWidget2()
	v.tree.SetColumnCount(4)
	v.tree.SetHeaderLabels([]string{"Step", "Type", "Elapsed", "Details"})
	v.tree.SetAlternatingRowColors(true)
	v.tree.SetSelectionMode(qt6.QAbstractItemView__ExtendedSelection)

	// Set column widths
	header := v.tree.Header()
	header.SetSectionResizeMode2(0, qt6.QHeaderView__Interactive)
	header.SetSectionResizeMode2(1, qt6.QHeaderView__ResizeToContents)
	header.SetSectionResizeMode2(2, qt6.QHeaderView__ResizeToContents)
	header.SetSectionResizeMode2(3, qt6.QHeaderView__Stretch)
	header.ResizeSection(0, 60)

	// Enable context menu
	v.tree.SetContextMenuPolicy(qt6.CustomContextMenu)
	v.tree.OnCustomContextMenuRequested(func(pos *qt6.QPoint) {
		v.showContextMenu(pos)
	})

	layout.AddWidget(v.tree.QWidget)

	v.widget.SetMinimumWidth(600)
	v.widget.SetMinimumHeight(400)

	return v
}

// createToolbar creates the toolbar with action buttons
func (v *ProfileView) createToolbar() *qt6.QHBoxLayout {
	toolbar := qt6.NewQHBoxLayout2()

	// Refresh button
	refreshBtn := qt6.NewQPushButton3("Refresh")
	refreshBtn.OnClicked(func() {
		if v.txID != 0 {
			v.LoadProfile(v.txID)
		}
	})
	toolbar.AddWidget(refreshBtn.QWidget)

	// Copy button
	copyBtn := qt6.NewQPushButton3("Copy Selected")
	copyBtn.OnClicked(func() {
		v.copySelectedToClipboard()
	})
	toolbar.AddWidget(copyBtn.QWidget)

	// Expand/Collapse buttons
	expandBtn := qt6.NewQPushButton3("Expand All")
	expandBtn.OnClicked(func() {
		v.tree.ExpandAll()
	})
	toolbar.AddWidget(expandBtn.QWidget)

	collapseBtn := qt6.NewQPushButton3("Collapse All")
	collapseBtn.OnClicked(func() {
		v.tree.CollapseAll()
	})
	toolbar.AddWidget(collapseBtn.QWidget)

	toolbar.AddStretch()

	return toolbar
}

// QWidget returns the underlying Qt widget
func (v *ProfileView) QWidget() *qt6.QWidget {
	return v.widget
}

// LoadProfile loads and displays a profile for the given transaction ID
func (v *ProfileView) LoadProfile(txID int64) {
	v.txID = txID
	v.tree.Clear()

	if v.proxy == nil {
		v.showError("No proxy connection available")
		return
	}

	// Fetch profile from server
	profile, err := v.proxy.GetXLogProfile(txID)
	if err != nil {
		v.showError(fmt.Sprintf("Failed to load profile: %v", err))
		return
	}

	if profile == nil {
		v.showError("Profile not found")
		return
	}

	v.profile = profile
	v.renderProfile()
}

// showError displays an error message in the tree
func (v *ProfileView) showError(message string) {
	item := qt6.NewQTreeWidgetItem()
	item.SetText(0, "Error")
	item.SetText(3, message)
	item.SetForeground(0, qt6.NewQBrush3(qt6.NewQColor3(220, 50, 50)))
	v.tree.AddTopLevelItem(item)
}

// renderProfile renders the profile data to the tree view
func (v *ProfileView) renderProfile() {
	if v.profile == nil || len(v.profile.Steps) == 0 {
		item := qt6.NewQTreeWidgetItem()
		item.SetText(0, "No profile data")
		v.tree.AddTopLevelItem(item)
		return
	}

	// Render each step
	for i, step := range v.profile.Steps {
		item := v.renderStep(i+1, step)
		if item != nil {
			v.tree.AddTopLevelItem(item)
		}
	}

	// Auto-resize first column
	v.tree.ResizeColumnToContents(0)
}

// renderStep renders a single profile step
func (v *ProfileView) renderStep(stepNum int, step pack.Step) *qt6.QTreeWidgetItem {
	item := qt6.NewQTreeWidgetItem()

	switch s := step.(type) {
	case *pack.MethodStep:
		v.renderMethodStep(item, stepNum, s)
	case *pack.SqlStep:
		v.renderSqlStep(item, stepNum, s)
	case *pack.SqlStep2:
		v.renderSqlStep2(item, stepNum, s)
	case *pack.ApiCallStep:
		v.renderApiCallStep(item, stepNum, s)
	case *pack.ApiCallStep2:
		v.renderApiCallStep2(item, stepNum, s)
	case *pack.SocketStep:
		v.renderSocketStep(item, stepNum, s)
	case *pack.MessageStep:
		v.renderMessageStep(item, stepNum, s)
	case *pack.ErrorStep:
		v.renderErrorStep(item, stepNum, s)
	case *pack.HashedStep:
		v.renderHashedStep(item, stepNum, s)
	default:
		item.SetText(0, fmt.Sprintf("%d", stepNum))
		item.SetText(1, "Unknown")
		item.SetText(3, fmt.Sprintf("Type: %d", step.StepType()))
	}

	return item
}

// renderMethodStep renders a method call step
func (v *ProfileView) renderMethodStep(item *qt6.QTreeWidgetItem, stepNum int, step *pack.MethodStep) {
	methodName := v.textCache.GetMethod(step.Hash)
	if methodName == "" {
		methodName = fmt.Sprintf("method#%d", step.Hash)
	}

	item.SetText(0, fmt.Sprintf("%d", stepNum))
	item.SetText(1, "Method")
	item.SetText(2, formatElapsed(step.Elapsed))
	item.SetText(3, methodName)

	// Gray color for methods
	item.SetForeground(1, qt6.NewQBrush3(qt6.NewQColor3(128, 128, 128)))
}

// renderSqlStep renders a SQL execution step
func (v *ProfileView) renderSqlStep(item *qt6.QTreeWidgetItem, stepNum int, step *pack.SqlStep) {
	sqlText := v.textCache.GetSQL(step.Hash)
	if sqlText == "" {
		sqlText = fmt.Sprintf("sql#%d", step.Hash)
	}

	item.SetText(0, fmt.Sprintf("%d", stepNum))
	item.SetText(1, "SQL")
	item.SetText(2, formatElapsed(step.Elapsed))
	item.SetText(3, v.formatSQL(sqlText))

	// Blue color for SQL
	item.SetForeground(1, qt6.NewQBrush3(qt6.NewQColor3(30, 120, 200)))

	// Add parameter as child if present
	if step.Param != "" {
		paramItem := qt6.NewQTreeWidgetItem()
		paramItem.SetText(1, "Param")
		paramItem.SetText(3, step.Param)
		item.AddChild(paramItem)
	}

	// Add error as child if present
	if step.Error != 0 {
		errorText := v.textCache.GetError(step.Error)
		if errorText == "" {
			errorText = fmt.Sprintf("error#%d", step.Error)
		}
		errorItem := qt6.NewQTreeWidgetItem()
		errorItem.SetText(1, "Error")
		errorItem.SetText(3, errorText)
		errorItem.SetForeground(1, qt6.NewQBrush3(qt6.NewQColor3(220, 50, 50)))
		item.AddChild(errorItem)
	}
}

// renderSqlStep2 renders a SQL execution step with extended info
func (v *ProfileView) renderSqlStep2(item *qt6.QTreeWidgetItem, stepNum int, step *pack.SqlStep2) {
	v.renderSqlStep(item, stepNum, &step.SqlStep)

	// Add bind hash if present
	if step.BindHash != 0 {
		bindItem := qt6.NewQTreeWidgetItem()
		bindItem.SetText(1, "Bind")
		bindItem.SetText(3, fmt.Sprintf("hash#%d", step.BindHash))
		item.AddChild(bindItem)
	}
}

// renderApiCallStep renders an API call step
func (v *ProfileView) renderApiCallStep(item *qt6.QTreeWidgetItem, stepNum int, step *pack.ApiCallStep) {
	apiURL := v.textCache.GetAPICall(step.Hash)
	if apiURL == "" {
		apiURL = fmt.Sprintf("api#%d", step.Hash)
	}

	item.SetText(0, fmt.Sprintf("%d", stepNum))
	item.SetText(1, "API")
	item.SetText(2, formatElapsed(step.Elapsed))
	item.SetText(3, apiURL)

	// Green color for API
	item.SetForeground(1, qt6.NewQBrush3(qt6.NewQColor3(50, 180, 80)))
}

// renderApiCallStep2 renders an API call step with extended info
func (v *ProfileView) renderApiCallStep2(item *qt6.QTreeWidgetItem, stepNum int, step *pack.ApiCallStep2) {
	v.renderApiCallStep(item, stepNum, &step.ApiCallStep)

	// Add transaction ID as child if present
	if step.TxID != 0 {
		txItem := qt6.NewQTreeWidgetItem()
		txItem.SetText(1, "TxID")
		txItem.SetText(3, formatInt64Hex(step.TxID))
		item.AddChild(txItem)
	}

	// Add error as child if present
	if step.Error != 0 {
		errorText := v.textCache.GetError(step.Error)
		if errorText == "" {
			errorText = fmt.Sprintf("error#%d", step.Error)
		}
		errorItem := qt6.NewQTreeWidgetItem()
		errorItem.SetText(1, "Error")
		errorItem.SetText(3, errorText)
		errorItem.SetForeground(1, qt6.NewQBrush3(qt6.NewQColor3(220, 50, 50)))
		item.AddChild(errorItem)
	}
}

// renderSocketStep renders a socket operation step
func (v *ProfileView) renderSocketStep(item *qt6.QTreeWidgetItem, stepNum int, step *pack.SocketStep) {
	ipAddr := formatIPAddr(step.IPAddr)
	socketInfo := fmt.Sprintf("%s:%d", ipAddr, step.Port)

	item.SetText(0, fmt.Sprintf("%d", stepNum))
	item.SetText(1, "Socket")
	item.SetText(2, formatElapsed(step.Elapsed))
	item.SetText(3, socketInfo)

	// Purple color for socket
	item.SetForeground(1, qt6.NewQBrush3(qt6.NewQColor3(150, 80, 200)))

	// Add error as child if present
	if step.Error != 0 {
		errorText := v.textCache.GetError(step.Error)
		if errorText == "" {
			errorText = fmt.Sprintf("error#%d", step.Error)
		}
		errorItem := qt6.NewQTreeWidgetItem()
		errorItem.SetText(1, "Error")
		errorItem.SetText(3, errorText)
		errorItem.SetForeground(1, qt6.NewQBrush3(qt6.NewQColor3(220, 50, 50)))
		item.AddChild(errorItem)
	}
}

// renderMessageStep renders a message/log step
func (v *ProfileView) renderMessageStep(item *qt6.QTreeWidgetItem, stepNum int, step *pack.MessageStep) {
	message := v.textCache.GetMessage(step.Hash)
	if message == "" {
		message = fmt.Sprintf("msg#%d", step.Hash)
	}

	item.SetText(0, fmt.Sprintf("%d", stepNum))
	item.SetText(1, "Message")
	item.SetText(2, formatElapsed(step.Elapsed))
	item.SetText(3, message)

	// Add value as child if present
	if step.Value != "" {
		valueItem := qt6.NewQTreeWidgetItem()
		valueItem.SetText(1, "Value")
		valueItem.SetText(3, step.Value)
		item.AddChild(valueItem)
	}
}

// renderErrorStep renders an error step
func (v *ProfileView) renderErrorStep(item *qt6.QTreeWidgetItem, stepNum int, step *pack.ErrorStep) {
	errorText := v.textCache.GetError(step.Hash)
	if errorText == "" {
		errorText = fmt.Sprintf("error#%d", step.Hash)
	}

	item.SetText(0, fmt.Sprintf("%d", stepNum))
	item.SetText(1, "Error")
	item.SetText(3, errorText)

	// Red color for errors
	item.SetForeground(1, qt6.NewQBrush3(qt6.NewQColor3(220, 50, 50)))

	// Add message as child if present
	if step.Message != "" {
		msgItem := qt6.NewQTreeWidgetItem()
		msgItem.SetText(1, "Details")
		msgItem.SetText(3, step.Message)
		item.AddChild(msgItem)
	}
}

// renderHashedStep renders a generic hashed step
func (v *ProfileView) renderHashedStep(item *qt6.QTreeWidgetItem, stepNum int, step *pack.HashedStep) {
	text := v.textCache.GetMessage(step.Hash)
	if text == "" {
		text = fmt.Sprintf("hash#%d", step.Hash)
	}

	item.SetText(0, fmt.Sprintf("%d", stepNum))
	item.SetText(1, "Hashed")
	item.SetText(2, formatElapsed(step.Time))
	item.SetText(3, text)

	// Add value as child if present
	if step.Value != "" {
		valueItem := qt6.NewQTreeWidgetItem()
		valueItem.SetText(1, "Value")
		valueItem.SetText(3, step.Value)
		item.AddChild(valueItem)
	}
}

// formatSQL formats SQL text with basic cleanup
func (v *ProfileView) formatSQL(sql string) string {
	// Replace multiple spaces with single space
	sql = strings.Join(strings.Fields(sql), " ")

	// Limit length for display
	if len(sql) > 200 {
		sql = sql[:200] + "..."
	}

	return sql
}

// formatIPAddr formats IP address bytes to string
func formatIPAddr(ip []byte) string {
	if len(ip) < 4 {
		return "0.0.0.0"
	}
	return fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3])
}

// copySelectedToClipboard copies selected items to clipboard
func (v *ProfileView) copySelectedToClipboard() {
	selected := v.tree.SelectedItems()
	if len(selected) == 0 {
		return
	}

	var lines []string
	for _, item := range selected {
		line := fmt.Sprintf("%s\t%s\t%s\t%s",
			item.Text(0),
			item.Text(1),
			item.Text(2),
			item.Text(3))
		lines = append(lines, line)
	}

	clipboard := qt6.QGuiApplication_Clipboard()
	clipboard.SetText(strings.Join(lines, "\n"))
}

// showContextMenu shows context menu
func (v *ProfileView) showContextMenu(pos *qt6.QPoint) {
	item := v.tree.ItemAt(pos)
	menu := qt6.NewQMenu2()

	// Copy selected
	copyAction := qt6.NewQAction2("Copy Selected")
	copyAction.OnTriggered(func() {
		v.copySelectedToClipboard()
	})
	menu.AddAction(copyAction)

	if item != nil {
		// Copy details
		copyDetailsAction := qt6.NewQAction2("Copy Details")
		copyDetailsAction.OnTriggered(func() {
			clipboard := qt6.QGuiApplication_Clipboard()
			clipboard.SetText(item.Text(3))
		})
		menu.AddAction(copyDetailsAction)
	}

	menu.AddSeparator()

	// Expand all
	expandAction := qt6.NewQAction2("Expand All")
	expandAction.OnTriggered(func() {
		v.tree.ExpandAll()
	})
	menu.AddAction(expandAction)

	// Collapse all
	collapseAction := qt6.NewQAction2("Collapse All")
	collapseAction.OnTriggered(func() {
		v.tree.CollapseAll()
	})
	menu.AddAction(collapseAction)

	cursorPos := qt6.QCursor_Pos()
	menu.ExecWithPos(cursorPos)
}

