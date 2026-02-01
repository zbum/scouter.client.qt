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

// PerspectiveState holds the state of a single perspective
type PerspectiveState struct {
	Name        string             `json:"name,omitempty"`
	WindowState []byte             `json:"windowState,omitempty"`
	Charts      []ChartConfig      `json:"charts,omitempty"`
	GroupCharts []GroupChartConfig  `json:"groupCharts,omitempty"`
	XLogViews   []XLogViewConfig   `json:"xlogViews,omitempty"`
	EQViews     []EQViewConfig     `json:"eqViews,omitempty"`
}

// AppSettings holds application settings
type AppSettings struct {
	Geometry    []byte             `json:"geometry"`
	WindowState []byte             `json:"windowState,omitempty"`
	ChartCount  int                `json:"chartCount,omitempty"`
	ChartTitles []string           `json:"chartTitles,omitempty"`
	Charts      []ChartConfig      `json:"charts,omitempty"`
	GroupCharts []GroupChartConfig  `json:"groupCharts,omitempty"`
	XLogViews   []XLogViewConfig   `json:"xlogViews,omitempty"`
	EQViews     []EQViewConfig     `json:"eqViews,omitempty"`

	// Navigation tree state
	NavCollapsedItems []string `json:"navCollapsedItems,omitempty"`
	NavActiveTab      int      `json:"navActiveTab,omitempty"`

	// Perspective state
	ActivePerspective string                       `json:"activePerspective,omitempty"`
	PerspectiveOrder  []string                     `json:"perspectiveOrder,omitempty"`
	Perspectives      map[string]*PerspectiveState `json:"perspectives,omitempty"`
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
	s.migrate()
	return s
}

// migrate moves legacy flat fields into the perspective state structure.
// If Perspectives["service"] already exists, this is a no-op.
func (s *AppSettings) migrate() {
	if s.Perspectives == nil {
		s.Perspectives = make(map[string]*PerspectiveState)
	}

	// Already migrated
	if _, ok := s.Perspectives["service"]; ok {
		return
	}

	// Check if there's anything to migrate
	hasLegacy := len(s.Charts) > 0 || len(s.GroupCharts) > 0 ||
		len(s.XLogViews) > 0 || len(s.EQViews) > 0 ||
		len(s.WindowState) > 0 || s.ChartCount > 1 || len(s.ChartTitles) > 0

	if !hasLegacy {
		return
	}

	// Migrate flat fields into service perspective
	ps := &PerspectiveState{
		WindowState: s.WindowState,
		Charts:      s.Charts,
		GroupCharts: s.GroupCharts,
		XLogViews:   s.XLogViews,
		EQViews:     s.EQViews,
	}

	// Backward compat: convert ChartCount+ChartTitles to Charts if Charts is empty
	if len(ps.Charts) == 0 && s.ChartCount > 0 {
		for i := 0; i < s.ChartCount; i++ {
			var title string
			if i < len(s.ChartTitles) {
				title = s.ChartTitles[i]
			}
			ps.Charts = append(ps.Charts, ChartConfig{ID: i + 1, Title: title})
		}
	}

	ps.Name = "Service"
	s.Perspectives["service"] = ps
	s.PerspectiveOrder = []string{"service"}
	if s.ActivePerspective == "" {
		s.ActivePerspective = "service"
	}

	// Clear legacy fields
	s.WindowState = nil
	s.Charts = nil
	s.GroupCharts = nil
	s.XLogViews = nil
	s.EQViews = nil
	s.ChartCount = 0
	s.ChartTitles = nil
}

// GetPerspectiveState returns the state for a perspective, creating if needed
func (s *AppSettings) GetPerspectiveState(id string) *PerspectiveState {
	if s.Perspectives == nil {
		s.Perspectives = make(map[string]*PerspectiveState)
	}
	ps, ok := s.Perspectives[id]
	if !ok {
		ps = &PerspectiveState{}
		s.Perspectives[id] = ps
	}
	return ps
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