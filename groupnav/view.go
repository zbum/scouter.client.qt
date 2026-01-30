package groupnav

import (
	"sort"
	"sync"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/dialogs"
	"scouter.client.qt/protocol/pack"
	"scouter.client.qt/server"
)

// View represents the Group Navigation View (dock widget with tabs)
type View struct {
	dock            *qt6.QDockWidget
	tabWidget       *qt6.QTabWidget
	objectNameBytes []byte // Keep object name bytes alive for QAnyStringView

	// Group tab
	groupTreeView *qt6.QTreeView
	groupModel    *qt6.QStandardItemModel
	groupMap      map[string]HierarchyObject
	groupObjMap   map[int64]HierarchyObject // Map item ID to object

	// Object tab
	objectTreeView *qt6.QTreeView
	objectModel    *qt6.QStandardItemModel
	objectMap      map[string]HierarchyObject
	objectObjMap   map[int64]HierarchyObject // Map item ID to object

	nextItemID int64 // Counter for unique item IDs
	timer      *qt6.QTimer
	mu         sync.RWMutex

	// Track collapsed items (by name) to prevent auto-expand
	collapsedItems map[string]bool
	// Flag to ignore programmatic expand/collapse
	ignoringExpandEvents bool

	// Track column width ratio (0-100) to preserve after refresh and resize
	groupCol0Ratio      int  // percentage for first column (default 80)
	objectCol0Ratio     int  // percentage for first column (default 80)
	columnsInited       bool
	lastGroupWidth      int  // last known group tree width for resize detection
	lastObjectWidth     int  // last known object tree width for resize detection
	ignoringResizeEvents bool // prevent ratio recalc during programmatic resize

	// Cache last successful object list per server to avoid flickering on transient errors
	cachedObjects map[int][]*pack.ObjectPack

	// Callbacks
	onGroupSelected  func(group *GroupObject)
	onAgentSelected  func(agent *AgentObject)
	onRefreshRequest func()
	onAddGroupChart  func(groupName, objType, counterName, displayName string)
	onAddGroupXLog   func(groupName, objType string)
}

