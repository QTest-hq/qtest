package depgraph

import (
	"testing"

	"github.com/QTest-hq/qtest/internal/parser"
	"github.com/stretchr/testify/assert"
)

func TestNewGraph(t *testing.T) {
	g := NewGraph()
	assert.NotNil(t, g)
	assert.Empty(t, g.Nodes)
	assert.Empty(t, g.Edges)
}

func TestGraph_AddNode(t *testing.T) {
	g := NewGraph()
	node := &Node{
		ID:   "test-node",
		Name: "TestNode",
		Type: NodeTypeFunction,
	}

	g.AddNode(node)

	assert.Len(t, g.Nodes, 1)
	assert.Equal(t, node, g.Nodes["test-node"])
	assert.NotNil(t, node.Metadata)
}

func TestGraph_AddEdge(t *testing.T) {
	g := NewGraph()
	g.AddNode(&Node{ID: "a", Name: "A", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "b", Name: "B", Type: NodeTypeFile})

	edge := &Edge{
		From: "a",
		To:   "b",
		Type: EdgeTypeImports,
	}
	g.AddEdge(edge)

	assert.Len(t, g.Edges, 1)
	assert.Equal(t, "a", g.Edges[0].From)
	assert.Equal(t, "b", g.Edges[0].To)
}

func TestGraph_GetNode(t *testing.T) {
	g := NewGraph()
	node := &Node{ID: "test", Name: "Test", Type: NodeTypeFile}
	g.AddNode(node)

	found := g.GetNode("test")
	assert.Equal(t, node, found)

	notFound := g.GetNode("nonexistent")
	assert.Nil(t, notFound)
}

func TestGraph_GetOutEdges(t *testing.T) {
	g := NewGraph()
	g.AddNode(&Node{ID: "a", Name: "A", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "b", Name: "B", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "c", Name: "C", Type: NodeTypeFile})

	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeImports})
	g.AddEdge(&Edge{From: "a", To: "c", Type: EdgeTypeImports})

	edges := g.GetOutEdges("a")
	assert.Len(t, edges, 2)

	edges = g.GetOutEdges("b")
	assert.Empty(t, edges)
}

func TestGraph_GetInEdges(t *testing.T) {
	g := NewGraph()
	g.AddNode(&Node{ID: "a", Name: "A", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "b", Name: "B", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "c", Name: "C", Type: NodeTypeFile})

	g.AddEdge(&Edge{From: "a", To: "c", Type: EdgeTypeImports})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeTypeImports})

	edges := g.GetInEdges("c")
	assert.Len(t, edges, 2)

	edges = g.GetInEdges("a")
	assert.Empty(t, edges)
}

func TestGraph_GetDependencies(t *testing.T) {
	g := NewGraph()
	g.AddNode(&Node{ID: "a", Name: "A", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "b", Name: "B", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "c", Name: "C", Type: NodeTypeFile})

	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeImports})
	g.AddEdge(&Edge{From: "a", To: "c", Type: EdgeTypeImports})

	deps := g.GetDependencies("a")
	assert.Len(t, deps, 2)

	deps = g.GetDependencies("b")
	assert.Empty(t, deps)
}

func TestGraph_GetDependents(t *testing.T) {
	g := NewGraph()
	g.AddNode(&Node{ID: "a", Name: "A", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "b", Name: "B", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "c", Name: "C", Type: NodeTypeFile})

	g.AddEdge(&Edge{From: "a", To: "c", Type: EdgeTypeImports})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeTypeImports})

	dependents := g.GetDependents("c")
	assert.Len(t, dependents, 2)

	dependents = g.GetDependents("a")
	assert.Empty(t, dependents)
}

func TestGraph_GetTransitiveDependencies(t *testing.T) {
	g := NewGraph()
	// a -> b -> c -> d
	g.AddNode(&Node{ID: "a", Name: "A", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "b", Name: "B", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "c", Name: "C", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "d", Name: "D", Type: NodeTypeFile})

	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeImports})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeTypeImports})
	g.AddEdge(&Edge{From: "c", To: "d", Type: EdgeTypeImports})

	deps := g.GetTransitiveDependencies("a", 0) // unlimited depth
	assert.Len(t, deps, 3)

	deps = g.GetTransitiveDependencies("a", 1) // max depth 1
	assert.Len(t, deps, 1)

	deps = g.GetTransitiveDependencies("a", 2) // max depth 2
	assert.Len(t, deps, 2)
}

