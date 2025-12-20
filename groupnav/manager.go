package groupnav

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const (
	keyObjType   = "_objType_"
	OthersGroup  = "Others"
	groupFile    = "groups.json"
)

// GroupData represents the persisted group data
type GroupData struct {
	// GroupTypes maps group name to objType
	GroupTypes map[string]string `json:"groupTypes"`
	// ObjGroups maps objHash (as string) to list of group names
	ObjGroups map[string][]string `json:"objGroups"`
}

// Manager manages object groups (singleton)
type Manager struct {
	mu         sync.RWMutex
	filePath   string
	reserved   map[string]bool
	groupTypes map[string]string   // groupName -> objType
	objGroups  map[string][]string // objHash -> []groupName

	// Cache for getObjectsByGroup
	cacheMu    sync.RWMutex
	groupCache map[string]map[int]bool
}

var (
	managerInstance *Manager
	managerOnce     sync.Once
)

// GetManager returns the singleton Manager instance
func GetManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = newManager()
	})
	return managerInstance
}

func newManager() *Manager {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".config", "scouter.client.go")
	os.MkdirAll(configDir, 0755)

	m := &Manager{
		filePath:   filepath.Join(configDir, groupFile),
		reserved:   make(map[string]bool),
		groupTypes: make(map[string]string),
		objGroups:  make(map[string][]string),
		groupCache: make(map[string]map[int]bool),
	}

	m.reserved[keyObjType] = true
	m.reserved[OthersGroup] = true

	m.load()
	return m
}

// load reads group data from file
func (m *Manager) load() {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return
	}

	var groupData GroupData
	if err := json.Unmarshal(data, &groupData); err != nil {
		return
	}

	if groupData.GroupTypes != nil {
		m.groupTypes = groupData.GroupTypes
	}
	if groupData.ObjGroups != nil {
		m.objGroups = groupData.ObjGroups
	}
}

// save writes group data to file (async)
func (m *Manager) save() {
	go func() {
		m.mu.RLock()
		groupData := GroupData{
			GroupTypes: m.groupTypes,
			ObjGroups:  m.objGroups,
		}
		m.mu.RUnlock()

		data, err := json.MarshalIndent(groupData, "", "  ")
		if err != nil {
			return
		}
		os.WriteFile(m.filePath, data, 0644)
	}()
}

// clearCache clears the group object cache
func (m *Manager) clearCache() {
	m.cacheMu.Lock()
	m.groupCache = make(map[string]map[int]bool)
	m.cacheMu.Unlock()
}

// AddGroup adds a new group with the specified objType
func (m *Manager) AddGroup(objType, groupName string) bool {
	groupName = trimString(groupName)
	if groupName == "" || m.reserved[groupName] {
		return false
	}

	m.mu.Lock()
	m.groupTypes[groupName] = objType
	m.mu.Unlock()

	m.clearCache()
	m.save()
	return true
}

// RemoveGroup removes a group by name
func (m *Manager) RemoveGroup(groupName string) {
	m.mu.Lock()
	delete(m.groupTypes, groupName)
	m.mu.Unlock()

	m.clearCache()
	m.save()
}

// ListGroups returns all group names
func (m *Manager) ListGroups() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]string, 0, len(m.groupTypes))
	for name := range m.groupTypes {
		groups = append(groups, name)
	}
	return groups
}

// GetGroupObjType returns the objType for a group
func (m *Manager) GetGroupObjType(groupName string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.groupTypes[groupName]
}

// GetObjTypeList returns all unique objTypes
func (m *Manager) GetObjTypeList() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	typeSet := make(map[string]bool)
	for _, objType := range m.groupTypes {
		typeSet[objType] = true
	}

	types := make([]string, 0, len(typeSet))
	for t := range typeSet {
		types = append(types, t)
	}
	return types
}

// AddObject adds an object to a group
func (m *Manager) AddObject(objHash int, groupName string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.groupTypes[groupName] == "" {
		return false
	}

	objKey := intToString(objHash)
	groups := m.objGroups[objKey]

	// Check if already in group
	for _, g := range groups {
		if g == groupName {
			return true
		}
	}

	m.objGroups[objKey] = append(groups, groupName)
	m.clearCache()
	m.save()
	return true
}

