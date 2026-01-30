package views

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/cache"
	"scouter.client.qt/protocol/pack"
	"scouter.client.qt/server"
)

// ObjectSelectedCallback is called when an object is selected
type ObjectSelectedCallback func(objHash int32, serverId int)

// ObjectNavView displays hierarchical object navigation tree
type ObjectNavView struct {
	dock         *qt6.QDockWidget
	widget       *qt6.QWidget
	tree         *qt6.QTreeWidget
	searchEdit   *qt6.QLineEdit
	objectCache  *cache.ObjectCache

	// Refresh timer
	refreshTimer *qt6.QTimer
	refreshMu    sync.Mutex

	// Callbacks
	onObjectSelected ObjectSelectedCallback

	// Track expanded state
	expandedNodes map[string]bool
}

// NewObjectNavView creates a new object navigation view
func NewObjectNavView(parent *qt6.QWidget) *ObjectNavView {
	v := &ObjectNavView{
		objectCache:   cache.GetObjectCache(),
		expandedNodes: make(map[string]bool),
	}

	// Create dock widget
	v.dock = qt6.NewQDockWidget2("Objects")
	v.widget = qt6.NewQWidget(parent)
	v.dock.SetWidget(v.widget)

	// Main layout
	layout := qt6.NewQVBoxLayout(v.widget)
	layout.SetContentsMargins(4, 4, 4, 4)
	layout.SetSpacing(4)

	// Search bar
	v.searchEdit = qt6.NewQLineEdit2()
	v.searchEdit.SetPlaceholderText("Search objects...")
	v.searchEdit.OnTextChanged(func(text string) {
		v.filterObjects(text)
	})
	layout.AddWidget(v.searchEdit.QWidget)

	// Tree widget
	v.tree = qt6.NewQTreeWidget2()
	v.tree.SetColumnCount(2)
	v.tree.SetHeaderLabels([]string{"Object", "Status"})
	v.tree.SetAlternatingRowColors(true)
	v.tree.SetAnimated(true)

	// Column sizing
	header := v.tree.Header()
	header.SetSectionResizeMode2(0, qt6.QHeaderView__Stretch)
	header.SetSectionResizeMode2(1, qt6.QHeaderView__ResizeToContents)

	// Selection handling
	v.tree.OnItemClicked(func(item *qt6.QTreeWidgetItem, column int) {
		v.onItemSelected(item)
	})

	v.tree.OnItemDoubleClicked(func(item *qt6.QTreeWidgetItem, column int) {
		v.onItemDoubleClicked(item)
	})

	// Context menu
	v.tree.SetContextMenuPolicy(qt6.CustomContextMenu)
	v.tree.OnCustomContextMenuRequested(func(pos *qt6.QPoint) {
		v.showContextMenu(pos)
	})

	layout.AddWidget(v.tree.QWidget)

	// Toolbar
	toolbar := qt6.NewQHBoxLayout2()

	refreshBtn := qt6.NewQPushButton3("Refresh")
	refreshBtn.OnClicked(func() {
		v.Refresh()
	})
	toolbar.AddWidget(refreshBtn.QWidget)

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

	layout.AddLayout(toolbar.QLayout)

	// Setup auto-refresh timer
	v.refreshTimer = qt6.NewQTimer()
	v.refreshTimer.OnTimeout(func() {
		v.Refresh()
	})
	v.refreshTimer.Start(5000) // Refresh every 5 seconds

	// Initial load
	v.Refresh()

	v.dock.SetMinimumWidth(250)

	return v
}

// QDockWidget returns the dock widget
func (v *ObjectNavView) QDockWidget() *qt6.QDockWidget {
	return v.dock
}

// SetOnObjectSelected sets the callback for object selection
func (v *ObjectNavView) SetOnObjectSelected(callback ObjectSelectedCallback) {
	v.onObjectSelected = callback
}