// NewView creates a new Group Navigation View
func NewView(mainWindow *qt6.QMainWindow) *View {
	v := &View{
		groupMap:       make(map[string]HierarchyObject),
		groupObjMap:    make(map[int64]HierarchyObject),
		objectMap:      make(map[string]HierarchyObject),
		objectObjMap:   make(map[int64]HierarchyObject),
		collapsedItems: make(map[string]bool),
		cachedObjects:  make(map[int][]*pack.ObjectPack),
		nextItemID:     1,
	}

	// Create dock widget
	v.dock = qt6.NewQDockWidget2("Navigation")

	// Create object name bytes and keep in memory for Qt state save/restore
	v.objectNameBytes = []byte("groupNavigationDock")
	objectNameView := qt6.NewQAnyStringView2(v.objectNameBytes)
	v.dock.SetObjectName(*objectNameView)
	v.dock.SetAllowedAreas(qt6.LeftDockWidgetArea | qt6.RightDockWidgetArea)

	// Create container widget with vertical layout
	container := qt6.NewQWidget2()
	layout := qt6.NewQVBoxLayout(container)
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(0)

	// Create tab widget
	v.tabWidget = qt6.NewQTabWidget2()
	v.tabWidget.SetTabPosition(qt6.QTabWidget__North)

	// Create Group tab
	groupTab := qt6.NewQWidget2()
	groupLayout := qt6.NewQVBoxLayout(groupTab)
	groupLayout.SetContentsMargins(0, 0, 0, 0)

	v.groupTreeView = qt6.NewQTreeView2()
	v.groupTreeView.SetHeaderHidden(false)
	v.groupTreeView.SetRootIsDecorated(true)
	v.groupTreeView.SetAlternatingRowColors(true)
	v.groupTreeView.SetSelectionMode(qt6.QAbstractItemView__SingleSelection)

	v.groupModel = qt6.NewQStandardItemModel2(0, 2)
	groupHeaderLabels := []string{"Group/Object", "Perf"}
	v.groupModel.SetHorizontalHeaderLabels(groupHeaderLabels)
	v.groupTreeView.SetModel(v.groupModel.QAbstractItemModel)

	// Set column widths - 80:20 ratio, user resizable, fills entire width
	groupHeader := v.groupTreeView.Header()
	groupHeader.SetStretchLastSection(true)
	groupHeader.SetSectionResizeMode2(0, qt6.QHeaderView__Interactive)
	groupHeader.ResizeSection(0, 200) // Initial width, will be adjusted

	groupLayout.AddWidget(v.groupTreeView.QWidget)
	v.tabWidget.AddTab(groupTab, "Group")

	// Create Object tab
	objectTab := qt6.NewQWidget2()
	objectLayout := qt6.NewQVBoxLayout(objectTab)
	objectLayout.SetContentsMargins(0, 0, 0, 0)

	v.objectTreeView = qt6.NewQTreeView2()
	v.objectTreeView.SetHeaderHidden(false)
	v.objectTreeView.SetRootIsDecorated(true)
	v.objectTreeView.SetAlternatingRowColors(true)
	v.objectTreeView.SetSelectionMode(qt6.QAbstractItemView__SingleSelection)

	v.objectModel = qt6.NewQStandardItemModel2(0, 2)
	objectHeaderLabels := []string{"Server/Object", "Perf"}
	v.objectModel.SetHorizontalHeaderLabels(objectHeaderLabels)
	v.objectTreeView.SetModel(v.objectModel.QAbstractItemModel)

	// Set column widths - 80:20 ratio, user resizable, fills entire width
	objectHeader := v.objectTreeView.Header()
	objectHeader.SetStretchLastSection(true)
	objectHeader.SetSectionResizeMode2(0, qt6.QHeaderView__Interactive)
	objectHeader.ResizeSection(0, 200) // Initial width, will be adjusted

	objectLayout.AddWidget(v.objectTreeView.QWidget)
	v.tabWidget.AddTab(objectTab, "Object")

	layout.AddWidget(v.tabWidget.QWidget)

	// Set container as dock widget content
	v.dock.SetWidget(container)

	// Add context menu for Group tab
	v.groupTreeView.SetContextMenuPolicy(qt6.CustomContextMenu)
	v.groupTreeView.OnCustomContextMenuRequested(func(pos *qt6.QPoint) {
		v.showGroupContextMenu(pos)
	})

	// Add context menu for Object tab
	v.objectTreeView.SetContextMenuPolicy(qt6.CustomContextMenu)
	v.objectTreeView.OnCustomContextMenuRequested(func(pos *qt6.QPoint) {
		v.showObjectContextMenu(pos)
	})

	// Add selection changed handler for Group tab
	v.groupTreeView.OnClicked(func(index *qt6.QModelIndex) {
		v.handleGroupSelection(index)
	})

	// Add selection changed handler for Object tab
	v.objectTreeView.OnClicked(func(index *qt6.QModelIndex) {
		v.handleObjectSelection(index)
	})

	// Track collapsed items in Group tab (only for user actions)
	v.groupTreeView.OnCollapsed(func(index *qt6.QModelIndex) {
		if v.ignoringExpandEvents {
			return
		}
		item := v.groupModel.ItemFromIndex(index)
		if item != nil {
			v.collapsedItems[item.Text()] = true
		}
	})
	v.groupTreeView.OnExpanded(func(index *qt6.QModelIndex) {
		if v.ignoringExpandEvents {
			return
		}
		item := v.groupModel.ItemFromIndex(index)
		if item != nil {
			delete(v.collapsedItems, item.Text())
		}
	})

	// Track collapsed items in Object tab (only for user actions)
	v.objectTreeView.OnCollapsed(func(index *qt6.QModelIndex) {
		if v.ignoringExpandEvents {
			return
		}
		item := v.objectModel.ItemFromIndex(index)
		if item != nil {
			v.collapsedItems[item.Text()] = true
		}
	})
	v.objectTreeView.OnExpanded(func(index *qt6.QModelIndex) {
		if v.ignoringExpandEvents {
			return
		}
		item := v.objectModel.ItemFromIndex(index)
		if item != nil {
			delete(v.collapsedItems, item.Text())
		}
	})

	// Add to main window (left side)
	mainWindow.AddDockWidget(qt6.LeftDockWidgetArea, v.dock)

	// Set initial column widths to 80:20 ratio when dock is shown
	v.dock.OnVisibilityChanged(func(visible bool) {
		if visible {
			v.adjustColumnWidths()
		}
	})

	// Adjust column widths when dock location changes
	v.dock.OnDockLocationChanged(func(area qt6.DockWidgetArea) {
		v.adjustColumnWidths()
	})

	// Save column ratio when user manually resizes columns
	groupHeader.OnSectionResized(func(logicalIndex int, oldSize int, newSize int) {
		if logicalIndex == 0 && !v.ignoringExpandEvents && !v.ignoringResizeEvents {
			groupWidth := v.groupTreeView.Width()
			if groupWidth > 0 && newSize > 0 {
				v.groupCol0Ratio = newSize * 100 / groupWidth
			}
		}
	})
	objectHeader.OnSectionResized(func(logicalIndex int, oldSize int, newSize int) {
		if logicalIndex == 0 && !v.ignoringExpandEvents && !v.ignoringResizeEvents {
			objectWidth := v.objectTreeView.Width()
			if objectWidth > 0 && newSize > 0 {
				v.objectCol0Ratio = newSize * 100 / objectWidth
			}
		}
	})

	// Setup refresh timer (3 seconds)
	v.timer = qt6.NewQTimer()
	v.timer.OnTimeout(func() {
		v.refresh()
	})
	v.timer.Start(3000)

	// Load groups from GroupManager
	v.organizeGroups()

	return v
}