// AddObjects adds multiple objects to a group
func (m *Manager) AddObjects(objHashs []int, groupName string) bool {
	m.mu.Lock()

	if m.groupTypes[groupName] == "" {
		m.mu.Unlock()
		return false
	}

	for _, objHash := range objHashs {
		objKey := intToString(objHash)
		groups := m.objGroups[objKey]

		// Check if already in group
		found := false
		for _, g := range groups {
			if g == groupName {
				found = true
				break
			}
		}
		if !found {
			m.objGroups[objKey] = append(groups, groupName)
		}
	}
	m.mu.Unlock()

	m.clearCache()
	m.save()
	return true
}

// RemoveObject removes an object from a group
func (m *Manager) RemoveObject(objHash int, groupName string) {
	m.mu.Lock()

	objKey := intToString(objHash)
	groups := m.objGroups[objKey]

	newGroups := make([]string, 0, len(groups))
	for _, g := range groups {
		if g != groupName {
			newGroups = append(newGroups, g)
		}
	}

	if len(newGroups) > 0 {
		m.objGroups[objKey] = newGroups
	} else {
		delete(m.objGroups, objKey)
	}
	m.mu.Unlock()

	m.clearCache()
	m.save()
}

// RemoveObjects removes multiple objects from a group
func (m *Manager) RemoveObjects(objHashs []int, groupName string) {
	m.mu.Lock()

	for _, objHash := range objHashs {
		objKey := intToString(objHash)
		groups := m.objGroups[objKey]

		newGroups := make([]string, 0, len(groups))
		for _, g := range groups {
			if g != groupName {
				newGroups = append(newGroups, g)
			}
		}

		if len(newGroups) > 0 {
			m.objGroups[objKey] = newGroups
		} else {
			delete(m.objGroups, objKey)
		}
	}
	m.mu.Unlock()

	m.clearCache()
	m.save()
}

// GetObjGroups returns all groups an object belongs to
func (m *Manager) GetObjGroups(objHash int) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	objKey := intToString(objHash)
	groups := m.objGroups[objKey]

	// Filter to only existing groups
	validGroups := make([]string, 0, len(groups))
	for _, g := range groups {
		if m.groupTypes[g] != "" {
			validGroups = append(validGroups, g)
		}
	}

	return validGroups
}

// AssignGroups sets the groups for an object (replaces existing)
func (m *Manager) AssignGroups(objHash int, groups []string) {
	m.mu.Lock()

	objKey := intToString(objHash)
	if len(groups) > 0 {
		m.objGroups[objKey] = groups
	} else {
		delete(m.objGroups, objKey)
	}
	m.mu.Unlock()

	m.clearCache()
	m.save()
}

// GetGroupsByType returns all groups of a specific objType
func (m *Manager) GetGroupsByType(objType string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]string, 0)
	for name, t := range m.groupTypes {
		if t == objType {
			groups = append(groups, name)
		}
	}
	return groups
}

// GetObjectsByGroup returns all objHashes in a group
func (m *Manager) GetObjectsByGroup(groupName string) map[int]bool {
	// Check cache first
	m.cacheMu.RLock()
	if cached, ok := m.groupCache[groupName]; ok {
		m.cacheMu.RUnlock()
		return cached
	}
	m.cacheMu.RUnlock()

	// Build the set
	m.mu.RLock()
	objSet := make(map[int]bool)
	for objKey, groups := range m.objGroups {
		for _, g := range groups {
			if g == groupName {
				if hash, ok := stringToInt(objKey); ok {
					objSet[hash] = true
				}
				break
			}
		}
	}
	m.mu.RUnlock()

	// Cache the result
	m.cacheMu.Lock()
	m.groupCache[groupName] = objSet
	m.cacheMu.Unlock()

	return objSet
}

// GroupExists checks if a group exists
func (m *Manager) GroupExists(groupName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.groupTypes[groupName]
	return exists
}

// Helper functions
func trimString(s string) string {
	// Simple trim implementation
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}

	neg := false
	if n < 0 {
		neg = true
		n = -n
	}

	var digits [20]byte
	i := len(digits)

	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}

	if neg {
		i--
		digits[i] = '-'
	}

	return string(digits[i:])
}

func stringToInt(s string) (int, bool) {
	if len(s) == 0 {
		return 0, false
	}

	neg := false
	i := 0

	if s[0] == '-' {
		neg = true
		i = 1
	}

	n := 0
	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		n = n*10 + int(s[i]-'0')
	}

	if neg {
		n = -n
	}
	return n, true
}
