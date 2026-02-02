package groupnav

import (
	"sort"
	"strings"

	"github.com/mappu/miqt/qt6"
)

// ManageGroupDialog shows a dialog to manage objects in a group
type ManageGroupDialog struct {
	dialog         *qt6.QDialog
	groupName      string
	objType        string

	searchEdit     *qt6.QLineEdit
	availableList  *qt6.QListWidget
	selectedList   *qt6.QListWidget

	// Track original and current state
	originalSelected map[int]bool
	currentSelected  map[int]bool

	// All available agents of this type
	allAgents []*AgentObject

	// Callback when OK is clicked
	onResult func(groupName string, addObjHashs, removeObjHashs []int)
}

// NewManageGroupDialog creates a new manage group dialog
func NewManageGroupDialog(parent *qt6.QWidget, groupName, objType string, agents []*AgentObject) *ManageGroupDialog {
	d := &ManageGroupDialog{
		groupName:        groupName,
		objType:          objType,
		allAgents:        agents,
		originalSelected: make(map[int]bool),
		currentSelected:  make(map[int]bool),
	}

	d.dialog = qt6.NewQDialog(parent)
	d.dialog.SetWindowTitle("Manage Group - " + groupName)
	d.dialog.SetMinimumSize2(700, 500)

	d.setupUI()
	d.loadData()

	return d
}

func (d *ManageGroupDialog) setupUI() {
	mainLayout := qt6.NewQVBoxLayout(d.dialog.QWidget)

	// Title label
	titleLabel := qt6.NewQLabel3(d.groupName + " (" + d.objType + ")")
	titleFont := titleLabel.Font()
	titleFont.SetPointSize(10)
	titleFont.SetBold(true)
	titleLabel.SetFont(titleFont)
	mainLayout.AddWidget(titleLabel.QWidget)

	// Search bar
	searchLayout := qt6.NewQHBoxLayout2()
	searchLabel := qt6.NewQLabel3("Search:")
	d.searchEdit = qt6.NewQLineEdit2()
	d.searchEdit.SetPlaceholderText("Enter keyword to filter...")
	d.searchEdit.OnTextChanged(func(text string) {
		d.filterAvailableList(text)
	})
	searchLayout.AddWidget(searchLabel.QWidget)
	searchLayout.AddWidget(d.searchEdit.QWidget)
	mainLayout.AddLayout(searchLayout.QLayout)

	// Main content area (3 columns)
	contentLayout := qt6.NewQHBoxLayout2()

	// Left panel - Available objects
	leftPanel := qt6.NewQVBoxLayout2()
	availableLabel := qt6.NewQLabel3("Available Objects")
	d.availableList = qt6.NewQListWidget2()
	d.availableList.SetSelectionMode(qt6.QAbstractItemView__ExtendedSelection)
	leftPanel.AddWidget(availableLabel.QWidget)
	leftPanel.AddWidget(d.availableList.QWidget)
	contentLayout.AddLayout(leftPanel.QLayout)

	// Center panel - Buttons
	centerPanel := qt6.NewQVBoxLayout2()
	centerPanel.AddStretchWithStretch(1)

	addBtn := qt6.NewQPushButton3("Add >>")
	addBtn.OnClicked(func() {
		d.moveToSelected()
	})
	centerPanel.AddWidget(addBtn.QWidget)

	removeBtn := qt6.NewQPushButton3("<< Remove")
	removeBtn.OnClicked(func() {
		d.moveToAvailable()
	})
	centerPanel.AddWidget(removeBtn.QWidget)

	centerPanel.AddStretchWithStretch(1)
	contentLayout.AddLayout(centerPanel.QLayout)

	// Right panel - Selected objects
	rightPanel := qt6.NewQVBoxLayout2()
	selectedLabel := qt6.NewQLabel3("Selected Objects (in group)")
	d.selectedList = qt6.NewQListWidget2()
	d.selectedList.SetSelectionMode(qt6.QAbstractItemView__ExtendedSelection)
	rightPanel.AddWidget(selectedLabel.QWidget)
	rightPanel.AddWidget(d.selectedList.QWidget)
	contentLayout.AddLayout(rightPanel.QLayout)

	mainLayout.AddLayout(contentLayout.QLayout)

	// Bottom buttons
	buttonLayout := qt6.NewQHBoxLayout2()
	buttonLayout.AddStretchWithStretch(1)

	okBtn := qt6.NewQPushButton3("OK")
	okBtn.OnClicked(func() {
		d.onOK()
	})
	buttonLayout.AddWidget(okBtn.QWidget)

	cancelBtn := qt6.NewQPushButton3("Cancel")
	cancelBtn.OnClicked(func() {
		d.dialog.Reject()
	})
	buttonLayout.AddWidget(cancelBtn.QWidget)

	mainLayout.AddLayout(buttonLayout.QLayout)
}

