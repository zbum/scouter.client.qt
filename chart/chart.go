package chart

import (
	"fmt"
	"time"

	"github.com/mappu/miqt/qt6"
)

// ThemeColors holds colors for light/dark themes
type ThemeColors struct {
	Background      *qt6.QColor
	ChartBackground *qt6.QColor
	GridColor       *qt6.QColor
	AxisColor       *qt6.QColor
	TextColor       *qt6.QColor
	TitleColor      *qt6.QColor
	MarkerColor     *qt6.QColor
	LineColor       *qt6.QColor
}

// isDarkMode detects if the system is in dark mode
func isDarkMode() bool {
	palette := qt6.QGuiApplication_Palette()
	windowColor := palette.Window().Color()
	// If the window background is dark (lightness < 128), we're in dark mode
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
			TitleColor:      qt6.NewQColor3(220, 220, 240),
			MarkerColor:     qt6.NewQColor3(255, 100, 100),
			LineColor:       qt6.NewQColor3(100, 150, 255),
		}
	}
	// Light mode colors
	return &ThemeColors{
		Background:      qt6.NewQColor3(245, 245, 250),
		ChartBackground: qt6.NewQColor3(255, 255, 255),
		GridColor:       qt6.NewQColor3(200, 200, 210),
		AxisColor:       qt6.NewQColor3(80, 80, 100),
		TextColor:       qt6.NewQColor3(50, 50, 70),
		TitleColor:      qt6.NewQColor3(30, 30, 50),
		MarkerColor:     qt6.NewQColor3(200, 50, 50),
		LineColor:       qt6.NewQColor3(50, 100, 200),
	}
}

// DataPoint represents a single data point in the chart
type DataPoint struct {
	Timestamp time.Time
	Value     int64 // milliseconds
}

// Series represents a named data series with its own color
type Series struct {
	Name       string
	Color      *qt6.QColor
	DataPoints []DataPoint
}

// Config holds chart configuration
type Config struct {
	MaxPoints      int                // Maximum number of points to display
	MaxValue       int64              // Maximum Y value (milliseconds)
	TimeRange      int                // Time range in seconds
	Title          string
	MinWidth       int
	MinHeight      int
	ShowMarkers    bool               // Show X markers at data points
	ShowLines      bool               // Show lines connecting data points
	YAxisFormatter func(int64) string // Custom Y-axis label formatter (nil = default "%dms")
}

// FormatAbbreviated formats a value using abbreviated notation (1K, 2.5M)
func FormatAbbreviated(v int64) string {
	if v >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(v)/1_000_000)
	}
	if v >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(v)/1_000)
	}
	return fmt.Sprintf("%d", v)
}

// DefaultConfig returns default chart configuration
func DefaultConfig() Config {
	return Config{
		MaxPoints:   60,
		MaxValue:    1000,
		TimeRange:   60,
		Title:       "Response Time",
		MinWidth:    300,
		MinHeight:   200,
		ShowMarkers: true,
		ShowLines:   true,
	}
}

// seriesPalette defines 8 distinct colors that work in both light and dark mode
var seriesPalette = [][3]int{
	{70, 130, 230},  // Blue
	{220, 60, 60},   // Red
	{50, 170, 80},   // Green
	{230, 150, 30},  // Orange
	{160, 90, 210},  // Purple
	{30, 180, 190},  // Cyan
	{210, 90, 160},  // Pink
	{120, 190, 50},  // Lime
}

// Widget represents a time-series chart widget
type Widget struct {
	widget      *qt6.QWidget
	dataPoints  []DataPoint    // single-series (backward compat)
	series      map[string]*Series
	seriesOrder []string       // insertion order for consistent rendering
	config      Config
	colorIndex  int            // next color from palette
}

// New creates a new chart widget with default configuration
func New(parent *qt6.QWidget) *Widget {
	return NewWithConfig(parent, DefaultConfig())
}

// Key constants
const (
	KeyUp   = 16777235
	KeyDown = 16777237
)

