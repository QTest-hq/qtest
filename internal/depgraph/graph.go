// Package depgraph provides code dependency graph construction and analysis
package depgraph

import (
	"path/filepath"
	"strings"

	"github.com/QTest-hq/qtest/internal/parser"
)

// NodeType represents the type of a dependency graph node
type NodeType string

const (
	NodeTypeFile     NodeType = "file"
	NodeTypeFunction NodeType = "function"
	NodeTypeClass    NodeType = "class"
	NodeTypeModule   NodeType = "module"
)

// EdgeType represents the type of relationship between nodes
type EdgeType string

const (
	EdgeTypeImports   EdgeType = "imports"   // File imports another file/module
	EdgeTypeCalls     EdgeType = "calls"     // Function calls another function
	EdgeTypeExtends   EdgeType = "extends"   // Class extends another class
	EdgeTypeUses      EdgeType = "uses"      // General usage dependency
	EdgeTypeContains  EdgeType = "contains"  // File contains function/class
)

// Node represents a node in the dependency graph
type Node struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Type     NodeType          `json:"type"`
	File     string            `json:"file,omitempty"`
	Line     int               `json:"line,omitempty"`
	Exported bool              `json:"exported,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Edge represents a directed edge in the dependency graph
type Edge struct {
	From     string            `json:"from"`      // Source node ID
	To       string            `json:"to"`        // Target node ID
	Type     EdgeType          `json:"type"`
	Weight   int               `json:"weight,omitempty"` // Number of references
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Graph represents a dependency graph
type Graph struct {
	Nodes map[string]*Node `json:"nodes"`
	Edges []*Edge          `json:"edges"`

	// Indexes for fast lookup
	outEdges map[string][]*Edge // Edges from a node
	inEdges  map[string][]*Edge // Edges to a node
}

// NewGraph creates a new empty dependency graph
func NewGraph() *Graph {
	return &Graph{
		Nodes:    make(map[string]*Node),
		Edges:    make([]*Edge, 0),
		outEdges: make(map[string][]*Edge),
		inEdges:  make(map[string][]*Edge),
	}
}

// AddNode adds a node to the graph
func (g *Graph) AddNode(node *Node) {
	if node.Metadata == nil {
		node.Metadata = make(map[string]string)
	}
	g.Nodes[node.ID] = node
}

// AddEdge adds an edge to the graph
func (g *Graph) AddEdge(edge *Edge) {
	if edge.Metadata == nil {
		edge.Metadata = make(map[string]string)
	}
	g.Edges = append(g.Edges, edge)
	g.outEdges[edge.From] = append(g.outEdges[edge.From], edge)
	g.inEdges[edge.To] = append(g.inEdges[edge.To], edge)
}

// GetNode returns a node by ID
func (g *Graph) GetNode(id string) *Node {
	return g.Nodes[id]
}

// GetOutEdges returns all edges originating from a node
func (g *Graph) GetOutEdges(nodeID string) []*Edge {
	return g.outEdges[nodeID]
}

// GetInEdges returns all edges pointing to a node
func (g *Graph) GetInEdges(nodeID string) []*Edge {
	return g.inEdges[nodeID]
}

// GetDependencies returns all nodes that a given node depends on
func (g *Graph) GetDependencies(nodeID string) []*Node {
	edges := g.outEdges[nodeID]
	nodes := make([]*Node, 0, len(edges))
	for _, edge := range edges {
		if node := g.Nodes[edge.To]; node != nil {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// GetDependents returns all nodes that depend on a given node
func (g *Graph) GetDependents(nodeID string) []*Node {
	edges := g.inEdges[nodeID]
	nodes := make([]*Node, 0, len(edges))
	for _, edge := range edges {
		if node := g.Nodes[edge.From]; node != nil {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// GetTransitiveDependencies returns all transitive dependencies of a node
func (g *Graph) GetTransitiveDependencies(nodeID string, maxDepth int) []*Node {
	visited := make(map[string]bool)
	result := make([]*Node, 0)

	g.collectTransitive(nodeID, visited, &result, 0, maxDepth, true)
	return result
}

// GetTransitiveDependents returns all transitive dependents of a node
func (g *Graph) GetTransitiveDependents(nodeID string, maxDepth int) []*Node {
	visited := make(map[string]bool)
	result := make([]*Node, 0)

	g.collectTransitive(nodeID, visited, &result, 0, maxDepth, false)
	return result
}

func (g *Graph) collectTransitive(nodeID string, visited map[string]bool, result *[]*Node, depth, maxDepth int, forward bool) {
	if visited[nodeID] || (maxDepth > 0 && depth >= maxDepth) {
		return
	}
	visited[nodeID] = true

	var edges []*Edge
	if forward {
		edges = g.outEdges[nodeID]
	} else {
		edges = g.inEdges[nodeID]
	}

	for _, edge := range edges {
		var targetID string
		if forward {
			targetID = edge.To
		} else {
			targetID = edge.From
		}

		if node := g.Nodes[targetID]; node != nil && !visited[targetID] {
			*result = append(*result, node)
			g.collectTransitive(targetID, visited, result, depth+1, maxDepth, forward)
		}
	}
}

// FindCycles detects cycles in the dependency graph
func (g *Graph) FindCycles() [][]string {
	var cycles [][]string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := make([]string, 0)

	for nodeID := range g.Nodes {
		if !visited[nodeID] {
			g.detectCycle(nodeID, visited, recStack, path, &cycles)
		}
	}

	return cycles
}

func (g *Graph) detectCycle(nodeID string, visited, recStack map[string]bool, path []string, cycles *[][]string) {
	visited[nodeID] = true
	recStack[nodeID] = true
	path = append(path, nodeID)

	for _, edge := range g.outEdges[nodeID] {
		if !visited[edge.To] {
			g.detectCycle(edge.To, visited, recStack, path, cycles)
		} else if recStack[edge.To] {
			// Found a cycle - extract it
			cycleStart := -1
			for i, id := range path {
				if id == edge.To {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cycle := make([]string, len(path)-cycleStart+1)
				copy(cycle, path[cycleStart:])
				cycle[len(cycle)-1] = edge.To
				*cycles = append(*cycles, cycle)
			}
		}
	}

	recStack[nodeID] = false
}

// TopologicalSort returns nodes in topological order
func (g *Graph) TopologicalSort() ([]*Node, error) {
	inDegree := make(map[string]int)
	for id := range g.Nodes {
		inDegree[id] = 0
	}
	for _, edge := range g.Edges {
		inDegree[edge.To]++
	}

	// Start with nodes that have no incoming edges
	queue := make([]string, 0)
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	result := make([]*Node, 0, len(g.Nodes))
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]

		if node := g.Nodes[nodeID]; node != nil {
			result = append(result, node)
		}

		for _, edge := range g.outEdges[nodeID] {
			inDegree[edge.To]--
			if inDegree[edge.To] == 0 {
				queue = append(queue, edge.To)
			}
		}
	}

	return result, nil
}

// Stats returns statistics about the graph
type Stats struct {
	TotalNodes     int            `json:"total_nodes"`
	TotalEdges     int            `json:"total_edges"`
	NodesByType    map[NodeType]int `json:"nodes_by_type"`
	EdgesByType    map[EdgeType]int `json:"edges_by_type"`
	AvgInDegree    float64        `json:"avg_in_degree"`
	AvgOutDegree   float64        `json:"avg_out_degree"`
	MaxInDegree    int            `json:"max_in_degree"`
	MaxOutDegree   int            `json:"max_out_degree"`
	CycleCount     int            `json:"cycle_count"`
}

// GetStats returns statistics about the graph
func (g *Graph) GetStats() *Stats {
	stats := &Stats{
		TotalNodes:  len(g.Nodes),
		TotalEdges:  len(g.Edges),
		NodesByType: make(map[NodeType]int),
		EdgesByType: make(map[EdgeType]int),
	}

	for _, node := range g.Nodes {
		stats.NodesByType[node.Type]++
	}

	for _, edge := range g.Edges {
		stats.EdgesByType[edge.Type]++
	}

	totalIn, totalOut := 0, 0
	for nodeID := range g.Nodes {
		inDeg := len(g.inEdges[nodeID])
		outDeg := len(g.outEdges[nodeID])
		totalIn += inDeg
		totalOut += outDeg

		if inDeg > stats.MaxInDegree {
			stats.MaxInDegree = inDeg
		}
		if outDeg > stats.MaxOutDegree {
			stats.MaxOutDegree = outDeg
		}
	}

	if len(g.Nodes) > 0 {
		stats.AvgInDegree = float64(totalIn) / float64(len(g.Nodes))
		stats.AvgOutDegree = float64(totalOut) / float64(len(g.Nodes))
	}

	stats.CycleCount = len(g.FindCycles())

	return stats
}

// Builder builds a dependency graph from parsed files
type Builder struct {
	graph        *Graph
	fileIndex    map[string]string // filename -> node ID
	moduleIndex  map[string]string // module name -> file path
}

// NewBuilder creates a new graph builder
func NewBuilder() *Builder {
	return &Builder{
		graph:       NewGraph(),
		fileIndex:   make(map[string]string),
		moduleIndex: make(map[string]string),
	}
}

// Build constructs a dependency graph from parsed files
func (b *Builder) Build(files []parser.ParsedFile) *Graph {
	// First pass: create nodes for all files, functions, and classes
	for _, file := range files {
		b.addFileNode(file)
	}

	// Second pass: create edges based on imports and references
	for _, file := range files {
		b.addFileEdges(file)
	}

	return b.graph
}

func (b *Builder) addFileNode(file parser.ParsedFile) {
	fileID := "file:" + file.Path
	b.fileIndex[file.Path] = fileID

	// Determine module name from path
	moduleName := b.pathToModule(file.Path, file.Language)
	b.moduleIndex[moduleName] = file.Path

	b.graph.AddNode(&Node{
		ID:   fileID,
		Name: filepath.Base(file.Path),
		Type: NodeTypeFile,
		File: file.Path,
		Metadata: map[string]string{
			"language": string(file.Language),
			"module":   moduleName,
		},
	})

	// Add function nodes
	for _, fn := range file.Functions {
		fnID := "func:" + file.Path + ":" + fn.Name
		b.graph.AddNode(&Node{
			ID:       fnID,
			Name:     fn.Name,
			Type:     NodeTypeFunction,
			File:     file.Path,
			Line:     fn.StartLine,
			Exported: fn.Exported,
			Metadata: map[string]string{
				"class":    fn.Class,
				"async":    boolToStr(fn.Async),
			},
		})

		// File contains function
		b.graph.AddEdge(&Edge{
			From: fileID,
			To:   fnID,
			Type: EdgeTypeContains,
		})
	}

	// Add class nodes
	for _, cls := range file.Classes {
		clsID := "class:" + file.Path + ":" + cls.Name
		b.graph.AddNode(&Node{
			ID:       clsID,
			Name:     cls.Name,
			Type:     NodeTypeClass,
			File:     file.Path,
			Line:     cls.StartLine,
			Exported: cls.Exported,
			Metadata: map[string]string{
				"extends": cls.Extends,
			},
		})

		// File contains class
		b.graph.AddEdge(&Edge{
			From: fileID,
			To:   clsID,
			Type: EdgeTypeContains,
		})

		// Add method nodes under class
		for _, method := range cls.Methods {
			methodID := "func:" + file.Path + ":" + cls.Name + "." + method.Name
			b.graph.AddNode(&Node{
				ID:       methodID,
				Name:     cls.Name + "." + method.Name,
				Type:     NodeTypeFunction,
				File:     file.Path,
				Line:     method.StartLine,
				Exported: method.Exported,
				Metadata: map[string]string{
					"class": cls.Name,
				},
			})

			// Class contains method
			b.graph.AddEdge(&Edge{
				From: clsID,
				To:   methodID,
				Type: EdgeTypeContains,
			})
		}
	}
}

func (b *Builder) addFileEdges(file parser.ParsedFile) {
	fileID := b.fileIndex[file.Path]

	// Process imports
	for _, imp := range file.Imports {
		targetPath := b.resolveImport(imp.Module, file.Path, file.Language)
		if targetPath == "" {
			// External module - create a module node if not exists
			moduleID := "module:" + imp.Module
			if b.graph.GetNode(moduleID) == nil {
				b.graph.AddNode(&Node{
					ID:   moduleID,
					Name: imp.Module,
					Type: NodeTypeModule,
					Metadata: map[string]string{
						"external": "true",
					},
				})
			}
			b.graph.AddEdge(&Edge{
				From: fileID,
				To:   moduleID,
				Type: EdgeTypeImports,
				Metadata: map[string]string{
					"names": strings.Join(imp.Names, ","),
					"alias": imp.Alias,
				},
			})
		} else {
			// Internal import
			if targetID, ok := b.fileIndex[targetPath]; ok {
				b.graph.AddEdge(&Edge{
					From: fileID,
					To:   targetID,
					Type: EdgeTypeImports,
					Metadata: map[string]string{
						"names": strings.Join(imp.Names, ","),
						"alias": imp.Alias,
					},
				})
			}
		}
	}

	// Process class inheritance
	for _, cls := range file.Classes {
		if cls.Extends != "" {
			clsID := "class:" + file.Path + ":" + cls.Name
			parentID := b.findClassNode(cls.Extends, file.Path)
			if parentID != "" {
				b.graph.AddEdge(&Edge{
					From: clsID,
					To:   parentID,
					Type: EdgeTypeExtends,
				})
			}
		}
	}
}

func (b *Builder) pathToModule(path string, lang parser.Language) string {
	// Convert file path to module name based on language conventions
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	switch lang {
	case parser.LanguageGo:
		// Go uses directory as package
		return filepath.Dir(path)
	case parser.LanguagePython:
		// Python uses file name as module
		return strings.ReplaceAll(strings.TrimSuffix(path, ".py"), "/", ".")
	case parser.LanguageJavaScript, parser.LanguageTypeScript:
		// JS/TS uses relative path
		return "./" + strings.TrimSuffix(path, ext)
	default:
		return name
	}
}

func (b *Builder) resolveImport(module, fromPath string, lang parser.Language) string {
	// Try to resolve import to a file path in the project

	// Check if it's in our module index
	if path, ok := b.moduleIndex[module]; ok {
		return path
	}

	// Try relative path resolution
	if strings.HasPrefix(module, "./") || strings.HasPrefix(module, "../") {
		dir := filepath.Dir(fromPath)
		resolved := filepath.Clean(filepath.Join(dir, module))

		// Try with common extensions
		extensions := []string{"", ".ts", ".tsx", ".js", ".jsx", ".go", ".py"}
		for _, ext := range extensions {
			candidate := resolved + ext
			if _, ok := b.fileIndex[candidate]; ok {
				return candidate
			}
		}

		// Try index files
		indexFiles := []string{"index.ts", "index.tsx", "index.js", "index.jsx"}
		for _, idx := range indexFiles {
			candidate := filepath.Join(resolved, idx)
			if _, ok := b.fileIndex[candidate]; ok {
				return candidate
			}
		}
	}

	return "" // External module
}

func (b *Builder) findClassNode(className, fromFile string) string {
	// Try to find a class node by name
	// First check in the same file
	clsID := "class:" + fromFile + ":" + className
	if b.graph.GetNode(clsID) != nil {
		return clsID
	}

	// Check in imported files
	fileID := b.fileIndex[fromFile]
	for _, edge := range b.graph.GetOutEdges(fileID) {
		if edge.Type == EdgeTypeImports {
			targetNode := b.graph.GetNode(edge.To)
			if targetNode != nil && targetNode.Type == NodeTypeFile {
				clsID = "class:" + targetNode.File + ":" + className
				if b.graph.GetNode(clsID) != nil {
					return clsID
				}
			}
		}
	}

	return ""
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
