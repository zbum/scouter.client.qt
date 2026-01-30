package views

import (
	"fmt"
	"sort"
	"sync"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/cache"
	"scouter.client.qt/groupnav"
	"scouter.client.qt/protocol"
	"scouter.client.qt/protocol/io"
	"scouter.client.qt/protocol/pack"
	"scouter.client.qt/server"
)

// Bar rendering constants
const (
	eqBarWidth   = 7
	eqBarPadding = 2
	eqAxisPad    = 16
	eqMinUnitH   = 20
	eqNameWidth  = 120
	eqCountWidth = 50
)

// ActiveSpeedData holds the three active speed levels
type ActiveSpeedData struct {
	Act1 int32 // normal (blue)
	Act2 int32 // slow (yellow)
	Act3 int32 // very slow (red)
}

// EqData represents one row in the EQ view
type EqData struct {
	ObjHash     int32
	DisplayName string
	Speed       ActiveSpeedData
}

// eqWidget is the custom-painted EQ widget
type eqWidget struct {
	*qt6.QWidget
	mu   sync.RWMutex
	data []EqData
}

func newEqWidget(parent *qt6.QWidget) *eqWidget {
	w := &eqWidget{}
	if parent != nil {
		w.QWidget = qt6.NewQWidget(parent)
	} else {
		w.QWidget = qt6.NewQWidget2()
	}
	w.QWidget.SetMinimumHeight(60)
	w.QWidget.SetMinimumWidth(200)

	w.QWidget.OnPaintEvent(func(super func(event *qt6.QPaintEvent), event *qt6.QPaintEvent) {
		w.paint()
	})

	return w
}

func (w *eqWidget) setData(data []EqData) {
	w.mu.Lock()
	w.data = data
	w.mu.Unlock()
	w.QWidget.Update()
}