// showGroupContextMenu displays the context menu for Group tab
func (v *View) showGroupContextMenu(pos *qt6.QPoint) {
	index := v.groupTreeView.IndexAt(pos)
	menu := qt6.NewQMenu2()

	if !index.IsValid() {
		// No item selected - show add group action
		addGroupAction := qt6.NewQAction2("Add Group")
		addGroupAction.OnTriggered(func() {
			v.showAddGroupDialog()
		})
		menu.AddAction(addGroupAction)
	} else {
		// Always get the item from column 0 (name column)
		col0Index := index.Sibling(index.Row(), 0)
		item := v.groupModel.ItemFromIndex(col0Index)
		if item != nil {
			// Get the item ID from UserRole data
			data := item.Data(int(qt6.UserRole))
			if data != nil {
				itemID := data.ToLongLong()
				v.mu.RLock()
				obj := v.groupObjMap[itemID]
				v.mu.RUnlock()

				if obj != nil {
					switch o := obj.(type) {
					case *GroupObject:
						groupName := o.GetName()
						objType := o.GetObjType()

						// Group header
						headerAction := qt6.NewQAction2(groupName)
						headerAction.SetEnabled(false)
						menu.AddAction(headerAction)
						menu.AddSeparator()

						// Manage group
						manageAction := qt6.NewQAction2("Manage Group")
						manageAction.OnTriggered(func() {
							v.showManageGroupDialog(groupName)
						})
						menu.AddAction(manageAction)

						// Remove group
						removeAction := qt6.NewQAction2("Remove")
						removeAction.OnTriggered(func() {
							v.removeGroup(groupName)
						})
						menu.AddAction(removeAction)

						// Counter chart items
						if v.onAddGroupChart != nil {
							menu.AddSeparator()
							type counterItem struct {
								display string
								counter string
							}
							counters := []counterItem{
								{"TPS Chart", "TPS"},
								{"Response Time Chart", "ElapsedTime"},
								{"Active Service Chart", "ActiveService"},
								{"CPU Chart", "Cpu"},
								{"Memory Chart", "UsedMemory"},
								{"GC Count Chart", "GcCount"},
								{"GC Time Chart", "GcTime"},
							}
							for _, ci := range counters {
								ci := ci // capture
								action := qt6.NewQAction2(ci.display)
								action.OnTriggered(func() {
									v.onAddGroupChart(groupName, objType, ci.counter, ci.display)
								})
								menu.AddAction(action)
							}
						}

						// XLog view
						if v.onAddGroupXLog != nil {
							menu.AddSeparator()
							xlogAction := qt6.NewQAction2("XLog")
							xlogAction.OnTriggered(func() {
								v.onAddGroupXLog(groupName, objType)
							})
							menu.AddAction(xlogAction)
						}

					case *AgentObject:
						objName := o.GetObjName()
						objHash := o.GetObjHash()

						// Agent header
						headerAction := qt6.NewQAction2(objName)
						headerAction.SetEnabled(false)
						menu.AddAction(headerAction)
						menu.AddSeparator()

						// Assign to group
						assignAction := qt6.NewQAction2("Assign to Group")
						assignAction.OnTriggered(func() {
							v.showAssignGroupDialog(objHash, objName)
						})
						menu.AddAction(assignAction)

					case *DummyObject:
						// Others folder - just show add group
						addGroupAction := qt6.NewQAction2("Add Group")
						addGroupAction.OnTriggered(func() {
							v.showAddGroupDialog()
						})
						menu.AddAction(addGroupAction)
					}
				}
			}
		}
	}

	cursorPos := qt6.QCursor_Pos()
	menu.ExecWithPos(cursorPos)
}