func (d *ManageGroupDialog) loadData() {
	mgr := GetManager()

	// Get objects currently in this group
	objsInGroup := mgr.GetObjectsByGroup(d.groupName)

	// Separate agents into selected and available
	for _, agent := range d.allAgents {
		if agent.GetObjType() != d.objType {
			continue
		}

		objHash := agent.GetObjHash()
		if objsInGroup[objHash] {
			d.originalSelected[objHash] = true
			d.currentSelected[objHash] = true
		}
	}

	d.refreshLists()
}

func (d *ManageGroupDialog) refreshLists() {
	d.availableList.Clear()
	d.selectedList.Clear()

	// Sort agents by name
	sortedAgents := make([]*AgentObject, len(d.allAgents))
	copy(sortedAgents, d.allAgents)
	sort.Slice(sortedAgents, func(i, j int) bool {
		return sortedAgents[i].GetObjName() < sortedAgents[j].GetObjName()
	})

	filter := strings.ToLower(d.searchEdit.Text())

	for _, agent := range sortedAgents {
		if agent.GetObjType() != d.objType {
			continue
		}

		objHash := agent.GetObjHash()
		displayText := agent.GetObjName()

		if d.currentSelected[objHash] {
			// Add to selected list
			item := qt6.NewQListWidgetItem()
			item.SetText(displayText)
			item.SetData(int(qt6.UserRole), qt6.NewQVariant6(int64(objHash)))
			d.selectedList.AddItemWithItem(item)
		} else {
			// Add to available list (with filter)
			if filter == "" || strings.Contains(strings.ToLower(displayText), filter) {
				item := qt6.NewQListWidgetItem()
				item.SetText(displayText)
				item.SetData(int(qt6.UserRole), qt6.NewQVariant6(int64(objHash)))
				d.availableList.AddItemWithItem(item)
			}
		}
	}
}

func (d *ManageGroupDialog) filterAvailableList(filter string) {
	d.refreshLists()
}

func (d *ManageGroupDialog) moveToSelected() {
	selectedItems := d.availableList.SelectedItems()
	for _, item := range selectedItems {
		data := item.Data(int(qt6.UserRole))
		if data != nil {
			objHash := int(data.ToLongLong())
			d.currentSelected[objHash] = true
		}
	}
	d.refreshLists()
}

func (d *ManageGroupDialog) moveToAvailable() {
	selectedItems := d.selectedList.SelectedItems()
	for _, item := range selectedItems {
		data := item.Data(int(qt6.UserRole))
		if data != nil {
			objHash := int(data.ToLongLong())
			delete(d.currentSelected, objHash)
		}
	}
	d.refreshLists()
}