func (w *eqWidget) paint() {
	w.mu.RLock()
	data := make([]EqData, len(w.data))
	copy(data, w.data)
	w.mu.RUnlock()

	painter := qt6.NewQPainter2(w.QWidget.QPaintDevice)
	defer painter.End()

	painter.SetRenderHint(qt6.QPainter__Antialiasing)

	widgetW := w.QWidget.Width()
	widgetH := w.QWidget.Height()

	// Colors used throughout
	grayColor := qt6.NewQColor3(128, 128, 128)
	dimColor := qt6.NewQColor3(100, 100, 100)
	darkColor := qt6.NewQColor3(80, 80, 80)
	colorAct1 := qt6.NewQColor3(59, 130, 246)  // blue #3B82F6
	colorAct2 := qt6.NewQColor3(234, 179, 8)   // yellow #EAB308
	colorAct3 := qt6.NewQColor3(239, 68, 68)   // red #EF4444
	blackColor := qt6.NewQColor3(0, 0, 0)

	if len(data) == 0 {
		painter.SetPen(grayColor)
		font := qt6.NewQFont()
		font.SetPointSize(10)
		painter.SetFont(font)
		rect := qt6.NewQRect4(0, 0, widgetW, widgetH)
		painter.DrawText6(rect, int(qt6.AlignCenter), "No active data")
		return
	}

	n := len(data)
	unitH := (widgetH - eqAxisPad) / n
	if unitH < eqMinUnitH {
		unitH = eqMinUnitH
	}

	// Find max total across all rows
	var maxTotal int32
	for _, d := range data {
		total := d.Speed.Act1 + d.Speed.Act2 + d.Speed.Act3
		if total > maxTotal {
			maxTotal = total
		}
	}
	if maxTotal == 0 {
		maxTotal = 1
	}

	barSpace := widgetW - eqNameWidth - eqCountWidth

	// Draw axis labels at top
	painter.SetPen(dimColor)
	smallFont := qt6.NewQFont()
	smallFont.SetPointSize(8)
	painter.SetFont(smallFont)

	// Scale labels: 0, max/2, max
	halfMax := maxTotal / 2
	painter.DrawText3(eqNameWidth, eqAxisPad-3, "0")
	if barSpace > 100 {
		midX := eqNameWidth + barSpace/2
		painter.DrawText3(midX, eqAxisPad-3, fmt.Sprintf("%d", halfMax))
	}
	painter.DrawText3(eqNameWidth+barSpace-30, eqAxisPad-3, fmt.Sprintf("%d", maxTotal))

	// Draw axis line
	axisLinePen := qt6.NewQPen3(darkColor)
	painter.SetPenWithPen(axisLinePen)
	painter.DrawLine2(eqNameWidth, eqAxisPad, eqNameWidth, eqAxisPad+n*unitH)

	nameFont := qt6.NewQFont()
	nameFont.SetPointSize(9)

	borderPen := qt6.NewQPen3(blackColor)

	for i, d := range data {
		y := eqAxisPad + i*unitH
		rowCenterY := y + unitH/2

		// Draw object name (left side)
		painter.SetPen(dimColor)
		painter.SetFont(nameFont)
		nameRect := qt6.NewQRect4(2, y, eqNameWidth-4, unitH)
		painter.DrawText6(nameRect, int(qt6.AlignVCenter|qt6.AlignRight), d.DisplayName)

		total := d.Speed.Act1 + d.Speed.Act2 + d.Speed.Act3
		if total == 0 {
			continue
		}

		// Calculate bar widths proportionally
		scale := float64(barSpace-4) / float64(maxTotal)
		w1 := int(float64(d.Speed.Act1) * scale)
		w2 := int(float64(d.Speed.Act2) * scale)
		w3 := int(float64(d.Speed.Act3) * scale)

		barH := unitH - eqBarPadding*2
		if barH > eqBarWidth*2 {
			barH = eqBarWidth * 2
		}
		barY := rowCenterY - barH/2

		// Draw bars: blue first, then yellow, then red
		x := eqNameWidth + 2
		painter.SetPenWithPen(borderPen)

		if w1 > 0 {
			painter.FillRect5(x, barY, w1, barH, colorAct1)
			painter.DrawRect2(x, barY, w1, barH)
			x += w1
		}
		if w2 > 0 {
			painter.FillRect5(x, barY, w2, barH, colorAct2)
			painter.DrawRect2(x, barY, w2, barH)
			x += w2
		}
		if w3 > 0 {
			painter.FillRect5(x, barY, w3, barH, colorAct3)
			painter.DrawRect2(x, barY, w3, barH)
			x += w3
		}

		// Draw total count (right side)
		painter.SetPen(dimColor)
		painter.SetFont(nameFont)
		countRect := qt6.NewQRect4(widgetW-eqCountWidth, y, eqCountWidth-4, unitH)
		painter.DrawText6(countRect, int(qt6.AlignVCenter|qt6.AlignLeft), fmt.Sprintf("%d", total))
	}

	// Draw legend at bottom-right
	legendY := eqAxisPad + n*unitH + 4
	if legendY+14 < widgetH {
		painter.SetFont(smallFont)
		lx := widgetW - 200
		sz := 8

		painter.FillRect5(lx, legendY+2, sz, sz, colorAct1)
		painter.SetPen(dimColor)
		painter.DrawText3(lx+sz+3, legendY+10, "Normal")

		lx += 60
		painter.FillRect5(lx, legendY+2, sz, sz, colorAct2)
		painter.DrawText3(lx+sz+3, legendY+10, "Slow")

		lx += 50
		painter.FillRect5(lx, legendY+2, sz, sz, colorAct3)
		painter.DrawText3(lx+sz+3, legendY+10, "Very Slow")
	}
}

// GroupEQView is the dock widget wrapper for the EQ view
type GroupEQView struct {
	dock            *qt6.QDockWidget
	objectNameBytes []byte
	widget          *eqWidget
	groupName       string
	objType         string
	timer           *qt6.QTimer
	active          bool
}