// Scale step for Y-axis adjustment
const scaleStep int64 = 100

// NewWithConfig creates a new chart widget with custom configuration
func NewWithConfig(parent *qt6.QWidget, config Config) *Widget {
	chart := &Widget{
		widget: qt6.NewQWidget(parent),
		config: config,
	}

	chart.widget.SetMinimumSize2(config.MinWidth, config.MinHeight)
	// Don't set stylesheet - colors will be determined by system theme in paint()

	// Enable focus for keyboard events
	chart.widget.SetFocusPolicy(qt6.StrongFocus)

	// Setup paint event
	chart.widget.OnPaintEvent(func(super func(event *qt6.QPaintEvent), event *qt6.QPaintEvent) {
		chart.paint()
	})

	// Setup keyboard event for Y-axis scale adjustment
	chart.widget.OnKeyPressEvent(func(super func(event *qt6.QKeyEvent), event *qt6.QKeyEvent) {
		key := event.Key()
		switch key {
		case KeyUp:
			// Increase max value (zoom out)
			chart.config.MaxValue += scaleStep
			chart.widget.Update()
		case KeyDown:
			// Decrease max value (zoom in)
			if chart.config.MaxValue > scaleStep {
				chart.config.MaxValue -= scaleStep
				chart.widget.Update()
			}
		default:
			super(event)
		}
	})

	return chart
}

// QWidget returns the underlying Qt widget
func (c *Widget) QWidget() *qt6.QWidget {
	return c.widget
}

// AddPoint adds a new data point to the chart
func (c *Widget) AddPoint(value int64) {
	c.dataPoints = append(c.dataPoints, DataPoint{
		Timestamp: time.Now(),
		Value:     value,
	})

	// Keep only last maxPoints
	if len(c.dataPoints) > c.config.MaxPoints {
		c.dataPoints = c.dataPoints[1:]
	}

	c.widget.Update()
}

// Clear removes all data points from the chart
func (c *Widget) Clear() {
	c.dataPoints = nil
	c.widget.Update()
}

// SetMaxValue sets the maximum Y-axis value
func (c *Widget) SetMaxValue(value int64) {
	c.config.MaxValue = value
	c.widget.Update()
}

// SetTitle sets the chart title
func (c *Widget) SetTitle(title string) {
	c.config.Title = title
	c.widget.Update()
}

// Title returns the chart title
func (c *Widget) Title() string {
	return c.config.Title
}

// SetShowMarkers enables or disables X markers at data points
func (c *Widget) SetShowMarkers(show bool) {
	c.config.ShowMarkers = show
	c.widget.Update()
}

// SetShowLines enables or disables lines connecting data points
func (c *Widget) SetShowLines(show bool) {
	c.config.ShowLines = show
	c.widget.Update()
}

// ShowMarkers returns whether X markers are enabled
func (c *Widget) ShowMarkers() bool {
	return c.config.ShowMarkers
}

// ShowLines returns whether connecting lines are enabled
func (c *Widget) ShowLines() bool {
	return c.config.ShowLines
}

// DataPoints returns a copy of current data points
func (c *Widget) DataPoints() []DataPoint {
	result := make([]DataPoint, len(c.dataPoints))
	copy(result, c.dataPoints)
	return result
}

// AddSeries creates a named series with an auto-assigned color
func (c *Widget) AddSeries(name string) *Series {
	if c.series == nil {
		c.series = make(map[string]*Series)
	}
	if s, ok := c.series[name]; ok {
		return s
	}
	color := seriesPalette[c.colorIndex%len(seriesPalette)]
	s := &Series{
		Name:  name,
		Color: qt6.NewQColor3(color[0], color[1], color[2]),
	}
	c.series[name] = s
	c.seriesOrder = append(c.seriesOrder, name)
	c.colorIndex++
	return s
}

