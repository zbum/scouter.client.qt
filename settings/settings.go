package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	configDir = "scouter.client.go"
	configFile = "settings.json"
)

// GroupChartConfig holds the configuration for a persisted group counter chart
type GroupChartConfig struct {
	ID          int    `json:"id"`
	GroupName   string `json:"groupName"`
	ObjType     string `json:"objType"`
	CounterName string `json:"counterName"`
	DisplayName string `json:"displayName"`
}

// XLogViewConfig holds the configuration for a persisted XLog view
type XLogViewConfig struct {
	ID        int    `json:"id"`
	GroupName string `json:"groupName"`
	ObjType   string `json:"objType"`
}

// ChartConfig holds the configuration for a persisted basic chart
type ChartConfig struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// EQViewConfig holds the configuration for a persisted EQ view
type EQViewConfig struct {
	ID        int    `json:"id"`
	GroupName string `json:"groupName"`
	ObjType   string `json:"objType"`
}

// AppSettings holds application settings
type AppSettings struct {
	Geometry    []byte             `json:"geometry"`
	WindowState []byte             `json:"windowState"`
	ChartCount  int                `json:"chartCount"`
	ChartTitles []string           `json:"chartTitles"`
	Charts      []ChartConfig      `json:"charts,omitempty"`
	GroupCharts []GroupChartConfig  `json:"groupCharts,omitempty"`
	XLogViews   []XLogViewConfig   `json:"xlogViews,omitempty"`
	EQViews     []EQViewConfig     `json:"eqViews,omitempty"`

	// Navigation tree state
	NavCollapsedItems []string `json:"navCollapsedItems,omitempty"`
	NavActiveTab      int      `json:"navActiveTab,omitempty"`
}

// getPath returns the settings file path
func getPath() string {
	homeDir, _ := os.UserHomeDir()
	dir := filepath.Join(homeDir, ".config", configDir)
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, configFile)
}

// Load loads settings from the JSON file
func Load() *AppSettings {
	s := &AppSettings{ChartCount: 1}
	data, err := os.ReadFile(getPath())
	if err != nil {
		return s
	}
	json.Unmarshal(data, s)
	return s
}

// Save saves settings to the JSON file
func (s *AppSettings) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(getPath(), data, 0644)
}

// SetGeometry updates and saves geometry
func (s *AppSettings) SetGeometry(geometry []byte) {
	s.Geometry = geometry
	s.Save()
}

// SetWindowState updates and saves window state
func (s *AppSettings) SetWindowState(state []byte) {
	s.WindowState = state
	s.Save()
}

// SetChartCount updates and saves chart count
func (s *AppSettings) SetChartCount(count int) {
	s.ChartCount = count
	s.Save()
}