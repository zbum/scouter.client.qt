package assets

import "embed"

//go:embed about_dialog.png
var AboutDialogPNG []byte

//go:embed app_icon.png
var AppIconPNG []byte

//go:embed icons/object
var ObjectIconsFS embed.FS