// showObjectContextMenu displays the context menu for Object tab
func (v *View) showObjectContextMenu(pos *qt6.QPoint) {
	index := v.objectTreeView.IndexAt(pos)
	menu := qt6.NewQMenu2()

	if !index.IsValid() {
		// No item selected - show add server action
		addServerAction := qt6.NewQAction2("Add Server")
		addServerAction.OnTriggered(func() {
			v.showAddServerDialog()
		})
		menu.AddAction(addServerAction)
	} else {
		// Always get the item from column 0 (name column)
		col0Index := index.Sibling(index.Row(), 0)
		item := v.objectModel.ItemFromIndex(col0Index)
		if item != nil {
			// Get item ID from UserRole data
			data := item.Data(int(qt6.UserRole))
			if data != nil {
				itemID := data.ToLongLong()
				v.mu.RLock()
				obj := v.objectObjMap[itemID]
				v.mu.RUnlock()

				if obj != nil {
					switch o := obj.(type) {
					case *AgentObject:
						objName := o.GetObjName()

						// Object header
						headerAction := qt6.NewQAction2(objName)
						headerAction.SetEnabled(false)
						menu.AddAction(headerAction)
						menu.AddSeparator()

						// Remove server
						removeAction := qt6.NewQAction2("Remove Server")
						removeAction.OnTriggered(func() {
							// TODO: implement remove server
						})
						menu.AddAction(removeAction)

					default:
						// For other items, just show add server
						addServerAction := qt6.NewQAction2("Add Server")
						addServerAction.OnTriggered(func() {
							v.showAddServerDialog()
						})
						menu.AddAction(addServerAction)
					}
				}
			}
		}
	}

	cursorPos := qt6.QCursor_Pos()
	menu.ExecWithPos(cursorPos)
}

// showAddServerDialog shows dialog to add a new server
func (v *View) showAddServerDialog() {
	dlg := dialogs.NewServerDialog(nil)
	dlg.SetOnResult(func(srv *server.Server) {
		v.organizeGroups()
	})
	dlg.Exec()
}

// handleGroupSelection handles tree item selection in Group tab
func (v *View) handleGroupSelection(index *qt6.QModelIndex) {
	if !index.IsValid() {
		return
	}

	// Always get the item from column 0 (name column)
	col0Index := index.Sibling(index.Row(), 0)
	item := v.groupModel.ItemFromIndex(col0Index)
	if item == nil {
		return
	}

	// Get item ID from UserRole data
	data := item.Data(int(qt6.UserRole))
	if data == nil {
		return
	}

	itemID := data.ToLongLong()
	v.mu.RLock()
	obj := v.groupObjMap[itemID]
	v.mu.RUnlock()

	if obj == nil {
		return
	}

	switch o := obj.(type) {
	case *GroupObject:
		if v.onGroupSelected != nil {
			v.onGroupSelected(o)
		}
	case *AgentObject:
		if v.onAgentSelected != nil {
			v.onAgentSelected(o)
		}
	}
}

// handleObjectSelection handles tree item selection in Object tab
func (v *View) handleObjectSelection(index *qt6.QModelIndex) {
	if !index.IsValid() {
		return
	}

	// Always get the item from column 0 (name column)
	col0Index := index.Sibling(index.Row(), 0)
	item := v.objectModel.ItemFromIndex(col0Index)
	if item == nil {
		return
	}

	// Get item ID from UserRole data
	data := item.Data(int(qt6.UserRole))
	if data == nil {
		return
	}

	itemID := data.ToLongLong()
	v.mu.RLock()
	obj := v.objectObjMap[itemID]
	v.mu.RUnlock()

	if obj == nil {
		return
	}

	if agent, ok := obj.(*AgentObject); ok {
		if v.onAgentSelected != nil {
			v.onAgentSelected(agent)
		}
	}
}

// refresh updates the tree view
func (v *View) refresh() {
	// Check if view was resized
	v.checkAndAdjustColumnWidths()

	if v.onRefreshRequest != nil {
		v.onRefreshRequest()
	}
	v.organizeGroups()
}

