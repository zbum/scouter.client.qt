// Package xlog provides XLog (transaction log) visualization
package xlog

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mappu/miqt/qt6"
)

// XLogPoint represents a single transaction in the scatter chart
type XLogPoint struct {
	EndTime      time.Time // Transaction end time (X-axis)
	Elapsed      int32     // Response time in ms (Y-axis)
	TxID         int64     // Transaction ID
	GxID         int64     // Global transaction ID
	Service      int32     // Service hash
	ObjHash      int32     // Object hash
	ServerID     int       // Server ID (for color assignment)
	IsError      bool      // Whether this transaction had an error
	Selected     bool      // Whether this point is selected
	CPU          int32     // CPU time (ms)
	SQLCount     int32     // SQL call count
	SQLTime      int32     // SQL total time (ms)
	APICallCount int32     // API call count
	APICallTime  int32     // API call total time (ms)
	KBytes       int32     // Traffic KB
	IPAddr       []byte    // Client IP address
	Login        int32     // Login hash
	Desc         int32     // Description hash
	Error        int32     // Error hash
	UserAgent    int32     // User agent hash
	HasDump      byte      // Profile dump flag
}

// serverColorPalette defines distinct colors per server (no red - reserved for errors).
// Dark mode and light mode share the same palette; they are vivid enough for both.
var serverColorPalette = [][3]int{
	{100, 200, 255}, // blue
	{80, 200, 120},  // green
	{180, 140, 255}, // purple
	{255, 180, 60},  // orange
	{0, 200, 200},   // cyan
	{200, 160, 100}, // brown
	{160, 220, 80},  // lime
	{255, 130, 200}, // pink
}

// getServerColor returns a QColor for the given server ID (never red).
func getServerColor(serverID int) *qt6.QColor {
	idx := serverID % len(serverColorPalette)
	if idx < 0 {
		idx += len(serverColorPalette)
	}
	c := serverColorPalette[idx]
	return qt6.NewQColor3(c[0], c[1], c[2])
}

// ThemeColors holds colors for the XLog chart
type ThemeColors struct {
	Background      *qt6.QColor
	ChartBackground *qt6.QColor
	GridColor       *qt6.QColor
	AxisColor       *qt6.QColor
	TextColor       *qt6.QColor
	NormalColor     *qt6.QColor
	ErrorColor      *qt6.QColor
	SelectionColor  *qt6.QColor
}

// isDarkMode detects if the system is in dark mode
func isDarkMode() bool {
	palette := qt6.QGuiApplication_Palette()
	windowColor := palette.Window().Color()
	return windowColor.Lightness() < 128
}

// getThemeColors returns colors appropriate for the current theme
func getThemeColors() *ThemeColors {
	if isDarkMode() {
		return &ThemeColors{
			Background:      qt6.NewQColor3(26, 26, 46),
			ChartBackground: qt6.NewQColor3(30, 30, 50),
			GridColor:       qt6.NewQColor3(60, 60, 90),
			AxisColor:       qt6.NewQColor3(150, 150, 180),
			TextColor:       qt6.NewQColor3(200, 200, 220),
			NormalColor:     qt6.NewQColor3(100, 200, 255),
			ErrorColor:      qt6.NewQColor3(255, 80, 80),
			SelectionColor:  qt6.NewQColor3(255, 255, 100),
		}
	}
	return &ThemeColors{
		Background:      qt6.NewQColor3(245, 245, 250),
		ChartBackground: qt6.NewQColor3(255, 255, 255),
		GridColor:       qt6.NewQColor3(200, 200, 210),
		AxisColor:       qt6.NewQColor3(80, 80, 100),
		TextColor:       qt6.NewQColor3(50, 50, 70),
		NormalColor:     qt6.NewQColor3(30, 120, 200),
		ErrorColor:      qt6.NewQColor3(220, 50, 50),
		SelectionColor:  qt6.NewQColor3(255, 200, 0),
	}
}

// Config holds scatter chart configuration
type Config struct {
	MaxElapsed int32 // Maximum Y-axis value (ms)
	TimeRange  int   // Time range in seconds (X-axis)
	MinWidth   int
	MinHeight  int
	PointSize  int
}

