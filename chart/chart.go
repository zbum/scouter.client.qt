package chart

import (
	"fmt"
	"time"

	"github.com/mappu/miqt/qt6"
)

// DataPoint represents a single data point in the chart
type DataPoint struct {
	Timestamp time.Time
	Value     int64 // milliseconds
}

// Config holds chart configuration
type Config struct {
	MaxPoints  int   // Maximum number of points to display
	MaxValue   int64 // Maximum Y value (milliseconds)
	TimeRange  int   // Time range in seconds
	Title      string
	MinWidth   int
	MinHeight  int
}

// DefaultConfig returns default chart configuration
func DefaultConfig() Config {
	return Config{
		MaxPoints:  60,
		MaxValue:   1000,
		TimeRange:  60,
		Title:      "Response Time",
		MinWidth:   300,
		MinHeight:  200,
	}
}

// Widget represents a time-series chart widget
type Widget struct {
	widget     *qt6.QWidget
	dataPoints []DataPoint
	config     Config
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
	chart.widget.SetStyleSheet("background-color: #1a1a2e; border: 1px solid #4a4a6a; border-radius: 5px;")

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

// DataPoints returns a copy of current data points
func (c *Widget) DataPoints() []DataPoint {
	result := make([]DataPoint, len(c.dataPoints))
	copy(result, c.dataPoints)
	return result
}

func (c *Widget) paint() {
	painter := qt6.NewQPainter2(c.widget.QPaintDevice)
	defer painter.Delete()

	painter.SetRenderHint(qt6.QPainter__Antialiasing)

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
	bgColor := qt6.NewQColor3(26, 26, 46)
	painter.FillRect5(0, 0, width, height, bgColor)

	// Draw chart area background
	chartBgColor := qt6.NewQColor3(30, 30, 50)
	painter.FillRect5(marginLeft, marginTop, chartWidth, chartHeight, chartBgColor)

	// Draw grid lines
	gridColor := qt6.NewQColor3(60, 60, 90)
	gridPen := qt6.NewQPen3(gridColor)
	gridPen.SetStyle(qt6.DotLine)
	painter.SetPenWithPen(gridPen)

	// Horizontal grid lines (Y axis)
	for i := 0; i <= 5; i++ {
		y := marginTop + (chartHeight * i / 5)
		painter.DrawLine2(marginLeft, y, marginLeft+chartWidth, y)
	}

	// Calculate grid positions based on time
	now := time.Now()
	gridInterval := c.config.TimeRange / 6 // seconds between grid lines

	// Calculate time offset for smooth scrolling
	totalSeconds := float64(now.Second()) + float64(now.Nanosecond())/1e9
	secondsIntoInterval := totalSeconds - float64(int(totalSeconds)/gridInterval*gridInterval)

	// Store grid line positions and times for both grid and labels
	type gridLine struct {
		x    int
		time time.Time
	}
	var gridLines []gridLine

	for i := -1; i <= 7; i++ {
		// Calculate how many seconds ago this grid line represents
		secondsAgo := secondsIntoInterval + float64(i*gridInterval)
		if secondsAgo < 0 || secondsAgo > float64(c.config.TimeRange) {
			continue
		}

		xPos := marginLeft + int(float64(chartWidth)*(1.0-secondsAgo/float64(c.config.TimeRange)))
		if xPos >= marginLeft && xPos <= marginLeft+chartWidth {
			t := now.Add(-time.Duration(secondsAgo * float64(time.Second)))
			gridLines = append(gridLines, gridLine{x: xPos, time: t})
		}
	}

	// Draw vertical grid lines
	for _, gl := range gridLines {
		painter.DrawLine2(gl.x, marginTop, gl.x, marginTop+chartHeight)
	}

	// Draw axes
	axisColor := qt6.NewQColor3(150, 150, 180)
	axisPen := qt6.NewQPen3(axisColor)
	axisPen.SetWidth(2)
	painter.SetPenWithPen(axisPen)
	painter.DrawLine2(marginLeft, marginTop, marginLeft, marginTop+chartHeight)
	painter.DrawLine2(marginLeft, marginTop+chartHeight, marginLeft+chartWidth, marginTop+chartHeight)

	// Draw Y axis labels (milliseconds)
	textColor := qt6.NewQColor3(200, 200, 220)
	painter.SetPen(textColor)
	for i := 0; i <= 5; i++ {
		y := marginTop + (chartHeight * i / 5)
		msValue := c.config.MaxValue - (int64(i) * c.config.MaxValue / 5)
		painter.DrawText3(5, y+5, fmt.Sprintf("%dms", msValue))
	}

	// Draw X axis labels (time) - same positions as grid lines
	painter.SetPen(textColor)
	for _, gl := range gridLines {
		painter.DrawText3(gl.x-20, height-5, gl.time.Format("15:04:05"))
	}

	// Draw data points as X markers
	if len(c.dataPoints) > 1 {
		markerColor := qt6.NewQColor3(255, 100, 100)
		markerPen := qt6.NewQPen3(markerColor)
		markerPen.SetWidth(2)
		painter.SetPenWithPen(markerPen)

		// Line color for connecting points
		lineColor := qt6.NewQColor3(100, 150, 255)
		linePen := qt6.NewQPen3(lineColor)
		linePen.SetWidth(1)

		now := time.Now()
		timeRange := float64(c.config.TimeRange)

		var prevX, prevY int

		for i, point := range c.dataPoints {
			// Calculate position
			secondsAgo := now.Sub(point.Timestamp).Seconds()
			if secondsAgo > timeRange {
				continue
			}

			xPos := marginLeft + int(float64(chartWidth)*(1.0-secondsAgo/timeRange))
			yPos := marginTop + chartHeight - int(float64(chartHeight)*float64(point.Value)/float64(c.config.MaxValue))

			// Clamp Y position
			if yPos < marginTop {
				yPos = marginTop
			}
			if yPos > marginTop+chartHeight {
				yPos = marginTop + chartHeight
			}

			// Draw connecting line
			if i > 0 && prevX > 0 {
				painter.SetPenWithPen(linePen)
				painter.DrawLine2(prevX, prevY, xPos, yPos)
			}

			// Draw X marker
			painter.SetPenWithPen(markerPen)
			markerSize := 4
			painter.DrawLine2(xPos-markerSize, yPos-markerSize, xPos+markerSize, yPos+markerSize)
			painter.DrawLine2(xPos-markerSize, yPos+markerSize, xPos+markerSize, yPos-markerSize)

			prevX = xPos
			prevY = yPos
		}
	}

	// Draw title
	titleColor := qt6.NewQColor3(220, 220, 240)
	painter.SetPen(titleColor)
	painter.DrawText3(marginLeft+chartWidth/2-40, 15, c.config.Title)

	painter.End()
}