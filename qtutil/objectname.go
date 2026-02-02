package qtutil

import "github.com/mappu/miqt/qt6"

// SetObjectName sets the objectName property on a QObject using SetProperty+QVariant.
// This avoids the QAnyStringView use-after-free bug in miqt bindings where
// the underlying string data is freed before Qt copies it.
func SetObjectName(obj *qt6.QObject, name string) {
	v := qt6.NewQVariant14(name) // QVariant(QString) - copies the string
	obj.SetProperty("objectName", v)
}