// DefaultConfig returns default chart configuration
func DefaultConfig() Config {
	return Config{
		MaxElapsed: 5000,
		TimeRange:  300,
		MinWidth:   100,
		MinHeight:  160,
		PointSize:  3,
	}
}

// Chart represents an XLog scatter chart widget
type Chart struct {
	widget *qt6.QWidget
	config Config
	points []XLogPoint

	// Selection state
	selecting     bool
	selectStartX  int
	selectStartY  int
	selectEndX    int
	selectEndY    int
	selectedTxIDs map[int64]bool

	// De-duplication: track known TxIDs
	knownTxIDs map[int64]bool

	// X-axis time offset (seconds, positive = looking at past)
	timeOffset int

	// Callbacks
	onPointSelected  func(point XLogPoint)
	onRangeSelected  func(points []XLogPoint)
	onNeedPastData   func(stime, etime time.Time)

	mu           sync.RWMutex
	needsRepaint atomic.Bool // Set from goroutine, consumed by main-thread timer
}

// NewChart creates a new XLog scatter chart
func NewChart(parent *qt6.QWidget) *Chart {
	return NewChartWithConfig(parent, DefaultConfig())
}

// NewChartWithConfig creates a new chart with custom configuration
func NewChartWithConfig(parent *qt6.QWidget, config Config) *Chart {
	c := &Chart{
		widget:        qt6.NewQWidget(parent),
		config:        config,
		selectedTxIDs: make(map[int64]bool),
		knownTxIDs:    make(map[int64]bool),
	}

	c.widget.SetMinimumSize2(config.MinWidth, config.MinHeight)
	c.widget.SetMouseTracking(true)

	// Paint event
	c.widget.OnPaintEvent(func(super func(event *qt6.QPaintEvent), event *qt6.QPaintEvent) {
		c.paint()
	})

	// Mouse press event
	c.widget.OnMousePressEvent(func(super func(event *qt6.QMouseEvent), event *qt6.QMouseEvent) {
		c.handleMousePress(event)
	})

	// Mouse move event
	c.widget.OnMouseMoveEvent(func(super func(event *qt6.QMouseEvent), event *qt6.QMouseEvent) {
		c.handleMouseMove(event)
	})

	// Mouse release event
	c.widget.OnMouseReleaseEvent(func(super func(event *qt6.QMouseEvent), event *qt6.QMouseEvent) {
		c.handleMouseRelease(event)
	})

	// Double click event
	c.widget.OnMouseDoubleClickEvent(func(super func(event *qt6.QMouseEvent), event *qt6.QMouseEvent) {
		c.handleDoubleClick(event)
	})

	// Key event for arrow keys
	c.widget.SetFocusPolicy(qt6.StrongFocus)
	c.widget.OnKeyPressEvent(func(super func(event *qt6.QKeyEvent), event *qt6.QKeyEvent) {
		c.handleKeyPress(event)
	})

	return c
}

// QWidget returns the underlying Qt widget
func (c *Chart) QWidget() *qt6.QWidget {
	return c.widget
}

// viewNow returns the reference "now" time, shifted by timeOffset for panning
func (c *Chart) viewNow() time.Time {
	return time.Now().Add(-time.Duration(c.timeOffset) * time.Second)
}

func (c *Chart) handleKeyPress(event *qt6.QKeyEvent) {
	key := event.Key()
	switch qt6.Key(key) {
	case qt6.Key_Up:
		// Increase Y-axis max (see higher values) - double it, max 60s
		newMax := c.config.MaxElapsed * 2
		if newMax > 60000 {
			newMax = 60000
		}
		c.config.MaxElapsed = newMax
		c.widget.Update()
	case qt6.Key_Down:
		// Decrease Y-axis max (zoom in to lower values) - halve it, min 500ms
		newMax := c.config.MaxElapsed / 2
		if newMax < 500 {
			newMax = 500
		}
		c.config.MaxElapsed = newMax
		c.widget.Update()
	case qt6.Key_Left:
		// Move X-axis to past
		step := c.config.TimeRange / 4
		if step < 5 {
			step = 5
		}
		c.timeOffset += step
		c.widget.Update()
		// Request past data for the newly visible time range
		if c.onNeedPastData != nil {
			viewEnd := time.Now().Add(-time.Duration(c.timeOffset-step) * time.Second)
			viewStart := time.Now().Add(-time.Duration(c.timeOffset+c.config.TimeRange) * time.Second)
			c.onNeedPastData(viewStart, viewEnd)
		}
	case qt6.Key_Right:
		// Move X-axis to present
		step := c.config.TimeRange / 4
		if step < 5 {
			step = 5
		}
		c.timeOffset -= step
		if c.timeOffset < 0 {
			c.timeOffset = 0
		}
		c.widget.Update()
	}
}

