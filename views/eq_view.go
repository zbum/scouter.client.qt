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
	"scouter.client.qt/qtutil"
	"scouter.client.qt/server"
)

// Bar rendering constants
const (
	eqBarW    = 6  // max width of each vertical bar
	eqAxisPad = 16 // top padding for axis labels
	eqCountW  = 30 // left count column width
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
	Alive       bool
}

// eqWidget is the custom-painted EQ widget
type eqWidget struct {
	*qt6.QWidget
	mu    sync.RWMutex
	data  []EqData
	unitH int // cached row height from last paint
}

func newEqWidget(parent *qt6.QWidget) *eqWidget {
	w := &eqWidget{}
	if parent != nil {
		w.QWidget = qt6.NewQWidget(parent)
	} else {
		w.QWidget = qt6.NewQWidget2()
	}
	w.QWidget.SetMinimumHeight(40)
	w.QWidget.SetMinimumWidth(100)

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

	widgetW := w.QWidget.Width()
	widgetH := w.QWidget.Height()

	// Colors
	grayColor := qt6.NewQColor3(128, 128, 128)
	dimColor := qt6.NewQColor3(100, 100, 100)
	darkColor := qt6.NewQColor3(80, 80, 80)
	colorAct1 := qt6.NewQColor3(59, 130, 246)  // blue (normal)
	colorAct2 := qt6.NewQColor3(234, 179, 8)   // yellow (slow)
	colorAct3 := qt6.NewQColor3(239, 68, 68)   // red (very slow)
	bgNameColor := qt6.NewQColor3(200, 200, 200)
	rowBorderColor := qt6.NewQColor3(220, 220, 220)

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
	if unitH < 20 {
		unitH = 20
	}
	w.unitH = unitH

	// Find max total across all rows for scale
	var maxTotal int32
	for _, d := range data {
		total := d.Speed.Act1 + d.Speed.Act2 + d.Speed.Act3
		if total > maxTotal {
			maxTotal = total
		}
	}
	if maxTotal < 10 {
		maxTotal = 10
	}

	barStartX := eqCountW
	barSpace := widgetW - barStartX

	// Draw axis labels at top
	painter.SetPen(dimColor)
	smallFont := qt6.NewQFont()
	smallFont.SetPointSize(8)
	painter.SetFont(smallFont)

	halfMax := maxTotal / 2
	painter.DrawText3(barStartX, eqAxisPad-3, "0")
	if barSpace > 100 {
		midX := barStartX + barSpace/2
		painter.DrawText3(midX, eqAxisPad-3, fmt.Sprintf("%d", halfMax))
	}
	painter.DrawText3(barStartX+barSpace-30, eqAxisPad-3, fmt.Sprintf("%d", maxTotal))

	nameFont := qt6.NewQFont()
	nameFont.SetPointSize(9)

	bgNameFont := qt6.NewQFont()
	bgNameFont.SetPointSize(9)

	rowBorderPen := qt6.NewQPen3(rowBorderColor)

	// Scale text height proportionally to row height
	textH := unitH / 4
	if textH < 10 {
		textH = 10
	}
	if textH > 16 {
		textH = 16
	}

	// Scale bar width based on row height
	barW := unitH / 10
	if barW < 3 {
		barW = 3
	}
	if barW > eqBarW {
		barW = eqBarW
	}
	barGap := barW / 3
	if barGap < 1 {
		barGap = 1
	}

	for i, d := range data {
		y := eqAxisPad + i*unitH

		// Draw row separator line
		painter.SetPenWithPen(rowBorderPen)
		painter.DrawLine2(barStartX, y+unitH, widgetW, y+unitH)

		// Draw agent name as background text (bottom-right of row)
		painter.SetPen(bgNameColor)
		if !d.Alive {
			strikeFont := qt6.NewQFont()
			strikeFont.SetPointSize(9)
			strikeFont.SetStrikeOut(true)
			painter.SetFont(strikeFont)
		} else {
			painter.SetFont(bgNameFont)
		}
		nameRect := qt6.NewQRect4(barStartX+4, y, barSpace-8, unitH)
		painter.DrawText6(nameRect, int(qt6.AlignBottom|qt6.AlignRight), d.DisplayName)

		total := d.Speed.Act1 + d.Speed.Act2 + d.Speed.Act3

		// Draw count on left
		painter.SetPen(dimColor)
		painter.SetFont(nameFont)
		countRect := qt6.NewQRect4(0, y, eqCountW-2, unitH)
		painter.DrawText6(countRect, int(qt6.AlignVCenter|qt6.AlignRight), fmt.Sprintf("%d", total))

		if total == 0 {
			continue
		}

		// Equalizer style: one vertical bar per active service, colored by type
		barMaxH := unitH - textH - 6
		if barMaxH < 8 {
			barMaxH = 8
		}
		barX := barStartX + 4
		barBottom := y + unitH - textH - 2

		// Draw each bar as a vertical column from bottom up
		var barIdx int
		drawBars := func(count int32, color *qt6.QColor) {
			for j := int32(0); j < count; j++ {
				x := barX + barIdx*(barW+barGap)
				painter.FillRect5(x, barBottom-barMaxH, barW, barMaxH, color)
				barIdx++
			}
		}
		drawBars(d.Speed.Act1, colorAct1)
		drawBars(d.Speed.Act2, colorAct2)
		drawBars(d.Speed.Act3, colorAct3)

		// Draw total count next to the bars
		totalX := barX + barIdx*(barW+barGap) + 4
		painter.SetPen(dimColor)
		painter.SetFont(smallFont)
		painter.DrawText3(totalX, barBottom-barMaxH+12, fmt.Sprintf("%d", total))

		// Draw breakdown text below bars: (act1 / act2 / act3)
		painter.SetPen(dimColor)
		painter.SetFont(smallFont)
		breakdownText := fmt.Sprintf("(%d / %d / %d)", d.Speed.Act1, d.Speed.Act2, d.Speed.Act3)
		painter.DrawText3(barX, barBottom+textH-2, breakdownText)
	}

	// Draw vertical axis line
	axisLinePen := qt6.NewQPen3(darkColor)
	painter.SetPenWithPen(axisLinePen)
	painter.DrawLine2(barStartX, eqAxisPad, barStartX, eqAxisPad+n*unitH)
}