// forceRefresh forces a full refresh
func (v *View) forceRefresh() {
	v.organizeGroups()
}

// updateGroupTree rebuilds the Group tab tree (must be called with lock held)
func (v *View) updateGroupTree() {
	// Clear object map
	v.groupObjMap = make(map[int64]HierarchyObject)

	// Clear existing items and reset headers
	v.groupModel.Clear()
	v.groupModel.SetColumnCount(2)
	headerLabels := []string{"Group/Object", "Perf"}
	v.groupModel.SetHorizontalHeaderLabels(headerLabels)

	// Sort group names
	names := make([]string, 0, len(v.groupMap))
	for name := range v.groupMap {
		names = append(names, name)
	}
	sort.Strings(names)

	// Add groups to tree
	for _, name := range names {
		obj := v.groupMap[name]
		v.addToGroupModel(obj, nil)
	}

	// Expand items that are not in the collapsed set
	v.expandItemsSelectively(v.groupTreeView, v.groupModel)
}

// updateObjectTree rebuilds the Object tab tree (must be called with lock held)
func (v *View) updateObjectTree() {
	// Clear object map
	v.objectObjMap = make(map[int64]HierarchyObject)

	// Clear existing items and reset headers
	v.objectModel.Clear()
	v.objectModel.SetColumnCount(2)
	headerLabels := []string{"Server/Object", "Perf"}
	v.objectModel.SetHorizontalHeaderLabels(headerLabels)

	// Sort object names
	names := make([]string, 0, len(v.objectMap))
	for name := range v.objectMap {
		names = append(names, name)
	}
	sort.Strings(names)

	// Add objects to tree
	for _, name := range names {
		obj := v.objectMap[name]
		v.addToObjectModel(obj, nil)
	}

	// Expand items that are not in the collapsed set
	v.expandItemsSelectively(v.objectTreeView, v.objectModel)
}

// expandItemsSelectively expands or collapses each item individually based on collapsedItems
func (v *View) expandItemsSelectively(treeView *qt6.QTreeView, model *qt6.QStandardItemModel) {
	// Ignore expand/collapse events during programmatic changes
	v.ignoringExpandEvents = true
	defer func() { v.ignoringExpandEvents = false }()

	// Expand or collapse each item individually (avoid ExpandAll + Collapse race)
	rootIndex := qt6.NewQModelIndex()
	rowCount := model.RowCount(rootIndex)
	for i := 0; i < rowCount; i++ {
		item := model.Item(i)
		v.expandOrCollapseItem(treeView, model, item)
	}
}

// expandOrCollapseItem recursively expands or collapses items based on the collapsed set
func (v *View) expandOrCollapseItem(treeView *qt6.QTreeView, model *qt6.QStandardItemModel, item *qt6.QStandardItem) {
	if item == nil {
		return
	}

	index := model.IndexFromItem(item)
	name := item.Text()
	if v.collapsedItems[name] {
		treeView.Collapse(index)
	} else {
		treeView.Expand(index)
	}

	// Process children
	for i := 0; i < item.RowCount(); i++ {
		child := item.Child(i)
		v.expandOrCollapseItem(treeView, model, child)
	}
}

// addToGroupModel adds a hierarchy object to the group model
func (v *View) addToGroupModel(obj HierarchyObject, parent *qt6.QStandardItem) {
	var nameItem, perfItem *qt6.QStandardItem

	switch o := obj.(type) {
	case *GroupObject:
		nameItem = qt6.NewQStandardItem2(o.GetName())
		nameItem.SetEditable(false)
		perfItem = qt6.NewQStandardItem()
		perfItem.SetEditable(false)

	case *DummyObject:
		nameItem = qt6.NewQStandardItem2(o.GetName())
		nameItem.SetEditable(false)
		perfItem = qt6.NewQStandardItem()
		perfItem.SetEditable(false)

	case *AgentObject:
		nameItem = qt6.NewQStandardItem2(o.GetObjName())
		nameItem.SetEditable(false)

		// Set color based on alive status
		if !o.IsAlive() {
			grayColor := qt6.NewQColor3(128, 128, 128)
			brush := qt6.NewQBrush3(grayColor)
			nameItem.SetForeground(brush)
		}

		perfItem = qt6.NewQStandardItem2(o.GetMasterCounter())
		perfItem.SetEditable(false)
		perfItem.SetTextAlignment(qt6.AlignRight | qt6.AlignVCenter)

	default:
		return
	}

	// Store item ID in UserRole and map ID to object
	itemID := v.nextItemID
	v.nextItemID++
	nameItem.SetData(qt6.NewQVariant6(itemID), int(qt6.UserRole))
	v.groupObjMap[itemID] = obj

	row := []*qt6.QStandardItem{nameItem, perfItem}

	if parent == nil {
		v.groupModel.AppendRow(row)
	} else {
		parent.AppendRow(row)
	}

	// Add children recursively
	for _, child := range obj.GetSortedChildArray() {
		v.addToGroupModel(child, nameItem)
	}
}