// AddPoint adds a transaction point to the chart (thread-safe, no Qt calls).
// Duplicate TxIDs are silently ignored.
func (c *Chart) AddPoint(point XLogPoint) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.knownTxIDs[point.TxID] {
		return
	}
	c.knownTxIDs[point.TxID] = true
	c.points = append(c.points, point)
	c.needsRepaint.Store(true)
}

// AddPoints adds multiple transaction points (thread-safe, no Qt calls)
func (c *Chart) AddPoints(points []XLogPoint) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.points = append(c.points, points...)
	c.needsRepaint.Store(true)
}

// FlushRepaint calls widget.Update() if new data was added.
// Must be called from the main Qt thread (e.g., from a QTimer callback).
func (c *Chart) FlushRepaint() {
	if c.needsRepaint.CompareAndSwap(true, false) {
		c.widget.Update()
	}
}

// Clear removes all points
func (c *Chart) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.points = nil
	c.selectedTxIDs = make(map[int64]bool)
	c.knownTxIDs = make(map[int64]bool)
	c.widget.Update()
}

// ClearOldPoints removes points older than the time range
func (c *Chart) ClearOldPoints() {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-time.Duration(c.config.TimeRange+c.timeOffset) * time.Second)
	newPoints := make([]XLogPoint, 0, len(c.points))
	for _, p := range c.points {
		if p.EndTime.After(cutoff) {
			newPoints = append(newPoints, p)
		} else {
			delete(c.knownTxIDs, p.TxID)
		}
	}
	c.points = newPoints
	c.widget.Update()
}

// SetMaxElapsed sets the maximum Y-axis value
func (c *Chart) SetMaxElapsed(max int32) {
	c.config.MaxElapsed = max
	c.widget.Update()
}

// SetTimeRange sets the X-axis time range in seconds
func (c *Chart) SetTimeRange(seconds int) {
	c.config.TimeRange = seconds
	c.widget.Update()
}

// SetOnPointSelected sets callback for single point selection
func (c *Chart) SetOnPointSelected(callback func(point XLogPoint)) {
	c.onPointSelected = callback
}

// SetOnRangeSelected sets callback for range selection
func (c *Chart) SetOnRangeSelected(callback func(points []XLogPoint)) {
	c.onRangeSelected = callback
}

// SetOnNeedPastData sets a callback invoked when the chart scrolls to a time range needing data
func (c *Chart) SetOnNeedPastData(callback func(stime, etime time.Time)) {
	c.onNeedPastData = callback
}

func (c *Chart) paint() {
	painter := qt6.NewQPainter2(c.widget.QPaintDevice)
	defer painter.Delete()

	painter.SetRenderHint(qt6.QPainter__Antialiasing)

	// Set font size for axis labels
	scaleFont := painter.Font()
	scaleFont.SetPointSize(10)
	painter.SetFont(scaleFont)

	theme := getThemeColors()
	width := c.widget.Width()
	height := c.widget.Height()

	// Margins
	marginLeft := 60
	marginRight := 20
	marginTop := 20
	marginBottom := 40

	chartWidth := width - marginLeft - marginRight
	chartHeight := height - marginTop - marginBottom

	// Draw background
	painter.FillRect5(0, 0, width, height, theme.Background)
	painter.FillRect5(marginLeft, marginTop, chartWidth, chartHeight, theme.ChartBackground)

	// Draw grid
	c.drawGrid(painter, theme, marginLeft, marginTop, chartWidth, chartHeight)

	// Clip drawing to chart area so points don't overflow boundaries
	painter.SetClipRect2(marginLeft, marginTop, chartWidth, chartHeight)

	// Draw points
	c.drawPoints(painter, theme, marginLeft, marginTop, chartWidth, chartHeight)

	// Draw selection rectangle
	if c.selecting {
		c.drawSelection(painter, theme)
	}

	// Remove clip for axes drawing
	painter.SetClipping(false)

	// Draw axes labels
	c.drawAxes(painter, theme, marginLeft, marginTop, chartWidth, chartHeight, height)
}