func TestGraph_GetTransitiveDependents(t *testing.T) {
	g := NewGraph()
	// a -> b -> c -> d
	g.AddNode(&Node{ID: "a", Name: "A", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "b", Name: "B", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "c", Name: "C", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "d", Name: "D", Type: NodeTypeFile})

	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeImports})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeTypeImports})
	g.AddEdge(&Edge{From: "c", To: "d", Type: EdgeTypeImports})

	dependents := g.GetTransitiveDependents("d", 0) // unlimited depth
	assert.Len(t, dependents, 3)

	dependents = g.GetTransitiveDependents("d", 1) // max depth 1
	assert.Len(t, dependents, 1) // only c
}

func TestGraph_FindCycles(t *testing.T) {
	t.Run("no cycles", func(t *testing.T) {
		g := NewGraph()
		g.AddNode(&Node{ID: "a", Name: "A", Type: NodeTypeFile})
		g.AddNode(&Node{ID: "b", Name: "B", Type: NodeTypeFile})
		g.AddNode(&Node{ID: "c", Name: "C", Type: NodeTypeFile})

		g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeImports})
		g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeTypeImports})

		cycles := g.FindCycles()
		assert.Empty(t, cycles)
	})

	t.Run("simple cycle", func(t *testing.T) {
		g := NewGraph()
		g.AddNode(&Node{ID: "a", Name: "A", Type: NodeTypeFile})
		g.AddNode(&Node{ID: "b", Name: "B", Type: NodeTypeFile})

		g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeImports})
		g.AddEdge(&Edge{From: "b", To: "a", Type: EdgeTypeImports})

		cycles := g.FindCycles()
		assert.NotEmpty(t, cycles)
	})
}

func TestGraph_TopologicalSort(t *testing.T) {
	g := NewGraph()
	g.AddNode(&Node{ID: "a", Name: "A", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "b", Name: "B", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "c", Name: "C", Type: NodeTypeFile})

	g.AddEdge(&Edge{From: "a", To: "b", Type: EdgeTypeImports})
	g.AddEdge(&Edge{From: "a", To: "c", Type: EdgeTypeImports})
	g.AddEdge(&Edge{From: "b", To: "c", Type: EdgeTypeImports})

	sorted, err := g.TopologicalSort()
	assert.NoError(t, err)
	assert.Len(t, sorted, 3)

	// a should come before b and c
	aIndex, bIndex, cIndex := -1, -1, -1
	for i, n := range sorted {
		switch n.ID {
		case "a":
			aIndex = i
		case "b":
			bIndex = i
		case "c":
			cIndex = i
		}
	}
	assert.True(t, aIndex < bIndex)
	assert.True(t, aIndex < cIndex)
	assert.True(t, bIndex < cIndex)
}

func TestGraph_GetStats(t *testing.T) {
	g := NewGraph()
	g.AddNode(&Node{ID: "f1", Name: "file1", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "f2", Name: "file2", Type: NodeTypeFile})
	g.AddNode(&Node{ID: "fn1", Name: "func1", Type: NodeTypeFunction})
	g.AddNode(&Node{ID: "cls1", Name: "class1", Type: NodeTypeClass})

	g.AddEdge(&Edge{From: "f1", To: "f2", Type: EdgeTypeImports})
	g.AddEdge(&Edge{From: "f1", To: "fn1", Type: EdgeTypeContains})
	g.AddEdge(&Edge{From: "f2", To: "cls1", Type: EdgeTypeContains})

	stats := g.GetStats()

	assert.Equal(t, 4, stats.TotalNodes)
	assert.Equal(t, 3, stats.TotalEdges)
	assert.Equal(t, 2, stats.NodesByType[NodeTypeFile])
	assert.Equal(t, 1, stats.NodesByType[NodeTypeFunction])
	assert.Equal(t, 1, stats.NodesByType[NodeTypeClass])
	assert.Equal(t, 1, stats.EdgesByType[EdgeTypeImports])
	assert.Equal(t, 2, stats.EdgesByType[EdgeTypeContains])
}