// NewGroupEQView creates a new group EQ dock view
func NewGroupEQView(mainWindow *qt6.QMainWindow, groupName, objType string) *GroupEQView {
	v := &GroupEQView{
		groupName: groupName,
		objType:   objType,
		active:    true,
	}

	title := fmt.Sprintf("%s - Active Service EQ", groupName)

	v.dock = qt6.NewQDockWidget2(title)
	v.objectNameBytes = []byte(fmt.Sprintf("eqDock_%s", groupName))
	objectNameView := qt6.NewQAnyStringView2(v.objectNameBytes)
	v.dock.SetObjectName(*objectNameView)
	v.dock.SetAllowedAreas(qt6.AllDockWidgetAreas)

	v.widget = newEqWidget(nil)
	v.dock.SetWidget(v.widget.QWidget)

	// 2-second polling timer
	v.timer = qt6.NewQTimer()
	v.timer.OnTimeout(func() {
		v.fetchAndUpdate()
	})
	v.timer.Start(2000)

	// Handle dock visibility
	v.dock.OnVisibilityChanged(func(visible bool) {
		if !visible {
			v.active = false
			if v.timer != nil {
				v.timer.Stop()
			}
		} else if !v.active {
			v.active = true
			if v.timer != nil {
				v.timer.Start(2000)
			}
		}
	})

	mainWindow.AddDockWidget(qt6.RightDockWidgetArea, v.dock)
	v.dock.Show()

	return v
}

// fetchAndUpdate fetches active speed data and updates the widget
func (v *GroupEQView) fetchAndUpdate() {
	members := groupnav.GetManager().GetObjectsByGroup(v.groupName)
	if len(members) == 0 {
		return
	}

	objHashList := &io.ListValue{}
	for hash := range members {
		objHashList.Add(io.NewDecimalValue(int32(hash)))
	}

	servers := server.GetManager().GetConnectedServers()
	if len(servers) == 0 {
		return
	}

	results := make(map[int32]ActiveSpeedData)

	for _, srv := range servers {
		session := srv.Session()
		if session == nil {
			continue
		}

		param := pack.NewMapPack()
		param.Put(protocol.ParamObjHash, objHashList)

		session.RequestStream(protocol.CMD_ACTIVESPEED_GROUP_REAL_TIME, param, func(p pack.Pack) bool {
			mp, ok := p.(*pack.MapPack)
			if !ok {
				return true
			}

			objHash := mp.GetDecimal("objHash")
			act1 := mp.GetDecimal("act1")
			act2 := mp.GetDecimal("act2")
			act3 := mp.GetDecimal("act3")

			results[objHash] = ActiveSpeedData{
				Act1: act1,
				Act2: act2,
				Act3: act3,
			}
			return true
		})
	}

	// Build sorted display data
	eqData := make([]EqData, 0, len(results))
	objCache := cache.GetObjectCache()
	for hash, speed := range results {
		name := objCache.GetObjName(hash)
		if name == "" {
			name = fmt.Sprintf("obj-%d", hash)
		}
		eqData = append(eqData, EqData{
			ObjHash:     hash,
			DisplayName: name,
			Speed:       speed,
		})
	}
	sort.Slice(eqData, func(i, j int) bool {
		return eqData[i].DisplayName < eqData[j].DisplayName
	})

	v.widget.setData(eqData)
}

// Dock returns the underlying dock widget
func (v *GroupEQView) Dock() *qt6.QDockWidget { return v.dock }

// GroupName returns the group name
func (v *GroupEQView) GroupName() string { return v.groupName }

// ObjType returns the object type
func (v *GroupEQView) ObjType() string { return v.objType }

// Close closes the view and cleans up
func (v *GroupEQView) Close() {
	v.active = false
	if v.timer != nil {
		v.timer.Stop()
	}
	v.dock.Close()
}