func (c *Chart) drawGrid(painter *qt6.QPainter, theme *ThemeColors, left, top, w, h int) {
	// Horizontal grid lines (5 lines) - pastel
	hGridColor := qt6.NewQColor3(215, 222, 232)
	if isDarkMode() {
		hGridColor = qt6.NewQColor3(55, 62, 78)
	}
	hGridPen := qt6.NewQPen3(hGridColor)
	hGridPen.SetStyle(qt6.DashLine)
	painter.SetPenWithPen(hGridPen)

	for i := 1; i < 5; i++ {
		y := top + (h * i / 5)
		painter.DrawLine2(left, y, left+w, y)
	}

	// Vertical grid lines at 10s intervals with pastel tones
	// 60s (minute boundary) = solid, slightly stronger pastel
	// 30s = solid, lighter pastel
	// 10s = dotted, very light pastel
	minuteColor := qt6.NewQColor3(180, 200, 220)  // pastel blue-gray
	halfMinColor := qt6.NewQColor3(210, 220, 230)  // lighter pastel
	tenSecColor := qt6.NewQColor3(225, 230, 240)   // very light pastel
	if isDarkMode() {
		minuteColor = qt6.NewQColor3(80, 90, 110)
		halfMinColor = qt6.NewQColor3(60, 68, 85)
		tenSecColor = qt6.NewQColor3(48, 55, 70)
	}

	minutePen := qt6.NewQPen3(minuteColor)
	minutePen.SetWidth(1)
	minutePen.SetStyle(qt6.SolidLine)

	halfMinPen := qt6.NewQPen3(halfMinColor)
	halfMinPen.SetWidth(1)
	halfMinPen.SetStyle(qt6.SolidLine)

	tenSecPen := qt6.NewQPen3(tenSecColor)
	tenSecPen.SetStyle(qt6.DotLine)

	for _, gl := range c.calcTimeGridLines(left, w) {
		if gl.seconds == 0 {
			painter.SetPenWithPen(minutePen)
		} else if gl.seconds == 30 {
			painter.SetPenWithPen(halfMinPen)
		} else {
			painter.SetPenWithPen(tenSecPen)
		}
		painter.DrawLine2(gl.x, top, gl.x, top+h)
	}
}

func (c *Chart) drawPoints(painter *qt6.QPainter, theme *ThemeColors, left, top, w, h int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.points) == 0 {
		return
	}

	now := c.viewNow()
	startTime := now.Add(-time.Duration(c.config.TimeRange) * time.Second)
	maxElapsed := float64(c.config.MaxElapsed)
	timeRange := float64(c.config.TimeRange * 1000) // Convert to ms

	// Pre-built pens/brushes
	errorPen := qt6.NewQPen3(theme.ErrorColor)
	errorPen.SetWidth(1)

	selectedPen := qt6.NewQPen3(theme.SelectionColor)
	selectedPen.SetWidth(1)

	// Cache server color pens/brushes to avoid repeated allocation
	serverPens := make(map[int]*qt6.QPen)
	serverBrushes := make(map[int]*qt6.QBrush)

	pointSize := c.config.PointSize

	for _, p := range c.points {
		if p.EndTime.Before(startTime) {
			continue
		}

		// Calculate position
		timeDiff := float64(p.EndTime.Sub(startTime).Milliseconds())
		x := left + int(timeDiff/timeRange*float64(w))

		elapsed := float64(p.Elapsed)
		if elapsed > maxElapsed {
			elapsed = maxElapsed
		}
		y := top + h - int(elapsed/maxElapsed*float64(h))

		// Choose color: selected > error (red) > agent color (by ObjHash)
		if c.selectedTxIDs[p.TxID] {
			painter.SetPenWithPen(selectedPen)
			painter.SetBrush(qt6.NewQBrush3(theme.SelectionColor))
		} else if p.IsError {
			painter.SetPenWithPen(errorPen)
			painter.SetBrush(qt6.NewQBrush3(theme.ErrorColor))
		} else {
			key := int(p.ObjHash)
			pen, ok := serverPens[key]
			if !ok {
				agentColor := getServerColor(key)
				pen = qt6.NewQPen3(agentColor)
				pen.SetWidth(1)
				serverPens[key] = pen
				serverBrushes[key] = qt6.NewQBrush3(agentColor)
			}
			painter.SetPenWithPen(pen)
			painter.SetBrush(serverBrushes[key])
		}

		// Draw point as a filled rectangle (simpler than ellipse)
		painter.DrawRect2(x-pointSize/2, y-pointSize/2, pointSize, pointSize)
	}
}