// AddSeriesPoint adds a data point to a named series (auto-creates if missing)
func (c *Widget) AddSeriesPoint(name string, value int64) {
	s := c.AddSeries(name)
	s.DataPoints = append(s.DataPoints, DataPoint{
		Timestamp: time.Now(),
		Value:     value,
	})
	if len(s.DataPoints) > c.config.MaxPoints {
		s.DataPoints = s.DataPoints[1:]
	}
	c.widget.Update()
}

// ClearAllSeries removes all series data
func (c *Widget) ClearAllSeries() {
	c.series = nil
	c.seriesOrder = nil
	c.colorIndex = 0
	c.widget.Update()
}

func (c *Widget) paint() {
	painter := qt6.NewQPainter2(c.widget.QPaintDevice)
	defer painter.Delete()

	painter.SetRenderHint(qt6.QPainter__Antialiasing)

	// Get theme colors
	theme := getThemeColors()

	width := c.widget.Width()
	height := c.widget.Height()

	// Margins
	marginLeft := 50
	marginRight := 20
	marginTop := 20
	marginBottom := 40

	chartWidth := width - marginLeft - marginRight
	chartHeight := height - marginTop - marginBottom

	// Draw background
	painter.FillRect5(0, 0, width, height, theme.Background)

	// Draw chart area background
	painter.FillRect5(marginLeft, marginTop, chartWidth, chartHeight, theme.ChartBackground)

	// Draw grid lines
	gridPen := qt6.NewQPen3(theme.GridColor)
	gridPen.SetStyle(qt6.DotLine)
	painter.SetPenWithPen(gridPen)

	// Horizontal grid lines (Y axis)
	for i := 0; i <= 5; i++ {
		y := marginTop + (chartHeight * i / 5)
		painter.DrawLine2(marginLeft, y, marginLeft+chartWidth, y)
	}

	// Calculate grid positions anchored to real clock times (smooth scrolling)
	now := time.Now()

	// Determine grid interval based on chart width
	maxLabels := chartWidth / 60
	if maxLabels < 2 {
		maxLabels = 2
	}
	if maxLabels > 6 {
		maxLabels = 6
	}
	gridInterval := c.config.TimeRange / maxLabels
	if gridInterval < 1 {
		gridInterval = 1
	}

	type gridLine struct {
		x    int
		time time.Time
	}
	var gridLines []gridLine

	// Anchor grid lines to round clock seconds (e.g., every 10s at :00, :10, :20...)
	// Find the most recent round time
	nowUnix := now.Unix()
	lastRound := nowUnix - (nowUnix % int64(gridInterval))
	startUnix := now.Add(-time.Duration(c.config.TimeRange) * time.Second).Unix()

	// Fractional offset for sub-second smooth scrolling
	fracOffset := float64(now.UnixMilli()%1000) / 1000.0
	pixPerSec := float64(chartWidth) / float64(c.config.TimeRange)

	for ts := lastRound; ts >= startUnix; ts -= int64(gridInterval) {
		secsAgo := float64(nowUnix-ts) + fracOffset
		xPos := marginLeft + chartWidth - int(secsAgo*pixPerSec)
		if xPos >= marginLeft && xPos <= marginLeft+chartWidth {
			gridLines = append(gridLines, gridLine{x: xPos, time: time.Unix(ts, 0)})
		}
	}

	// Draw vertical grid lines
	for _, gl := range gridLines {
		painter.DrawLine2(gl.x, marginTop, gl.x, marginTop+chartHeight)
	}

	// Draw axes
	axisPen := qt6.NewQPen3(theme.AxisColor)
	axisPen.SetWidth(2)
	painter.SetPenWithPen(axisPen)
	painter.DrawLine2(marginLeft, marginTop, marginLeft, marginTop+chartHeight)
	painter.DrawLine2(marginLeft, marginTop+chartHeight, marginLeft+chartWidth, marginTop+chartHeight)

	// Draw Y axis labels
	painter.SetPen(theme.TextColor)
	for i := 0; i <= 5; i++ {
		y := marginTop + (chartHeight * i / 5)
		yValue := c.config.MaxValue - (int64(i) * c.config.MaxValue / 5)
		var label string
		if c.config.YAxisFormatter != nil {
			label = c.config.YAxisFormatter(yValue)
		} else {
			label = fmt.Sprintf("%dms", yValue)
		}
		painter.DrawText3(5, y+5, label)
	}

	// Draw X axis labels (time)
	painter.SetPen(theme.TextColor)
	for _, gl := range gridLines {
		painter.DrawText3(gl.x-20, height-5, gl.time.Format("15:04:05"))
	}

	// Draw data
	now = time.Now()
	timeRange := float64(c.config.TimeRange)

	if len(c.seriesOrder) > 0 {
		// Multi-series mode
		for _, name := range c.seriesOrder {
			s, ok := c.series[name]
			if !ok || len(s.DataPoints) < 2 {
				continue
			}

			pen := qt6.NewQPen3(s.Color)
			pen.SetWidth(1)

			var prevX, prevY int
			var hasPrev bool

			for _, point := range s.DataPoints {
				secondsAgo := now.Sub(point.Timestamp).Seconds()
				if secondsAgo > timeRange {
					continue
				}

				xPos := marginLeft + int(float64(chartWidth)*(1.0-secondsAgo/timeRange))
				yPos := marginTop + chartHeight - int(float64(chartHeight)*float64(point.Value)/float64(c.config.MaxValue))

				if yPos < marginTop {
					yPos = marginTop
				}
				if yPos > marginTop+chartHeight {
					yPos = marginTop + chartHeight
				}

				if c.config.ShowLines && hasPrev {
					painter.SetPenWithPen(pen)
					painter.DrawLine2(prevX, prevY, xPos, yPos)
				}

				if c.config.ShowMarkers {
					painter.SetPenWithPen(pen)
					markerSize := 2
					painter.DrawLine2(xPos-markerSize, yPos-markerSize, xPos+markerSize, yPos+markerSize)
					painter.DrawLine2(xPos-markerSize, yPos+markerSize, xPos+markerSize, yPos-markerSize)
				}

				prevX = xPos
				prevY = yPos
				hasPrev = true
			}
		}
	} else if len(c.dataPoints) > 1 && (c.config.ShowMarkers || c.config.ShowLines) {
		// Single-series mode (backward compat)
		var markerPen *qt6.QPen
		var linePen *qt6.QPen

		if c.config.ShowMarkers {
			markerPen = qt6.NewQPen3(theme.MarkerColor)
			markerPen.SetWidth(1)
		}

		if c.config.ShowLines {
			linePen = qt6.NewQPen3(theme.LineColor)
			linePen.SetWidth(1)
		}

		var prevX, prevY int
		var hasPrev bool

		for _, point := range c.dataPoints {
			secondsAgo := now.Sub(point.Timestamp).Seconds()
			if secondsAgo > timeRange {
				continue
			}

			xPos := marginLeft + int(float64(chartWidth)*(1.0-secondsAgo/timeRange))
			yPos := marginTop + chartHeight - int(float64(chartHeight)*float64(point.Value)/float64(c.config.MaxValue))

			if yPos < marginTop {
				yPos = marginTop
			}
			if yPos > marginTop+chartHeight {
				yPos = marginTop + chartHeight
			}

			if c.config.ShowLines && hasPrev {
				painter.SetPenWithPen(linePen)
				painter.DrawLine2(prevX, prevY, xPos, yPos)
			}

			if c.config.ShowMarkers {
				painter.SetPenWithPen(markerPen)
				markerSize := 2
				painter.DrawLine2(xPos-markerSize, yPos-markerSize, xPos+markerSize, yPos+markerSize)
				painter.DrawLine2(xPos-markerSize, yPos+markerSize, xPos+markerSize, yPos-markerSize)
			}

			prevX = xPos
			prevY = yPos
			hasPrev = true
		}
	}

	painter.End()
}