// addToObjectModel adds a hierarchy object to the object model
func (v *View) addToObjectModel(obj HierarchyObject, parent *qt6.QStandardItem) {
	var nameItem, perfItem *qt6.QStandardItem

	switch o := obj.(type) {
	case *DummyObject:
		nameItem = qt6.NewQStandardItem2(o.GetName())
		nameItem.SetEditable(false)
		perfItem = qt6.NewQStandardItem()
		perfItem.SetEditable(false)

	case *AgentObject:
		nameItem = qt6.NewQStandardItem2(o.GetObjName())
		nameItem.SetEditable(false)

		// Set color based on alive status
		if !o.IsAlive() {
			grayColor := qt6.NewQColor3(128, 128, 128)
			brush := qt6.NewQBrush3(grayColor)
			nameItem.SetForeground(brush)
		}

		perfItem = qt6.NewQStandardItem2(o.GetMasterCounter())
		perfItem.SetEditable(false)
		perfItem.SetTextAlignment(qt6.AlignRight | qt6.AlignVCenter)

	default:
		return
	}

	// Store item ID in UserRole and map ID to object
	itemID := v.nextItemID
	v.nextItemID++
	nameItem.SetData(qt6.NewQVariant6(itemID), int(qt6.UserRole))
	v.objectObjMap[itemID] = obj

	row := []*qt6.QStandardItem{nameItem, perfItem}

	if parent == nil {
		v.objectModel.AppendRow(row)
	} else {
		parent.AppendRow(row)
	}

	// Add children recursively
	for _, child := range obj.GetSortedChildArray() {
		v.addToObjectModel(child, nameItem)
	}
}

// organizeGroups loads groups from GroupManager and organizes the trees
func (v *View) organizeGroups() {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Suppress ratio recalculation during entire rebuild
	v.ignoringResizeEvents = true
	defer func() { v.ignoringResizeEvents = false }()

	// Suppress repaints during clear+rebuild to prevent flickering
	v.groupTreeView.SetUpdatesEnabled(false)
	v.objectTreeView.SetUpdatesEnabled(false)
	defer func() {
		v.groupTreeView.SetUpdatesEnabled(true)
		v.objectTreeView.SetUpdatesEnabled(true)
	}()

	mgr := GetManager()
	v.groupMap = make(map[string]HierarchyObject)
	v.objectMap = make(map[string]HierarchyObject)

	// 1. Create groups from GroupManager for Group tab
	for _, groupName := range mgr.ListGroups() {
		objType := mgr.GetGroupObjType(groupName)
		if objType != "" {
			v.groupMap[groupName] = NewGroupObject(objType, groupName)
		}
	}

	// 2. Create Others group for unassigned agents
	othersGroup := NewDummyObject(OthersGroup)
	v.groupMap[OthersGroup] = othersGroup

	// 3. Fetch objects from connected servers for Object tab
	v.fetchServerObjects()

	// Update both trees
	v.updateGroupTree()
	v.updateObjectTree()

	// Restore column widths after refresh
	v.adjustColumnWidths()
}