func (c *Chart) drawSelection(painter *qt6.QPainter, theme *ThemeColors) {
	pen := qt6.NewQPen3(theme.SelectionColor)
	pen.SetStyle(qt6.DashLine)
	painter.SetPenWithPen(pen)

	// Semi-transparent fill
	selColor := qt6.NewQColor3(255, 200, 0)
	selColor.SetAlpha(50)
	painter.SetBrush(qt6.NewQBrush3(selColor))

	x1, y1 := c.selectStartX, c.selectStartY
	x2, y2 := c.selectEndX, c.selectEndY

	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}

	painter.DrawRect2(x1, y1, x2-x1, y2-y1)
}

type xlogGridLine struct {
	x       int
	time    time.Time
	seconds int // second-of-minute (0-59)
}

// calcTimeGridLines computes grid line positions at 10-second intervals anchored to clock times
func (c *Chart) calcTimeGridLines(left, w int) []xlogGridLine {
	now := c.viewNow()
	nowUnix := now.Unix()
	// Align to 10-second boundary
	lastRound := nowUnix - (nowUnix % 10)
	startUnix := now.Add(-time.Duration(c.config.TimeRange) * time.Second).Unix()

	fracOffset := float64(now.UnixMilli()%1000) / 1000.0
	pixPerSec := float64(w) / float64(c.config.TimeRange)

	var lines []xlogGridLine
	for ts := lastRound; ts >= startUnix; ts -= 10 {
		secsAgo := float64(nowUnix-ts) + fracOffset
		xPos := left + w - int(secsAgo*pixPerSec)
		if xPos >= left && xPos <= left+w {
			t := time.Unix(ts, 0)
			lines = append(lines, xlogGridLine{x: xPos, time: t, seconds: t.Second()})
		}
	}
	return lines
}

func (c *Chart) drawAxes(painter *qt6.QPainter, theme *ThemeColors, left, top, w, h, totalHeight int) {
	// Draw solid border around chart area
	axisPen := qt6.NewQPen3(theme.AxisColor)
	axisPen.SetWidth(1)
	painter.SetPenWithPen(axisPen)
	painter.DrawLine2(left, top, left+w, top)           // top
	painter.DrawLine2(left, top+h, left+w, top+h)       // bottom
	painter.DrawLine2(left, top, left, top+h)            // left
	painter.DrawLine2(left+w, top, left+w, top+h)       // right

	painter.SetPen(theme.TextColor)

	// Y-axis labels
	for i := 0; i <= 5; i++ {
		val := c.config.MaxElapsed * int32(5-i) / 5
		y := top + (h * i / 5)
		text := formatElapsed(val)
		painter.DrawText3(5, y+4, text)
	}

	// X-axis labels at 30s and 60s boundaries only
	for _, gl := range c.calcTimeGridLines(left, w) {
		if gl.seconds == 0 || gl.seconds == 30 {
			painter.DrawText3(gl.x-22, top+h+13, gl.time.Format("15:04:05"))
		}
	}

	// Y-axis title
	painter.DrawText3(5, top-5, "ms")
}

func formatElapsed(ms int32) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	}
	return fmt.Sprintf("%dms", ms)
}

func (c *Chart) handleMousePress(event *qt6.QMouseEvent) {
	if event.Button() == qt6.LeftButton {
		c.selecting = true
		pos := event.Pos()
		c.selectStartX = pos.X()
		c.selectStartY = pos.Y()
		c.selectEndX = pos.X()
		c.selectEndY = pos.Y()
	}
}

