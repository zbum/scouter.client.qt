package dialogs

import (
	"fmt"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/server"
)

// ServerListDialog shows a list of configured servers
type ServerListDialog struct {
	dialog     *qt6.QDialog
	serverList *qt6.QListWidget
	serverMap  map[int]*server.Server // row -> server

	statusLabel *qt6.QLabel
}

// NewServerListDialog creates a new server list dialog
func NewServerListDialog(parent *qt6.QWidget) *ServerListDialog {
	d := &ServerListDialog{
		serverMap: make(map[int]*server.Server),
	}

	d.dialog = qt6.NewQDialog(parent)
	d.dialog.SetWindowTitle("Server Manager")
	d.dialog.SetMinimumSize2(500, 400)

	d.setupUI()
	d.loadServers()

	return d
}

func (d *ServerListDialog) setupUI() {
	mainLayout := qt6.NewQVBoxLayout(d.dialog.QWidget)

	// Server list
	d.serverList = qt6.NewQListWidget2()
	d.serverList.SetSelectionMode(qt6.QAbstractItemView__SingleSelection)
	mainLayout.AddWidget(d.serverList.QWidget)

	// Status label
	d.statusLabel = qt6.NewQLabel2()
	mainLayout.AddWidget(d.statusLabel.QWidget)

	// Buttons
	buttonLayout := qt6.NewQHBoxLayout2()

	addBtn := qt6.NewQPushButton3("Add")
	addBtn.OnClicked(func() {
		d.addServer()
	})
	buttonLayout.AddWidget(addBtn.QWidget)

	editBtn := qt6.NewQPushButton3("Edit")
	editBtn.OnClicked(func() {
		d.editServer()
	})
	buttonLayout.AddWidget(editBtn.QWidget)

	removeBtn := qt6.NewQPushButton3("Remove")
	removeBtn.OnClicked(func() {
		d.removeServer()
	})
	buttonLayout.AddWidget(removeBtn.QWidget)

	buttonLayout.AddStretch()

	connectBtn := qt6.NewQPushButton3("Connect")
	connectBtn.OnClicked(func() {
		d.connectServer()
	})
	buttonLayout.AddWidget(connectBtn.QWidget)

	disconnectBtn := qt6.NewQPushButton3("Disconnect")
	disconnectBtn.OnClicked(func() {
		d.disconnectServer()
	})
	buttonLayout.AddWidget(disconnectBtn.QWidget)

	buttonLayout.AddStretch()

	closeBtn := qt6.NewQPushButton3("Close")
	closeBtn.OnClicked(func() {
		d.dialog.Accept()
	})
	buttonLayout.AddWidget(closeBtn.QWidget)

	mainLayout.AddLayout(buttonLayout.QLayout)
}

func (d *ServerListDialog) loadServers() {
	d.serverList.Clear()
	d.serverMap = make(map[int]*server.Server)

	servers := server.GetManager().GetServers()
	for _, srv := range servers {
		item := qt6.NewQListWidgetItem5(d.serverList)
		d.updateItemText(item, srv)
		d.serverMap[d.serverList.Count()-1] = srv
	}
}

func (d *ServerListDialog) updateItemText(item *qt6.QListWidgetItem, srv *server.Server) {
	status := "Disconnected"
	if srv.IsConnected() {
		status = "Connected"
		if srv.Version() != "" {
			status += " (v" + srv.Version() + ")"
		}
	}

	text := fmt.Sprintf("%s - %s:%d [%s]", srv.DisplayName(), srv.Host, srv.Port, status)
	item.SetText(text)
}

func (d *ServerListDialog) getSelectedServer() *server.Server {
	row := d.serverList.CurrentRow()
	if row < 0 {
		return nil
	}
	return d.serverMap[row]
}

func (d *ServerListDialog) addServer() {
	dlg := NewServerDialog(d.dialog.QWidget)
	dlg.SetOnResult(func(srv *server.Server) {
		d.loadServers()
	})
	dlg.Exec()
}

func (d *ServerListDialog) editServer() {
	srv := d.getSelectedServer()
	if srv == nil {
		d.statusLabel.SetText("Please select a server")
		return
	}

	dlg := NewServerEditDialog(d.dialog.QWidget, srv)
	dlg.SetOnResult(func(updatedSrv *server.Server) {
		d.loadServers()
	})
	dlg.Exec()
}

func (d *ServerListDialog) removeServer() {
	srv := d.getSelectedServer()
	if srv == nil {
		d.statusLabel.SetText("Please select a server")
		return
	}

	// Confirm deletion
	result := qt6.QMessageBox_Question(
		d.dialog.QWidget,
		"Confirm Remove",
		"Are you sure you want to remove server '"+srv.DisplayName()+"'?",
	)
	if result == qt6.QMessageBox__Yes {
		server.GetManager().RemoveServer(srv.ID)
		d.loadServers()
		d.statusLabel.SetText("Server removed")
	}
}

func (d *ServerListDialog) connectServer() {
	srv := d.getSelectedServer()
	if srv == nil {
		d.statusLabel.SetText("Please select a server")
		return
	}

	if srv.IsConnected() {
		d.statusLabel.SetText("Already connected")
		return
	}

	d.statusLabel.SetText("Connecting...")
	qt6.QCoreApplication_ProcessEvents()

	// Connect synchronously (safer for Qt)
	err := srv.Connect()
	if err != nil {
		d.statusLabel.SetText("Connection failed: " + err.Error())
	} else {
		d.statusLabel.SetText("Connected successfully")
		d.loadServers()
	}
}

func (d *ServerListDialog) disconnectServer() {
	srv := d.getSelectedServer()
	if srv == nil {
		d.statusLabel.SetText("Please select a server")
		return
	}

	if !srv.IsConnected() {
		d.statusLabel.SetText("Not connected")
		return
	}

	srv.Disconnect()
	d.loadServers()
	d.statusLabel.SetText("Disconnected")
}

// Exec shows the dialog modally
func (d *ServerListDialog) Exec() int {
	return d.dialog.Exec()
}
