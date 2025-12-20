package main

import (
	"github.com/mappu/miqt/qt6"
)

// TreeManager manages the tree view dock widget
type TreeManager struct {
	dock     *qt6.QDockWidget
	treeView *qt6.QTreeView
	model    *qt6.QStandardItemModel
}

// NewTreeManager creates a new tree manager with a dock widget
func NewTreeManager(mainWindow *qt6.QMainWindow) *TreeManager {
	tm := &TreeManager{}

	// Create dock widget
	tm.dock = qt6.NewQDockWidget2("Explorer")
	objectName := qt6.NewQAnyStringView3("explorerDock")
	tm.dock.SetObjectName(*objectName)
	tm.dock.SetAllowedAreas(qt6.LeftDockWidgetArea | qt6.RightDockWidgetArea)

	// Create tree view
	tm.treeView = qt6.NewQTreeView2()
	tm.treeView.SetHeaderHidden(true)

	// Create model
	tm.model = qt6.NewQStandardItemModel2()
	tm.treeView.SetModel(tm.model.QAbstractItemModel)

	// Add sample data
	tm.addSampleData()

	// Set tree view as dock widget content
	tm.dock.SetWidget(tm.treeView.QWidget)

	// Add to main window (left side)
	mainWindow.AddDockWidget(qt6.LeftDockWidgetArea, tm.dock)

	return tm
}

// addSampleData adds sample items to the tree
func (tm *TreeManager) addSampleData() {
	// Root items
	projectItem := qt6.NewQStandardItem2("Project")
	projectItem.SetEditable(false)

	// Source folder
	srcItem := qt6.NewQStandardItem2("src")
	srcItem.SetEditable(false)

	mainFile := qt6.NewQStandardItem2("main.go")
	mainFile.SetEditable(false)
	srcItem.AppendRow(mainFile)

	chartFile := qt6.NewQStandardItem2("chart.go")
	chartFile.SetEditable(false)
	srcItem.AppendRow(chartFile)

	menuFile := qt6.NewQStandardItem2("menu.go")
	menuFile.SetEditable(false)
	srcItem.AppendRow(menuFile)

	projectItem.AppendRow(srcItem)

	// Assets folder
	assetsItem := qt6.NewQStandardItem2("assets")
	assetsItem.SetEditable(false)

	iconFile := qt6.NewQStandardItem2("AppIcon.icns")
	iconFile.SetEditable(false)
	assetsItem.AppendRow(iconFile)

	projectItem.AppendRow(assetsItem)

	// Config files
	makefileItem := qt6.NewQStandardItem2("Makefile")
	makefileItem.SetEditable(false)
	projectItem.AppendRow(makefileItem)

	goModItem := qt6.NewQStandardItem2("go.mod")
	goModItem.SetEditable(false)
	projectItem.AppendRow(goModItem)

	// Add to model
	tm.model.AppendRow(projectItem)

	// Expand project item
	tm.treeView.ExpandAll()
}

// AddItem adds a new item to the tree under the specified parent
func (tm *TreeManager) AddItem(parentText, itemText string) {
	// Find parent item
	rootIndex := tm.model.Index(0, 0, qt6.QModelIndex{})
	parentItem := tm.model.ItemFromIndex(rootIndex)

	if parentItem != nil && parentItem.Text() == parentText {
		newItem := qt6.NewQStandardItem2(itemText)
		newItem.SetEditable(false)
		parentItem.AppendRow(newItem)
	}
}

// Dock returns the dock widget
func (tm *TreeManager) Dock() *qt6.QDockWidget {
	return tm.dock
}