func (c *Chart) handleMouseMove(event *qt6.QMouseEvent) {
	if c.selecting {
		pos := event.Pos()
		c.selectEndX = pos.X()
		c.selectEndY = pos.Y()
		c.widget.Update()
	}
}

func (c *Chart) handleMouseRelease(event *qt6.QMouseEvent) {
	if !c.selecting {
		return
	}

	c.selecting = false
	pos := event.Pos()
	c.selectEndX = pos.X()
	c.selectEndY = pos.Y()

	// Calculate selected area
	x1 := c.selectStartX
	y1 := c.selectStartY
	x2 := c.selectEndX
	y2 := c.selectEndY

	// If it's a small drag, treat as click
	if abs(x2-x1) < 5 && abs(y2-y1) < 5 {
		c.handleClick(x1, y1)
		return
	}

	// Range selection
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}

	c.selectPointsInRect(x1, y1, x2, y2)
	c.widget.Update()
}

func (c *Chart) handleDoubleClick(event *qt6.QMouseEvent) {
	// Clear selection
	c.selectedTxIDs = make(map[int64]bool)
	c.widget.Update()
}

func (c *Chart) handleClick(x, y int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Find closest point
	width := c.widget.Width()
	height := c.widget.Height()

	marginLeft := 60
	marginRight := 20
	marginTop := 20
	marginBottom := 40

	chartWidth := width - marginLeft - marginRight
	chartHeight := height - marginTop - marginBottom

	now := c.viewNow()
	startTime := now.Add(-time.Duration(c.config.TimeRange) * time.Second)
	maxElapsed := float64(c.config.MaxElapsed)
	timeRange := float64(c.config.TimeRange * 1000)

	var closestPoint *XLogPoint
	closestDist := 100.0 // Maximum click distance squared

	for i := range c.points {
		p := &c.points[i]
		if p.EndTime.Before(startTime) {
			continue
		}

		timeDiff := float64(p.EndTime.Sub(startTime).Milliseconds())
		px := marginLeft + int(timeDiff/timeRange*float64(chartWidth))

		elapsed := float64(p.Elapsed)
		if elapsed > maxElapsed {
			elapsed = maxElapsed
		}
		py := marginTop + chartHeight - int(elapsed/maxElapsed*float64(chartHeight))

		dist := distanceSquared(x, y, px, py)
		if dist < closestDist {
			closestDist = dist
			closestPoint = p
		}
	}

	if closestPoint != nil && c.onPointSelected != nil {
		c.onPointSelected(*closestPoint)
	}
}

func (c *Chart) selectPointsInRect(x1, y1, x2, y2 int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	width := c.widget.Width()
	height := c.widget.Height()

	marginLeft := 60
	marginRight := 20
	marginTop := 20
	marginBottom := 40

	chartWidth := width - marginLeft - marginRight
	chartHeight := height - marginTop - marginBottom

	now := c.viewNow()
	startTime := now.Add(-time.Duration(c.config.TimeRange) * time.Second)
	maxElapsed := float64(c.config.MaxElapsed)
	timeRange := float64(c.config.TimeRange * 1000)

	c.selectedTxIDs = make(map[int64]bool)
	var selectedPoints []XLogPoint

	for _, p := range c.points {
		if p.EndTime.Before(startTime) {
			continue
		}

		timeDiff := float64(p.EndTime.Sub(startTime).Milliseconds())
		px := marginLeft + int(timeDiff/timeRange*float64(chartWidth))

		elapsed := float64(p.Elapsed)
		if elapsed > maxElapsed {
			elapsed = maxElapsed
		}
		py := marginTop + chartHeight - int(elapsed/maxElapsed*float64(chartHeight))

		if px >= x1 && px <= x2 && py >= y1 && py <= y2 {
			c.selectedTxIDs[p.TxID] = true
			selectedPoints = append(selectedPoints, p)
		}
	}

	if len(selectedPoints) > 0 && c.onRangeSelected != nil {
		c.onRangeSelected(selectedPoints)
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func distanceSquared(x1, y1, x2, y2 int) float64 {
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	return dx*dx + dy*dy
}