// fetchServerObjects fetches object data from all connected servers
func (v *View) fetchServerObjects() {
	servers := server.GetManager().GetServers()

	for _, srv := range servers {
		// Create server folder
		serverFolder := NewDummyObject(srv.DisplayName())
		v.objectMap[srv.DisplayName()] = serverFolder

		if !srv.IsConnected() {
			// Add a "Not connected" indicator
			notConnected := NewDummyObject("(Not connected)")
			serverFolder.PutChild("(Not connected)", notConnected)
			continue
		}

		// Fetch objects from this server
		session := srv.Session()
		if session == nil {
			continue
		}

		objects, err := session.GetObjectList()
		if err != nil || len(objects) == 0 {
			// Use cached data on transient error to prevent flickering
			if cached, ok := v.cachedObjects[srv.ID]; ok {
				objects = cached
			} else {
				continue
			}
		} else {
			v.cachedObjects[srv.ID] = objects
		}

		// Group objects by type
		typeGroups := make(map[string]*DummyObject)

		for _, obj := range objects {
			// Get or create type group
			typeGroup, ok := typeGroups[obj.ObjType]
			if !ok {
				typeGroup = NewDummyObject(obj.ObjType)
				typeGroups[obj.ObjType] = typeGroup
				serverFolder.PutChild(obj.ObjType, typeGroup)
			}

			// Create agent object
			agent := NewAgentObjectFromPack(
				obj.ObjHash,
				obj.ObjName,
				obj.ObjType,
				obj.Address,
				obj.Version,
				obj.Alive,
				srv.ID,
			)
			typeGroup.PutChild(obj.ObjName, agent)

			// Also add to groups in Group tab if assigned
			v.addAgentToGroups(agent)
		}
	}
}

// addAgentToGroups adds an agent to its assigned groups
func (v *View) addAgentToGroups(agent *AgentObject) {
	mgr := GetManager()
	groups := mgr.GetGroupsForObject(agent.GetObjHash())

	if len(groups) == 0 {
		// Add to Others group
		if others, ok := v.groupMap[OthersGroup]; ok {
			others.PutChild(agent.GetObjName(), agent)
		}
		return
	}

	for _, groupName := range groups {
		if group, ok := v.groupMap[groupName]; ok {
			group.PutChild(agent.GetObjName(), agent)
		}
	}
}

// showAddGroupDialog shows dialog to add a new group
func (v *View) showAddGroupDialog() {
	objTypes := v.getAvailableObjTypes()
	dialog := NewAddGroupDialog(nil, objTypes)
	dialog.SetOnResult(func(objType, groupName string) {
		v.organizeGroups()
	})
	dialog.Exec()
}

// getAvailableObjTypes returns all unique object types from connected servers
func (v *View) getAvailableObjTypes() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	typeSet := make(map[string]bool)
	for _, obj := range v.objectMap {
		v.collectObjTypes(obj, typeSet)
	}

	types := make([]string, 0, len(typeSet))
	for t := range typeSet {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

func (v *View) collectObjTypes(obj HierarchyObject, typeSet map[string]bool) {
	if agent, ok := obj.(*AgentObject); ok {
		typeSet[agent.GetObjType()] = true
	}
	for _, child := range obj.GetChildren() {
		v.collectObjTypes(child, typeSet)
	}
}

// showManageGroupDialog shows dialog to manage a group
func (v *View) showManageGroupDialog(groupName string) {
	mgr := GetManager()
	objType := mgr.GetGroupObjType(groupName)
	if objType == "" {
		return
	}

	// Get all agents of this type
	agents := v.getAllAgentsOfType(objType)

	dialog := NewManageGroupDialog(nil, groupName, objType, agents)
	dialog.SetOnResult(func(name string, added, removed []int) {
		v.organizeGroups()
	})
	dialog.Exec()
}

// getAllAgentsOfType returns all agents of a specific objType
func (v *View) getAllAgentsOfType(objType string) []*AgentObject {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var agents []*AgentObject
	for _, obj := range v.objectMap {
		v.collectAgentsOfType(obj, objType, &agents)
	}
	return agents
}

func (v *View) collectAgentsOfType(obj HierarchyObject, objType string, agents *[]*AgentObject) {
	if agent, ok := obj.(*AgentObject); ok {
		if agent.GetObjType() == objType {
			*agents = append(*agents, agent)
		}
	}
	for _, child := range obj.GetChildren() {
		v.collectAgentsOfType(child, objType, agents)
	}
}

// showAssignGroupDialog shows dialog to assign agent to group
func (v *View) showAssignGroupDialog(objHash int, objName string) {
	// TODO: Implement assign group dialog
	_ = objHash
	_ = objName
}

// removeGroup removes a group
func (v *View) removeGroup(groupName string) {
	mgr := GetManager()
	mgr.RemoveGroup(groupName)
	v.organizeGroups()
}

// AddGroup adds a new group
func (v *View) AddGroup(objType, name string) bool {
	mgr := GetManager()
	if mgr.AddGroup(objType, name) {
		v.organizeGroups()
		return true
	}
	return false
}

// RemoveGroup removes a group by name
func (v *View) RemoveGroup(name string) {
	mgr := GetManager()
	mgr.RemoveGroup(name)
	v.organizeGroups()
}

// SetGroups sets the entire group map
func (v *View) SetGroups(groups map[string]HierarchyObject) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.groupMap = groups
	v.updateGroupTree()
}