// Refresh refreshes the object tree
func (v *ObjectNavView) Refresh() {
	v.refreshMu.Lock()
	defer v.refreshMu.Unlock()

	// Save expanded state
	v.saveExpandedState(nil)

	// Clear tree
	v.tree.Clear()

	// Refresh cache
	v.objectCache.Refresh()

	// Build hierarchy
	v.buildHierarchy()

	// Restore expanded state
	v.restoreExpandedState(nil)
}

// buildHierarchy builds the object tree hierarchy
func (v *ObjectNavView) buildHierarchy() {
	servers := server.GetManager().GetConnectedServers()

	for _, srv := range servers {
		// Create server node
		serverItem := qt6.NewQTreeWidgetItem()
		serverItem.SetText(0, srv.Name)
		serverItem.SetText(1, "Server")
		serverItem.SetData(0, int(qt6.UserRole), qt6.NewQVariant4(-srv.ID))

		// Make server node bold
		font := serverItem.Font(0)
		font.SetBold(true)
		serverItem.SetFont(0, font)

		v.tree.AddTopLevelItem(serverItem)

		// Get objects for this server
		objects := v.getObjectsForServer(srv.ID)

		// Group by type
		typeGroups := make(map[string][]*pack.ObjectPack)
		for _, obj := range objects {
			typeGroups[obj.ObjType] = append(typeGroups[obj.ObjType], obj)
		}

		// Sort type names
		typeNames := make([]string, 0, len(typeGroups))
		for t := range typeGroups {
			typeNames = append(typeNames, t)
		}
		sort.Strings(typeNames)

		// Create type group nodes
		for _, typeName := range typeNames {
			typeObjects := typeGroups[typeName]

			typeItem := qt6.NewQTreeWidgetItem()
			typeItem.SetText(0, fmt.Sprintf("%s (%d)", typeName, len(typeObjects)))
			typeItem.SetText(1, "Type")

			// Sort objects by name
			sort.Slice(typeObjects, func(i, j int) bool {
				return typeObjects[i].ObjName < typeObjects[j].ObjName
			})

			// Create object nodes
			for _, obj := range typeObjects {
				objItem := qt6.NewQTreeWidgetItem()
				objItem.SetText(0, obj.ExtractShortName())

				// Status column
				if obj.Alive {
					objItem.SetText(1, "Active")
					objItem.SetForeground(1, qt6.NewQBrush3(qt6.NewQColor3(50, 180, 80)))
				} else {
					objItem.SetText(1, "Inactive")
					objItem.SetForeground(1, qt6.NewQBrush3(qt6.NewQColor3(180, 80, 80)))
				}

				// Store object hash in item data
				objItem.SetData(0, int(qt6.UserRole), qt6.NewQVariant4(int(obj.ObjHash)))
				objItem.SetData(0, int(qt6.UserRole)+1, qt6.NewQVariant4(srv.ID))

				// Tooltip with full info
				tooltip := fmt.Sprintf("Name: %s\nType: %s\nAddress: %s\nVersion: %s",
					obj.ObjName, obj.ObjType, obj.Address, obj.Version)
				objItem.SetToolTip(0, tooltip)

				typeItem.AddChild(objItem)
			}

			serverItem.AddChild(typeItem)
		}

		serverItem.SetExpanded(true)
	}
}

// getObjectsForServer returns objects for a specific server
func (v *ObjectNavView) getObjectsForServer(serverId int) []*pack.ObjectPack {
	srv := server.GetManager().GetServer(serverId)
	if srv == nil {
		return nil
	}
	return srv.GetObjects()
}

// filterObjects filters the tree based on search text
func (v *ObjectNavView) filterObjects(filter string) {
	if filter == "" {
		// Show all items
		v.setAllItemsVisible(nil, true)
		return
	}

	// Hide items that don't match
	v.filterItems(nil, filter)
}

// setAllItemsVisible sets visibility for all items
func (v *ObjectNavView) setAllItemsVisible(parent *qt6.QTreeWidgetItem, visible bool) {
	var count int
	if parent == nil {
		count = v.tree.TopLevelItemCount()
	} else {
		count = parent.ChildCount()
	}

	for i := 0; i < count; i++ {
		var item *qt6.QTreeWidgetItem
		if parent == nil {
			item = v.tree.TopLevelItem(i)
		} else {
			item = parent.Child(i)
		}

		item.SetHidden(!visible)
		v.setAllItemsVisible(item, visible)
	}
}

