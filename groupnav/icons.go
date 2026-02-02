package groupnav

import (
	"fmt"
	"sync"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/assets"
)

var (
	iconCache   = make(map[string]*qt6.QIcon)
	iconCacheMu sync.Mutex
)

// GetObjectIcon returns a QIcon for the given objType and alive status.
// Icons are cached for reuse. Falls back to context.png if the objType icon is not found.
func GetObjectIcon(objType string, alive bool) *qt6.QIcon {
	suffix := ""
	if !alive {
		suffix = "_inact"
	}

	key := fmt.Sprintf("%s%s", objType, suffix)

	iconCacheMu.Lock()
	defer iconCacheMu.Unlock()

	if icon, ok := iconCache[key]; ok {
		return icon
	}

	// Try objType-specific icon
	path := fmt.Sprintf("icons/object/%s%s.png", objType, suffix)
	data, err := assets.ObjectIconsFS.ReadFile(path)
	if err != nil {
		// Fallback to context icon
		fallbackPath := fmt.Sprintf("icons/object/context%s.png", suffix)
		data, err = assets.ObjectIconsFS.ReadFile(fallbackPath)
		if err != nil {
			return nil
		}
	}

	pixmap := qt6.NewQPixmap()
	pixmap.LoadFromDataWithData(data)
	icon := qt6.NewQIcon2(pixmap)
	iconCache[key] = icon
	return icon
}