// findAgentByHash finds an agent by its hash
func (v *View) findAgentByHash(objHash int) *AgentObject {
	for _, obj := range v.groupMap {
		if agent := v.findAgentInHierarchy(obj, objHash); agent != nil {
			return agent
		}
	}
	return nil
}

func (v *View) findAgentInHierarchy(obj HierarchyObject, objHash int) *AgentObject {
	if agent, ok := obj.(*AgentObject); ok {
		if agent.GetObjHash() == objHash {
			return agent
		}
	}
	for _, child := range obj.GetChildren() {
		if agent := v.findAgentInHierarchy(child, objHash); agent != nil {
			return agent
		}
	}
	return nil
}

// SetOnGroupSelected sets callback for group selection
func (v *View) SetOnGroupSelected(callback func(group *GroupObject)) {
	v.onGroupSelected = callback
}

// SetOnAgentSelected sets callback for agent selection
func (v *View) SetOnAgentSelected(callback func(agent *AgentObject)) {
	v.onAgentSelected = callback
}

// SetOnRefreshRequest sets callback for refresh request
func (v *View) SetOnRefreshRequest(callback func()) {
	v.onRefreshRequest = callback
}

// SetOnAddGroupChart sets callback for adding a group chart
func (v *View) SetOnAddGroupChart(callback func(groupName, objType, counterName, displayName string)) {
	v.onAddGroupChart = callback
}

// SetOnAddGroupXLog sets callback for adding a group XLog view
func (v *View) SetOnAddGroupXLog(callback func(groupName, objType string)) {
	v.onAddGroupXLog = callback
}

// Dock returns the dock widget
func (v *View) Dock() *qt6.QDockWidget {
	return v.dock
}

// GetCollapsedItems returns the list of collapsed tree item names
func (v *View) GetCollapsedItems() []string {
	items := make([]string, 0, len(v.collapsedItems))
	for name := range v.collapsedItems {
		items = append(items, name)
	}
	return items
}

// SetCollapsedItems restores the collapsed tree item names
func (v *View) SetCollapsedItems(items []string) {
	v.collapsedItems = make(map[string]bool, len(items))
	for _, name := range items {
		v.collapsedItems[name] = true
	}
}

// GetActiveTab returns the current tab index
func (v *View) GetActiveTab() int {
	return v.tabWidget.CurrentIndex()
}

// SetActiveTab sets the current tab index
func (v *View) SetActiveTab(index int) {
	if index >= 0 && index < v.tabWidget.Count() {
		v.tabWidget.SetCurrentIndex(index)
	}
}

// Stop stops the refresh timer
func (v *View) Stop() {
	if v.timer != nil {
		v.timer.Stop()
	}
}

// adjustColumnWidths applies column width ratios
func (v *View) adjustColumnWidths() {
	if !v.columnsInited {
		// First time: set to 80:20 ratio
		v.groupCol0Ratio = 80
		v.objectCol0Ratio = 80
		v.columnsInited = true
	}

	v.ignoringResizeEvents = true
	defer func() { v.ignoringResizeEvents = false }()

	// Apply ratios based on current view width
	groupWidth := v.groupTreeView.Width()
	if groupWidth > 0 && v.groupCol0Ratio > 0 {
		col0Width := groupWidth * v.groupCol0Ratio / 100
		v.groupTreeView.Header().ResizeSection(0, col0Width)
		v.lastGroupWidth = groupWidth
	}

	objectWidth := v.objectTreeView.Width()
	if objectWidth > 0 && v.objectCol0Ratio > 0 {
		col0Width := objectWidth * v.objectCol0Ratio / 100
		v.objectTreeView.Header().ResizeSection(0, col0Width)
		v.lastObjectWidth = objectWidth
	}
}

// checkAndAdjustColumnWidths checks if view was resized and adjusts columns
func (v *View) checkAndAdjustColumnWidths() {
	groupWidth := v.groupTreeView.Width()
	objectWidth := v.objectTreeView.Width()

	// If width changed, adjust columns
	if groupWidth != v.lastGroupWidth || objectWidth != v.lastObjectWidth {
		v.adjustColumnWidths()
	}
}