// filterItems recursively filters items
func (v *ObjectNavView) filterItems(parent *qt6.QTreeWidgetItem, filter string) bool {
	var count int
	if parent == nil {
		count = v.tree.TopLevelItemCount()
	} else {
		count = parent.ChildCount()
	}

	anyChildVisible := false

	for i := 0; i < count; i++ {
		var item *qt6.QTreeWidgetItem
		if parent == nil {
			item = v.tree.TopLevelItem(i)
		} else {
			item = parent.Child(i)
		}

		// Check if item text matches
		text := item.Text(0)
		matches := containsIgnoreCase(text, filter)

		// Check children
		childVisible := v.filterItems(item, filter)

		// Show item if it matches or has visible children
		visible := matches || childVisible
		item.SetHidden(!visible)

		if visible {
			anyChildVisible = true
			// Expand parent to show matching items
			if parent != nil {
				parent.SetExpanded(true)
			}
		}
	}

	return anyChildVisible
}

// saveExpandedState saves the expanded state of tree items
func (v *ObjectNavView) saveExpandedState(parent *qt6.QTreeWidgetItem) {
	var count int
	if parent == nil {
		count = v.tree.TopLevelItemCount()
		v.expandedNodes = make(map[string]bool)
	} else {
		count = parent.ChildCount()
	}

	for i := 0; i < count; i++ {
		var item *qt6.QTreeWidgetItem
		if parent == nil {
			item = v.tree.TopLevelItem(i)
		} else {
			item = parent.Child(i)
		}

		key := item.Text(0)
		v.expandedNodes[key] = item.IsExpanded()

		v.saveExpandedState(item)
	}
}

// restoreExpandedState restores the expanded state of tree items
func (v *ObjectNavView) restoreExpandedState(parent *qt6.QTreeWidgetItem) {
	var count int
	if parent == nil {
		count = v.tree.TopLevelItemCount()
	} else {
		count = parent.ChildCount()
	}

	for i := 0; i < count; i++ {
		var item *qt6.QTreeWidgetItem
		if parent == nil {
			item = v.tree.TopLevelItem(i)
		} else {
			item = parent.Child(i)
		}

		key := item.Text(0)
		if expanded, ok := v.expandedNodes[key]; ok && expanded {
			item.SetExpanded(true)
		}

		v.restoreExpandedState(item)
	}
}

// onItemSelected handles item selection
func (v *ObjectNavView) onItemSelected(item *qt6.QTreeWidgetItem) {
	if item == nil {
		return
	}

	// Get object hash from item data
	data := item.Data(0, int(qt6.UserRole))
	if data.IsNull() {
		return
	}

	objHash := int32(data.ToLongLong())
	if objHash <= 0 {
		return // Server or type node
	}

	// Get server ID
	serverData := item.Data(0, int(qt6.UserRole)+1)
	serverId := 0
	if !serverData.IsNull() {
		serverId = int(serverData.ToLongLong())
	}

	if v.onObjectSelected != nil {
		v.onObjectSelected(objHash, serverId)
	}
}

// onItemDoubleClicked handles double-click
func (v *ObjectNavView) onItemDoubleClicked(item *qt6.QTreeWidgetItem) {
	// Same as single click for now
	v.onItemSelected(item)
}