func (d *ManageGroupDialog) onOK() {
	// Calculate added and removed objects
	var addObjHashs []int
	var removeObjHashs []int

	// Find newly added (in current but not in original)
	for objHash := range d.currentSelected {
		if !d.originalSelected[objHash] {
			addObjHashs = append(addObjHashs, objHash)
		}
	}

	// Find removed (in original but not in current)
	for objHash := range d.originalSelected {
		if !d.currentSelected[objHash] {
			removeObjHashs = append(removeObjHashs, objHash)
		}
	}

	// Apply changes to GroupManager
	mgr := GetManager()
	if len(addObjHashs) > 0 {
		mgr.AddObjects(addObjHashs, d.groupName)
	}
	if len(removeObjHashs) > 0 {
		mgr.RemoveObjects(removeObjHashs, d.groupName)
	}

	if d.onResult != nil {
		d.onResult(d.groupName, addObjHashs, removeObjHashs)
	}

	d.dialog.Accept()
}

// SetOnResult sets the callback for when OK is clicked
func (d *ManageGroupDialog) SetOnResult(callback func(groupName string, addObjHashs, removeObjHashs []int)) {
	d.onResult = callback
}

// Exec shows the dialog modally and returns the result
func (d *ManageGroupDialog) Exec() int {
	return d.dialog.Exec()
}

// AddGroupDialog shows a dialog to add a new group
type AddGroupDialog struct {
	dialog      *qt6.QDialog
	nameEdit    *qt6.QLineEdit
	typeCombo   *qt6.QComboBox

	onResult func(objType, groupName string)
}

// NewAddGroupDialog creates a new add group dialog
func NewAddGroupDialog(parent *qt6.QWidget, objTypes []string) *AddGroupDialog {
	d := &AddGroupDialog{}

	d.dialog = qt6.NewQDialog(parent)
	d.dialog.SetWindowTitle("Add Group")
	d.dialog.SetMinimumSize2(300, 150)

	d.setupUI(objTypes)

	return d
}

func (d *AddGroupDialog) setupUI(objTypes []string) {
	layout := qt6.NewQVBoxLayout(d.dialog.QWidget)

	// Group name
	nameLayout := qt6.NewQHBoxLayout2()
	nameLabel := qt6.NewQLabel3("Group Name:")
	d.nameEdit = qt6.NewQLineEdit2()
	d.nameEdit.SetPlaceholderText("Enter group name...")
	nameLayout.AddWidget(nameLabel.QWidget)
	nameLayout.AddWidget(d.nameEdit.QWidget)
	layout.AddLayout(nameLayout.QLayout)

	// Object type - dynamically populated from connected servers
	typeLayout := qt6.NewQHBoxLayout2()
	typeLabel := qt6.NewQLabel3("Object Type:")
	d.typeCombo = qt6.NewQComboBox2()
	for _, t := range objTypes {
		d.typeCombo.AddItem(t)
	}
	typeLayout.AddWidget(typeLabel.QWidget)
	typeLayout.AddWidget(d.typeCombo.QWidget)
	layout.AddLayout(typeLayout.QLayout)

	layout.AddStretchWithStretch(1)

	// Buttons
	buttonLayout := qt6.NewQHBoxLayout2()
	buttonLayout.AddStretchWithStretch(1)

	okBtn := qt6.NewQPushButton3("OK")
	okBtn.OnClicked(func() {
		d.onOK()
	})
	buttonLayout.AddWidget(okBtn.QWidget)

	cancelBtn := qt6.NewQPushButton3("Cancel")
	cancelBtn.OnClicked(func() {
		d.dialog.Reject()
	})
	buttonLayout.AddWidget(cancelBtn.QWidget)

	layout.AddLayout(buttonLayout.QLayout)
}

func (d *AddGroupDialog) onOK() {
	groupName := strings.TrimSpace(d.nameEdit.Text())
	if groupName == "" {
		return
	}

	objType := d.typeCombo.CurrentText()

	// Add to GroupManager
	mgr := GetManager()
	if mgr.AddGroup(objType, groupName) {
		if d.onResult != nil {
			d.onResult(objType, groupName)
		}
		d.dialog.Accept()
	}
}

// SetOnResult sets the callback for when OK is clicked
func (d *AddGroupDialog) SetOnResult(callback func(objType, groupName string)) {
	d.onResult = callback
}

// Exec shows the dialog modally
func (d *AddGroupDialog) Exec() int {
	return d.dialog.Exec()
}