func TestBuilder_Build(t *testing.T) {
	files := []parser.ParsedFile{
		{
			Path:     "src/main.ts",
			Language: parser.LanguageTypeScript,
			Functions: []parser.Function{
				{Name: "main", StartLine: 10, Exported: true},
				{Name: "helper", StartLine: 20, Exported: false},
			},
			Imports: []parser.Import{
				{Module: "./utils", Names: []string{"formatDate"}},
			},
		},
		{
			Path:     "src/utils.ts",
			Language: parser.LanguageTypeScript,
			Functions: []parser.Function{
				{Name: "formatDate", StartLine: 5, Exported: true},
			},
			Exports: []parser.Export{
				{Name: "formatDate", Kind: "function"},
			},
		},
	}

	builder := NewBuilder()
	graph := builder.Build(files)

	// Check nodes were created
	assert.NotEmpty(t, graph.Nodes)

	// Check file nodes exist
	assert.NotNil(t, graph.GetNode("file:src/main.ts"))
	assert.NotNil(t, graph.GetNode("file:src/utils.ts"))

	// Check function nodes exist
	assert.NotNil(t, graph.GetNode("func:src/main.ts:main"))
	assert.NotNil(t, graph.GetNode("func:src/main.ts:helper"))
	assert.NotNil(t, graph.GetNode("func:src/utils.ts:formatDate"))

	// Check contains edges
	containsEdges := 0
	for _, edge := range graph.Edges {
		if edge.Type == EdgeTypeContains {
			containsEdges++
		}
	}
	assert.Equal(t, 3, containsEdges) // 2 functions in main.ts + 1 in utils.ts
}

func TestBuilder_ClassInheritance(t *testing.T) {
	files := []parser.ParsedFile{
		{
			Path:     "src/animal.ts",
			Language: parser.LanguageTypeScript,
			Classes: []parser.Class{
				{Name: "Animal", StartLine: 1, Exported: true},
			},
		},
		{
			Path:     "src/dog.ts",
			Language: parser.LanguageTypeScript,
			Classes: []parser.Class{
				{Name: "Dog", StartLine: 1, Exported: true, Extends: "Animal"},
			},
			Imports: []parser.Import{
				{Module: "./animal", Names: []string{"Animal"}},
			},
		},
	}

	builder := NewBuilder()
	graph := builder.Build(files)

	// Check class nodes exist
	assert.NotNil(t, graph.GetNode("class:src/animal.ts:Animal"))
	assert.NotNil(t, graph.GetNode("class:src/dog.ts:Dog"))

	// Check extends edge exists
	extendsEdges := 0
	for _, edge := range graph.Edges {
		if edge.Type == EdgeTypeExtends {
			extendsEdges++
		}
	}
	assert.Equal(t, 1, extendsEdges)
}

func TestBuilder_ExternalModules(t *testing.T) {
	files := []parser.ParsedFile{
		{
			Path:     "src/app.ts",
			Language: parser.LanguageTypeScript,
			Imports: []parser.Import{
				{Module: "express", Names: []string{"Router"}},
				{Module: "lodash", Names: []string{"map", "filter"}},
			},
		},
	}

	builder := NewBuilder()
	graph := builder.Build(files)

	// Check external module nodes were created
	expressNode := graph.GetNode("module:express")
	assert.NotNil(t, expressNode)
	assert.Equal(t, NodeTypeModule, expressNode.Type)
	assert.Equal(t, "true", expressNode.Metadata["external"])

	lodashNode := graph.GetNode("module:lodash")
	assert.NotNil(t, lodashNode)
}

func TestNodeTypes(t *testing.T) {
	assert.Equal(t, NodeType("file"), NodeTypeFile)
	assert.Equal(t, NodeType("function"), NodeTypeFunction)
	assert.Equal(t, NodeType("class"), NodeTypeClass)
	assert.Equal(t, NodeType("module"), NodeTypeModule)
}

func TestEdgeTypes(t *testing.T) {
	assert.Equal(t, EdgeType("imports"), EdgeTypeImports)
	assert.Equal(t, EdgeType("calls"), EdgeTypeCalls)
	assert.Equal(t, EdgeType("extends"), EdgeTypeExtends)
	assert.Equal(t, EdgeType("uses"), EdgeTypeUses)
	assert.Equal(t, EdgeType("contains"), EdgeTypeContains)
}

func TestGraph_EmptyGraph(t *testing.T) {
	g := NewGraph()

	// Operations on empty graph should not panic
	assert.Empty(t, g.GetDependencies("nonexistent"))
	assert.Empty(t, g.GetDependents("nonexistent"))
	assert.Empty(t, g.GetTransitiveDependencies("nonexistent", 0))
	assert.Empty(t, g.GetTransitiveDependents("nonexistent", 0))
	assert.Empty(t, g.FindCycles())

	sorted, err := g.TopologicalSort()
	assert.NoError(t, err)
	assert.Empty(t, sorted)

	stats := g.GetStats()
	assert.Equal(t, 0, stats.TotalNodes)
	assert.Equal(t, 0, stats.TotalEdges)
}
