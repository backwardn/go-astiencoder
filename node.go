package astiencoder

import (
	"context"
	"sync"

	"sort"

	"github.com/asticode/go-astitools/worker"
)

// Node represents a node
type Node interface {
	NodeChild
	NodeDescriptor
	NodeParent
	Starter
}

// NodeDescriptor represents an object that can describe a node
type NodeDescriptor interface {
	Metadata() NodeMetadata
}

// NodeMetadata represents node metadata
type NodeMetadata struct {
	Description string
	Label       string
	Name        string
}

// NodeChild represents an object with parent nodes
type NodeChild interface {
	AddParent(n Node)
	ParentIsDone(m NodeMetadata)
	Parents() []Node
}

// NodeParent represents an object with child nodes
type NodeParent interface {
	AddChild(n Node)
	ChildIsDone(m NodeMetadata)
	Children() []Node
}

// Starter represents an object that can start/stop
type Starter interface {
	IsStopped() bool
	Start(ctx context.Context, o WorkflowStartOptions, t CreateTaskFunc)
	Stop()
}

// ConnectNodes connects 2 nodes
func ConnectNodes(parent, child Node) {
	parent.AddChild(child)
	child.AddParent(parent)
}

// BaseNode represents a base node
type BaseNode struct {
	cancel       context.CancelFunc
	children     map[string]Node
	childrenDone map[string]bool
	ctx          context.Context
	m            *sync.Mutex
	md           NodeMetadata
	o            WorkflowStartOptions
	oStart       *sync.Once
	oStop        *sync.Once
	parents      map[string]Node
	parentsDone  map[string]bool
}

// NewBaseNode creates a new base node
func NewBaseNode(m NodeMetadata) *BaseNode {
	return &BaseNode{
		children:     make(map[string]Node),
		childrenDone: make(map[string]bool),
		m:            &sync.Mutex{},
		md:           m,
		oStart:       &sync.Once{},
		oStop:        &sync.Once{},
		parents:      make(map[string]Node),
		parentsDone:  make(map[string]bool),
	}
}

// Context returns the node context
func (n *BaseNode) Context() context.Context {
	return n.ctx
}

// CreateTaskFunc is a method that can create a task
type CreateTaskFunc func() *astiworker.Task

// BaseNodeStartFunc represents a node start func
type BaseNodeStartFunc func()

// BaseNodeExecFunc represents a node exec func
type BaseNodeExecFunc func(t *astiworker.Task)

// IsStopped implements the Starter interface
func (n *BaseNode) IsStopped() bool {
	return n.Context() == nil || n.Context().Err() != nil
}

// Start starts the node
func (n *BaseNode) Start(ctx context.Context, o WorkflowStartOptions, tc CreateTaskFunc, execFunc BaseNodeExecFunc) {
	// Make sure the node can only be started once
	n.oStart.Do(func() {
		// Store options
		n.o = o

		// Check context
		if ctx.Err() != nil {
			return
		}

		// Create task
		t := tc()

		// Reset context
		n.ctx, n.cancel = context.WithCancel(ctx)

		// Reset once
		n.oStop = &sync.Once{}

		// Execute the rest in a goroutine
		go func() {
			// Task is done
			defer t.Done()

			// Make sure the node is stopped properly
			defer n.Stop()

			// Exec func
			execFunc(t)

			// Loop through children
			for _, c := range n.Children() {
				c.ParentIsDone(n.md)
			}

			// Loop through parents
			for _, p := range n.Parents() {
				p.ChildIsDone(n.md)
			}
		}()
	})
}

// Stop stops the node
func (n *BaseNode) Stop() {
	// Make sure the node can only be stopped once
	n.oStop.Do(func() {
		// Cancel context
		if n.cancel != nil {
			n.cancel()
		}

		// Reset once
		n.oStart = &sync.Once{}
	})
}

// AddChild implements the NodeParent interface
func (n *BaseNode) AddChild(i Node) {
	n.m.Lock()
	defer n.m.Unlock()
	if _, ok := n.children[i.Metadata().Name]; ok {
		return
	}
	n.children[i.Metadata().Name] = i
}

// ChildIsDone implements the NodeParent interface
func (n *BaseNode) ChildIsDone(m NodeMetadata) {
	n.m.Lock()
	defer n.m.Unlock()
	if _, ok := n.children[m.Name]; !ok {
		return
	}
	n.childrenDone[m.Name] = true
	if n.o.StopWhenNodesAreDone && len(n.childrenDone) == len(n.children) {
		n.Stop()
	}
}

// Children implements the NodeParent interface
func (n *BaseNode) Children() (ns []Node) {
	n.m.Lock()
	defer n.m.Unlock()
	var ks []string
	for k := range n.children {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		ns = append(ns, n.children[k])
	}
	return
}

// AddParent implements the NodeChild interface
func (n *BaseNode) AddParent(i Node) {
	n.m.Lock()
	defer n.m.Unlock()
	if _, ok := n.parents[i.Metadata().Name]; ok {
		return
	}
	n.parents[i.Metadata().Name] = i
}

// ParentIsDone implements the NodeChild interface
func (n *BaseNode) ParentIsDone(m NodeMetadata) {
	n.m.Lock()
	defer n.m.Unlock()
	if _, ok := n.parents[m.Name]; !ok {
		return
	}
	n.parentsDone[m.Name] = true
	if n.o.StopWhenNodesAreDone && len(n.parentsDone) == len(n.parents) {
		n.Stop()
	}
}

// Parents implements the NodeChild interface
func (n *BaseNode) Parents() (ns []Node) {
	n.m.Lock()
	defer n.m.Unlock()
	var ks []string
	for k := range n.parents {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		ns = append(ns, n.parents[k])
	}
	return
}

// Metadata implements the Node interface
func (n *BaseNode) Metadata() NodeMetadata {
	return n.md
}
