package dialogs

import (
	"fmt"
	"runtime"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/assets"
)

const aboutText = `<h2 style="margin:0;">Scouter Client Qt</h2>
<p style="color:#888; margin:2px 0 10px 0;">Open Source S/W Performance Monitoring</p>
<hr>
<p style="font-size:12px;">
Go %s / Qt6<br>
OS: %s / %s
</p>
<p style="font-size:11px; color:#666;">
© 2015 Scouter Project<br>
<a href="https://github.com/scouter-project/scouter">github.com/scouter-project/scouter</a>
</p>
<p style="font-size:10px; color:#999;">
Licensed under the Apache License, Version 2.0
</p>`

func ShowAboutDialog(parent *qt6.QWidget) {
	dialog := qt6.NewQDialog(parent)
	dialog.SetWindowTitle("About Scouter")
	dialog.SetFixedSize2(340, 420)

	mainLayout := qt6.NewQVBoxLayout(dialog.QWidget)
	mainLayout.SetContentsMargins(20, 20, 20, 20)

	// Logo image
	pixmap := qt6.NewQPixmap()
	pixmap.LoadFromDataWithData(assets.AboutDialogPNG)

	logoLabel := qt6.NewQLabel2()
	logoLabel.SetPixmap(pixmap)
	logoLabel.SetAlignment(qt6.AlignCenter)
	mainLayout.AddWidget(logoLabel.QWidget)

	mainLayout.AddSpacing(10)

	// About text
	textLabel := qt6.NewQLabel2()
	text := formatAboutText()
	textLabel.SetText(text)
	textLabel.SetAlignment(qt6.AlignCenter)
	textLabel.SetWordWrap(true)
	textLabel.SetOpenExternalLinks(true)
	textLabel.SetTextFormat(qt6.RichText)
	mainLayout.AddWidget(textLabel.QWidget)

	mainLayout.AddStretch()

	// Close button
	buttonLayout := qt6.NewQHBoxLayout2()
	buttonLayout.AddStretch()
	closeBtn := qt6.NewQPushButton3("Close")
	closeBtn.SetFixedWidth(80)
	closeBtn.OnClicked(func() {
		dialog.Accept()
	})
	buttonLayout.AddWidget(closeBtn.QWidget)
	buttonLayout.AddStretch()
	mainLayout.AddLayout(buttonLayout.QLayout)

	dialog.Exec()
}

func formatAboutText() string {
	goVersion := runtime.Version()
	osName := runtime.GOOS
	arch := runtime.GOARCH
	return fmt.Sprintf(aboutText, goVersion, osName, arch)
}