// showContextMenu shows context menu for tree item
func (v *ObjectNavView) showContextMenu(pos *qt6.QPoint) {
	item := v.tree.ItemAt(pos)
	if item == nil {
		return
	}

	menu := qt6.NewQMenu2()

	// Get item type
	statusText := item.Text(1)

	if statusText == "Active" || statusText == "Inactive" {
		// Object item
		viewAction := qt6.NewQAction2("View Details")
		viewAction.OnTriggered(func() {
			v.onItemSelected(item)
		})
		menu.AddAction(viewAction)

		menu.AddSeparator()

		refreshAction := qt6.NewQAction2("Refresh Status")
		refreshAction.OnTriggered(func() {
			v.Refresh()
		})
		menu.AddAction(refreshAction)
	} else if statusText == "Server" {
		// Server item
		refreshAction := qt6.NewQAction2("Refresh All Objects")
		refreshAction.OnTriggered(func() {
			v.Refresh()
		})
		menu.AddAction(refreshAction)
	} else {
		// Type group item
		expandAction := qt6.NewQAction2("Expand All")
		expandAction.OnTriggered(func() {
			v.expandAllChildren(item)
		})
		menu.AddAction(expandAction)

		collapseAction := qt6.NewQAction2("Collapse All")
		collapseAction.OnTriggered(func() {
			v.collapseAllChildren(item)
		})
		menu.AddAction(collapseAction)
	}

	cursorPos := qt6.QCursor_Pos()
	menu.ExecWithPos(cursorPos)
}

// expandAllChildren expands all children of an item
func (v *ObjectNavView) expandAllChildren(item *qt6.QTreeWidgetItem) {
	item.SetExpanded(true)
	for i := 0; i < item.ChildCount(); i++ {
		v.expandAllChildren(item.Child(i))
	}
}

// collapseAllChildren collapses all children of an item
func (v *ObjectNavView) collapseAllChildren(item *qt6.QTreeWidgetItem) {
	for i := 0; i < item.ChildCount(); i++ {
		v.collapseAllChildren(item.Child(i))
	}
	item.SetExpanded(false)
}

// Close cleans up resources
func (v *ObjectNavView) Close() {
	if v.refreshTimer != nil {
		v.refreshTimer.Stop()
	}
}

// GetSelectedObject returns the currently selected object hash and server ID
func (v *ObjectNavView) GetSelectedObject() (int32, int) {
	item := v.tree.CurrentItem()
	if item == nil {
		return 0, 0
	}

	data := item.Data(0, int(qt6.UserRole))
	if data.IsNull() {
		return 0, 0
	}

	objHash := int32(data.ToLongLong())
	if objHash <= 0 {
		return 0, 0
	}

	serverData := item.Data(0, int(qt6.UserRole)+1)
	serverId := 0
	if !serverData.IsNull() {
		serverId = int(serverData.ToLongLong())
	}

	return objHash, serverId
}

// SelectObject selects an object by hash
func (v *ObjectNavView) SelectObject(objHash int32) bool {
	return v.selectObjectInItem(nil, objHash)
}

// selectObjectInItem recursively searches for an object
func (v *ObjectNavView) selectObjectInItem(parent *qt6.QTreeWidgetItem, objHash int32) bool {
	var count int
	if parent == nil {
		count = v.tree.TopLevelItemCount()
	} else {
		count = parent.ChildCount()
	}

	for i := 0; i < count; i++ {
		var item *qt6.QTreeWidgetItem
		if parent == nil {
			item = v.tree.TopLevelItem(i)
		} else {
			item = parent.Child(i)
		}

		data := item.Data(0, int(qt6.UserRole))
		if !data.IsNull() && int32(data.ToLongLong()) == objHash {
			v.tree.SetCurrentItem(item)
			v.tree.ScrollToItem(item)
			return true
		}

		if v.selectObjectInItem(item, objHash) {
			return true
		}
	}

	return false
}


// formatUptime formats uptime duration
func formatUptime(wakeupMs int64) string {
	if wakeupMs == 0 {
		return "-"
	}

	now := time.Now().UnixMilli()
	uptime := time.Duration(now-wakeupMs) * time.Millisecond

	if uptime < time.Minute {
		return fmt.Sprintf("%ds", int(uptime.Seconds()))
	}
	if uptime < time.Hour {
		return fmt.Sprintf("%dm", int(uptime.Minutes()))
	}
	if uptime < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(uptime.Hours()), int(uptime.Minutes())%60)
	}
	days := int(uptime.Hours()) / 24
	hours := int(uptime.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}