// GroupEQView is the dock widget wrapper for the EQ view
type GroupEQView struct {
	dock               *qt6.QDockWidget
	widget             *eqWidget
	id                 int
	groupName          string
	objType            string
	timer              *qt6.QTimer
	active             bool
	lastData           map[int32]ActiveSpeedData // retain previous data until new arrives
	onAgentDoubleClick func(objHash int32, objType string)
}

// NewGroupEQView creates a new group EQ dock view
func NewGroupEQView(mainWindow *qt6.QMainWindow, groupName, objType string) *GroupEQView {
	return NewGroupEQViewWithID(mainWindow, 0, groupName, objType)
}

// NewGroupEQViewWithID creates a new group EQ dock view with a specific ID
func NewGroupEQViewWithID(mainWindow *qt6.QMainWindow, id int, groupName, objType string) *GroupEQView {
	v := &GroupEQView{
		id:        id,
		groupName: groupName,
		objType:   objType,
		active:    true,
		lastData:  make(map[int32]ActiveSpeedData),
	}

	title := fmt.Sprintf("%s - Active Service EQ", groupName)

	v.dock = qt6.NewQDockWidget2(title)
	objectName := fmt.Sprintf("eqDock_%d_%s", id, groupName)
	qtutil.SetObjectName(v.dock.QWidget.QObject, objectName)
	v.dock.SetAllowedAreas(qt6.AllDockWidgetAreas)

	v.widget = newEqWidget(nil)
	v.dock.SetWidget(v.widget.QWidget)

	// Double-click handler: identify the agent bar that was clicked
	v.widget.QWidget.OnMouseDoubleClickEvent(func(super func(event *qt6.QMouseEvent), event *qt6.QMouseEvent) {
		super(event)
		if v.onAgentDoubleClick == nil {
			return
		}
		pos := event.Pos()
		y := pos.Y()
		unitH := v.widget.unitH
		if unitH == 0 {
			return
		}
		if y <= eqAxisPad {
			return
		}
		v.widget.mu.RLock()
		data := make([]EqData, len(v.widget.data))
		copy(data, v.widget.data)
		v.widget.mu.RUnlock()

		index := (y - eqAxisPad) / unitH
		if index < 0 || index >= len(data) {
			return
		}
		d := data[index]
		if !d.Alive {
			return
		}
		v.onAgentDoubleClick(d.ObjHash, v.objType)
	})

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

	// Merge new results into lastData (keep previous for agents that didn't respond)
	for hash, speed := range results {
		v.lastData[hash] = speed
	}

	// Remove agents no longer in the group
	for hash := range v.lastData {
		if _, ok := members[int(hash)]; !ok {
			delete(v.lastData, hash)
		}
	}

	if len(v.lastData) == 0 {
		return
	}

	// Build sorted display data from merged data
	eqData := make([]EqData, 0, len(v.lastData))
	objCache := cache.GetObjectCache()
	for hash, speed := range v.lastData {
		name := objCache.GetObjName(hash)
		if name == "" {
			name = fmt.Sprintf("obj-%d", hash)
		}
		alive := true
		if obj := objCache.Get(hash); obj != nil {
			alive = obj.Alive
		}
		eqData = append(eqData, EqData{
			ObjHash:     hash,
			DisplayName: name,
			Speed:       speed,
			Alive:       alive,
		})
	}
	sort.Slice(eqData, func(i, j int) bool {
		return eqData[i].DisplayName < eqData[j].DisplayName
	})

	v.widget.setData(eqData)
}

// Dock returns the underlying dock widget
func (v *GroupEQView) Dock() *qt6.QDockWidget { return v.dock }

// ID returns the view ID
func (v *GroupEQView) ID() int { return v.id }

// GroupName returns the group name
func (v *GroupEQView) GroupName() string { return v.groupName }

// ObjType returns the object type
func (v *GroupEQView) ObjType() string { return v.objType }

// SetOnAgentDoubleClick sets the callback for double-clicking an agent bar
func (v *GroupEQView) SetOnAgentDoubleClick(cb func(objHash int32, objType string)) {
	v.onAgentDoubleClick = cb
}

// Close closes the view and cleans up
func (v *GroupEQView) Close() {
	v.active = false
	if v.timer != nil {
		v.timer.Stop()
	}
	v.dock.Close()
}
