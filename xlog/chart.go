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
	EndTime  time.Time // Transaction end time (X-axis)
	Elapsed  int32     // Response time in ms (Y-axis)
	TxID     int64     // Transaction ID
	Service  int32     // Service hash
	ObjHash  int32     // Object hash
	IsError  bool      // Whether this transaction had an error
	Selected bool      // Whether this point is selected
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
		TimeRange:  60,
		MinWidth:   400,
		MinHeight:  300,
		PointSize:  4,
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

	// Callbacks
	onPointSelected func(txID int64)
	onRangeSelected func(txIDs []int64)

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

	return c
}

// QWidget returns the underlying Qt widget
func (c *Chart) QWidget() *qt6.QWidget {
	return c.widget
}

// AddPoint adds a transaction point to the chart (thread-safe, no Qt calls)
func (c *Chart) AddPoint(point XLogPoint) {
	c.mu.Lock()
	defer c.mu.Unlock()

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
	c.widget.Update()
}

// ClearOldPoints removes points older than the time range
func (c *Chart) ClearOldPoints() {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-time.Duration(c.config.TimeRange) * time.Second)
	newPoints := make([]XLogPoint, 0, len(c.points))
	for _, p := range c.points {
		if p.EndTime.After(cutoff) {
			newPoints = append(newPoints, p)
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
func (c *Chart) SetOnPointSelected(callback func(txID int64)) {
	c.onPointSelected = callback
}

// SetOnRangeSelected sets callback for range selection
func (c *Chart) SetOnRangeSelected(callback func(txIDs []int64)) {
	c.onRangeSelected = callback
}

func (c *Chart) paint() {
	painter := qt6.NewQPainter2(c.widget.QPaintDevice)
	defer painter.Delete()

	painter.SetRenderHint(qt6.QPainter__Antialiasing)

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

	// Draw points
	c.drawPoints(painter, theme, marginLeft, marginTop, chartWidth, chartHeight)

	// Draw selection rectangle
	if c.selecting {
		c.drawSelection(painter, theme)
	}

	// Draw axes labels
	c.drawAxes(painter, theme, marginLeft, marginTop, chartWidth, chartHeight, height)
}

func (c *Chart) drawGrid(painter *qt6.QPainter, theme *ThemeColors, left, top, w, h int) {
	gridPen := qt6.NewQPen3(theme.GridColor)
	gridPen.SetStyle(qt6.DashLine)
	painter.SetPenWithPen(gridPen)

	// Horizontal grid lines (5 lines)
	for i := 1; i < 5; i++ {
		y := top + (h * i / 5)
		painter.DrawLine2(left, y, left+w, y)
	}

	// Vertical grid lines (6 lines for time)
	for i := 1; i < 6; i++ {
		x := left + (w * i / 6)
		painter.DrawLine2(x, top, x, top+h)
	}
}

func (c *Chart) drawPoints(painter *qt6.QPainter, theme *ThemeColors, left, top, w, h int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.points) == 0 {
		return
	}

	now := time.Now()
	startTime := now.Add(-time.Duration(c.config.TimeRange) * time.Second)
	maxElapsed := float64(c.config.MaxElapsed)
	timeRange := float64(c.config.TimeRange * 1000) // Convert to ms

	// Create pens for different states
	normalPen := qt6.NewQPen3(theme.NormalColor)
	normalPen.SetWidth(1)

	errorPen := qt6.NewQPen3(theme.ErrorColor)
	errorPen.SetWidth(1)

	selectedPen := qt6.NewQPen3(theme.SelectionColor)
	selectedPen.SetWidth(1)

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

		// Choose color based on state
		if c.selectedTxIDs[p.TxID] {
			painter.SetPenWithPen(selectedPen)
			painter.SetBrush(qt6.NewQBrush3(theme.SelectionColor))
		} else if p.IsError {
			painter.SetPenWithPen(errorPen)
			painter.SetBrush(qt6.NewQBrush3(theme.ErrorColor))
		} else {
			painter.SetPenWithPen(normalPen)
			painter.SetBrush(qt6.NewQBrush3(theme.NormalColor))
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

func (c *Chart) drawAxes(painter *qt6.QPainter, theme *ThemeColors, left, top, w, h, totalHeight int) {
	painter.SetPen(theme.TextColor)

	// Y-axis labels
	for i := 0; i <= 5; i++ {
		val := c.config.MaxElapsed * int32(5-i) / 5
		y := top + (h * i / 5)
		text := formatElapsed(val)
		painter.DrawText3(5, y+4, text)
	}

	// X-axis labels (time)
	now := time.Now()
	for i := 0; i <= 6; i++ {
		offset := time.Duration(c.config.TimeRange*(6-i)/6) * time.Second
		t := now.Add(-offset)
		x := left + (w * i / 6)
		text := t.Format("15:04:05")
		painter.DrawText3(x-25, top+h+15, text)
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

	now := time.Now()
	startTime := now.Add(-time.Duration(c.config.TimeRange) * time.Second)
	maxElapsed := float64(c.config.MaxElapsed)
	timeRange := float64(c.config.TimeRange * 1000)

	var closestTxID int64
	closestDist := 100.0 // Maximum click distance squared

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

		dist := distanceSquared(x, y, px, py)
		if dist < closestDist {
			closestDist = dist
			closestTxID = p.TxID
		}
	}

	if closestTxID != 0 && c.onPointSelected != nil {
		c.onPointSelected(closestTxID)
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

	now := time.Now()
	startTime := now.Add(-time.Duration(c.config.TimeRange) * time.Second)
	maxElapsed := float64(c.config.MaxElapsed)
	timeRange := float64(c.config.TimeRange * 1000)

	c.selectedTxIDs = make(map[int64]bool)
	var selectedIDs []int64

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
			selectedIDs = append(selectedIDs, p.TxID)
		}
	}

	if len(selectedIDs) > 0 && c.onRangeSelected != nil {
		c.onRangeSelected(selectedIDs)
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
