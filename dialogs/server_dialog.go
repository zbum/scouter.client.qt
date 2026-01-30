// Package dialogs provides dialog windows for the application
package dialogs

import (
	"strconv"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/protocol"
	"scouter.client.qt/server"
)

// ServerDialog is a dialog for adding/editing server connections
type ServerDialog struct {
	dialog    *qt6.QDialog
	isEdit    bool
	serverID  int

	nameEdit     *qt6.QLineEdit
	hostEdit     *qt6.QLineEdit
	portEdit     *qt6.QSpinBox
	userEdit     *qt6.QLineEdit
	passEdit     *qt6.QLineEdit
	autoConnect  *qt6.QCheckBox
	statusLabel  *qt6.QLabel

	onResult func(server *server.Server)
}

// NewServerDialog creates a new server dialog for adding a server
func NewServerDialog(parent *qt6.QWidget) *ServerDialog {
	d := &ServerDialog{
		isEdit: false,
	}
	d.setup(parent)
	return d
}

// NewServerEditDialog creates a new server dialog for editing an existing server
func NewServerEditDialog(parent *qt6.QWidget, srv *server.Server) *ServerDialog {
	d := &ServerDialog{
		isEdit:   true,
		serverID: srv.ID,
	}
	d.setup(parent)

	// Populate fields with existing values
	d.nameEdit.SetText(srv.Name)
	d.hostEdit.SetText(srv.Host)
	d.portEdit.SetValue(srv.Port)
	d.userEdit.SetText(srv.UserID)
	d.passEdit.SetText(srv.Password)
	d.autoConnect.SetChecked(srv.AutoConnect)

	return d
}

func (d *ServerDialog) setup(parent *qt6.QWidget) {
	d.dialog = qt6.NewQDialog(parent)
	if d.isEdit {
		d.dialog.SetWindowTitle("Edit Server")
	} else {
		d.dialog.SetWindowTitle("Add Server")
	}
	d.dialog.SetMinimumSize2(400, 300)

	mainLayout := qt6.NewQVBoxLayout(d.dialog.QWidget)

	// Form layout
	formLayout := qt6.NewQFormLayout2()

	// Server name
	d.nameEdit = qt6.NewQLineEdit2()
	d.nameEdit.SetPlaceholderText("My Scouter Server")
	formLayout.AddRow3("Name:", d.nameEdit.QWidget)

	// Host
	d.hostEdit = qt6.NewQLineEdit2()
	d.hostEdit.SetPlaceholderText("localhost")
	formLayout.AddRow3("Host:", d.hostEdit.QWidget)

	// Port
	d.portEdit = qt6.NewQSpinBox2()
	d.portEdit.SetMinimum(1)
	d.portEdit.SetMaximum(65535)
	d.portEdit.SetValue(protocol.DefaultCollectorPort)
	formLayout.AddRow3("Port:", d.portEdit.QWidget)

	// Username
	d.userEdit = qt6.NewQLineEdit2()
	d.userEdit.SetPlaceholderText("admin")
	formLayout.AddRow3("Username:", d.userEdit.QWidget)

	// Password
	d.passEdit = qt6.NewQLineEdit2()
	d.passEdit.SetEchoMode(qt6.QLineEdit__Password)
	formLayout.AddRow3("Password:", d.passEdit.QWidget)

	// Auto-connect
	d.autoConnect = qt6.NewQCheckBox3("Connect on startup")
	formLayout.AddRow3("", d.autoConnect.QWidget)

	mainLayout.AddLayout(formLayout.QLayout)

	// Status label
	d.statusLabel = qt6.NewQLabel2()
	d.statusLabel.SetStyleSheet("color: red;")
	mainLayout.AddWidget(d.statusLabel.QWidget)

	// Spacer
	mainLayout.AddStretch()

	// Buttons
	buttonLayout := qt6.NewQHBoxLayout2()
	buttonLayout.AddStretch()

	testBtn := qt6.NewQPushButton3("Test Connection")
	testBtn.OnClicked(func() {
		d.testConnection()
	})
	buttonLayout.AddWidget(testBtn.QWidget)

	cancelBtn := qt6.NewQPushButton3("Cancel")
	cancelBtn.OnClicked(func() {
		d.dialog.Reject()
	})
	buttonLayout.AddWidget(cancelBtn.QWidget)

	okBtn := qt6.NewQPushButton3("OK")
	okBtn.SetDefault(true)
	okBtn.OnClicked(func() {
		d.accept()
	})
	buttonLayout.AddWidget(okBtn.QWidget)

	mainLayout.AddLayout(buttonLayout.QLayout)
}

func (d *ServerDialog) testConnection() {
	host := d.hostEdit.Text()
	port := d.portEdit.Value()
	userID := d.userEdit.Text()
	password := d.passEdit.Text()

	if host == "" {
		d.statusLabel.SetText("Host is required")
		return
	}

	d.statusLabel.SetStyleSheet("color: blue;")
	d.statusLabel.SetText("Testing connection...")

	// Force UI update
	qt6.QCoreApplication_ProcessEvents()

	// Test connection synchronously
	srv := server.NewServer(0, "", host, port)
	srv.SetCredentials(userID, password)
	err := srv.Connect()

	if err != nil {
		d.statusLabel.SetStyleSheet("color: red;")
		d.statusLabel.SetText("Connection failed: " + err.Error())
	} else {
		version := srv.Version()
		srv.Disconnect()
		msg := "Connected successfully!"
		if version != "" {
			msg = "Connected! Server version: " + version
		}
		d.statusLabel.SetStyleSheet("color: green;")
		d.statusLabel.SetText(msg)
	}
}

func (d *ServerDialog) accept() {
	host := d.hostEdit.Text()
	if host == "" {
		d.statusLabel.SetText("Host is required")
		return
	}

	name := d.nameEdit.Text()
	if name == "" {
		name = host + ":" + strconv.Itoa(d.portEdit.Value())
	}

	port := d.portEdit.Value()
	userID := d.userEdit.Text()
	password := d.passEdit.Text()
	autoConnect := d.autoConnect.IsChecked()

	var srv *server.Server
	if d.isEdit {
		err := server.GetManager().UpdateServer(
			d.serverID, name, host, port, userID, password, autoConnect,
		)
		if err != nil {
			d.statusLabel.SetText("Failed to update: " + err.Error())
			return
		}
		srv = server.GetManager().GetServer(d.serverID)
	} else {
		srv = server.GetManager().AddServer(name, host, port, userID, password)
		srv.AutoConnect = autoConnect
	}

	// Connect the server
	if srv != nil {
		if err := srv.Connect(); err != nil {
			d.statusLabel.SetStyleSheet("color: orange;")
			d.statusLabel.SetText("Server added but connection failed: " + err.Error())
			// Still proceed - server is saved
		}
	}

	if d.onResult != nil && srv != nil {
		d.onResult(srv)
	}

	d.dialog.Accept()
}

// SetOnResult sets the callback for when OK is clicked
func (d *ServerDialog) SetOnResult(callback func(server *server.Server)) {
	d.onResult = callback
}

// Exec shows the dialog modally
func (d *ServerDialog) Exec() int {
	return d.dialog.Exec()
}

// Show shows the dialog non-modally
func (d *ServerDialog) Show() {
	d.dialog.Show()
}
