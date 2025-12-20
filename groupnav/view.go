package groupnav

import (
	"sort"
	"sync"

	"github.com/mappu/miqt/qt6"
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

	// Callbacks
	onGroupSelected  func(group *GroupObject)
	onAgentSelected  func(agent *AgentObject)
	onRefreshRequest func()
}

// NewView creates a new Group Navigation View
func NewView(mainWindow *qt6.QMainWindow) *View {
	v := &View{
		groupMap:     make(map[string]HierarchyObject),
		groupObjMap:  make(map[int64]HierarchyObject),
		objectMap:    make(map[string]HierarchyObject),
		objectObjMap: make(map[int64]HierarchyObject),
		nextItemID:   1,
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

	// Set proportional column widths (180:60 = 3:1 ratio)
	groupHeader := v.groupTreeView.Header()
	groupHeader.SetStretchLastSection(true)
	groupHeader.SetSectionResizeMode2(0, qt6.QHeaderView__Interactive)
	groupHeader.ResizeSection(0, 180)

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

	// Set proportional column widths (180:60 = 3:1 ratio)
	objectHeader := v.objectTreeView.Header()
	objectHeader.SetStretchLastSection(true)
	objectHeader.SetSectionResizeMode2(0, qt6.QHeaderView__Interactive)
	objectHeader.ResizeSection(0, 180)

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

	// Add to main window (left side)
	mainWindow.AddDockWidget(qt6.LeftDockWidgetArea, v.dock)

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
	// TODO: Implement add server dialog
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

	v.groupTreeView.ExpandAll()
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

	v.objectTreeView.ExpandAll()
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

	// 3. Create object list for Object tab
	// For now, create a placeholder "Servers" folder
	serversFolder := NewDummyObject("Servers")
	v.objectMap["Servers"] = serversFolder

	// Update both trees
	v.updateGroupTree()
	v.updateObjectTree()
}

// showAddGroupDialog shows dialog to add a new group
func (v *View) showAddGroupDialog() {
	dialog := NewAddGroupDialog(nil)
	dialog.SetOnResult(func(objType, groupName string) {
		v.organizeGroups()
	})
	dialog.Exec()
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
	for _, obj := range v.groupMap {
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

// Dock returns the dock widget
func (v *View) Dock() *qt6.QDockWidget {
	return v.dock
}

// Stop stops the refresh timer
func (v *View) Stop() {
	if v.timer != nil {
		v.timer.Stop()
	}
}