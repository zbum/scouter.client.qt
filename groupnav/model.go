package groupnav

import (
	"sort"
	"sync"
)

// HierarchyObject is the base interface for tree items
type HierarchyObject interface {
	GetName() string
	GetParent() HierarchyObject
	SetParent(parent HierarchyObject)
	GetChildren() map[string]HierarchyObject
	GetChild(name string) HierarchyObject
	PutChild(name string, child HierarchyObject)
	RemoveChild(name string)
	GetChildSize() int
	GetSortedChildArray() []HierarchyObject
}

// BaseHierarchyObject provides common implementation
type BaseHierarchyObject struct {
	name     string
	parent   HierarchyObject
	children map[string]HierarchyObject
	mu       sync.RWMutex
}

func NewBaseHierarchyObject(name string) *BaseHierarchyObject {
	return &BaseHierarchyObject{
		name:     name,
		children: make(map[string]HierarchyObject),
	}
}

func (b *BaseHierarchyObject) GetName() string {
	return b.name
}

func (b *BaseHierarchyObject) GetParent() HierarchyObject {
	return b.parent
}

func (b *BaseHierarchyObject) SetParent(parent HierarchyObject) {
	b.parent = parent
}

func (b *BaseHierarchyObject) GetChildren() map[string]HierarchyObject {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.children
}

func (b *BaseHierarchyObject) GetChild(name string) HierarchyObject {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.children[name]
}

func (b *BaseHierarchyObject) PutChild(name string, child HierarchyObject) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.children[name] = child
}

func (b *BaseHierarchyObject) RemoveChild(name string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.children, name)
}

func (b *BaseHierarchyObject) GetChildSize() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.children)
}

func (b *BaseHierarchyObject) GetSortedChildArray() []HierarchyObject {
	b.mu.RLock()
	defer b.mu.RUnlock()

	keys := make([]string, 0, len(b.children))
	for k := range b.children {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]HierarchyObject, 0, len(b.children))
	for _, k := range keys {
		result = append(result, b.children[k])
	}
	return result
}

// GroupObject represents a group of agents
type GroupObject struct {
	*BaseHierarchyObject
	objType string
}

func NewGroupObject(objType, name string) *GroupObject {
	return &GroupObject{
		BaseHierarchyObject: NewBaseHierarchyObject(name),
		objType:             objType,
	}
}

func (g *GroupObject) GetObjType() string {
	return g.objType
}

func (g *GroupObject) GetFirstChild() HierarchyObject {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, child := range g.children {
		return child
	}
	return nil
}

// AgentObject represents a monitoring agent
type AgentObject struct {
	*BaseHierarchyObject
	objHash       int
	objName       string
	objType       string
	serverId      int
	alive         bool
	masterCounter string
	address       string
	version       string
}

func NewAgentObject(objHash int, objName, objType string, serverId int) *AgentObject {
	return &AgentObject{
		BaseHierarchyObject: NewBaseHierarchyObject(objName),
		objHash:             objHash,
		objName:             objName,
		objType:             objType,
		serverId:            serverId,
		alive:               true,
	}
}

// NewAgentObjectFromPack creates an AgentObject from ObjectPack data
func NewAgentObjectFromPack(objHash int32, objName, objType, address, version string, alive bool, serverId int) *AgentObject {
	return &AgentObject{
		BaseHierarchyObject: NewBaseHierarchyObject(objName),
		objHash:             int(objHash),
		objName:             objName,
		objType:             objType,
		serverId:            serverId,
		alive:               alive,
		address:             address,
		version:             version,
	}
}

func (a *AgentObject) GetObjHash() int {
	return a.objHash
}

func (a *AgentObject) GetObjName() string {
	return a.objName
}

func (a *AgentObject) GetObjType() string {
	return a.objType
}

func (a *AgentObject) GetServerId() int {
	return a.serverId
}

func (a *AgentObject) IsAlive() bool {
	return a.alive
}

func (a *AgentObject) SetAlive(alive bool) {
	a.alive = alive
}

func (a *AgentObject) GetMasterCounter() string {
	return a.masterCounter
}

func (a *AgentObject) SetMasterCounter(value string) {
	a.masterCounter = value
}

func (a *AgentObject) GetAddress() string {
	return a.address
}

func (a *AgentObject) GetVersion() string {
	return a.version
}

// DummyObject represents a folder/category (like "Others")
type DummyObject struct {
	*BaseHierarchyObject
}

func NewDummyObject(name string) *DummyObject {
	return &DummyObject{
		BaseHierarchyObject: NewBaseHierarchyObject(name),
	}
}

// OthersGroup constant is defined in manager.go
