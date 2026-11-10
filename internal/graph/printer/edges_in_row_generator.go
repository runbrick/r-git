package printer

import (
	"github.com/runbrick/r-git/internal/graph"
)

type EdgeMap map[graph.GraphEdge]struct{}

func (m EdgeMap) AddAll(edges []graph.GraphEdge) {
	for _, e := range edges {
		m[e] = struct{}{}
	}
}

func (m EdgeMap) RemoveAll(edges []graph.GraphEdge) {
	for _, e := range edges {
		delete(m, e)
	}
}

type graphEdges struct {
	edges EdgeMap
	row   int
}

func (ge *graphEdges) copyInstance() *graphEdges {
	newEdges := make(EdgeMap, len(ge.edges))
	for k, v := range ge.edges {
		newEdges[k] = v
	}
	return &graphEdges{
		edges: newEdges,
		row:   ge.row,
	}
}

type EdgesInRowGenerator struct {
	graph    graph.LinearGraph
	walkSize int
	cacheNU  map[int]*graphEdges // Simplified without LRU, could use simple limit or pass-through
	cacheND  map[int]*graphEdges
}

const blockSize = 40

func NewEdgesInRowGenerator(g graph.LinearGraph, walkSize int) *EdgesInRowGenerator {
	if walkSize == 0 {
		walkSize = 1000
	}
	return &EdgesInRowGenerator{
		graph:    g,
		walkSize: walkSize,
		cacheNU:  make(map[int]*graphEdges),
		cacheND:  make(map[int]*graphEdges),
	}
}

func (g *EdgesInRowGenerator) GetEdgesInRow(rowIndex int) EdgeMap {
	neighborU := g.getNeighborU(rowIndex)
	for neighborU.row < rowIndex {
		neighborU = g.oneDownStep(neighborU)
	}

	neighborD := g.getNeighborD(rowIndex)
	for neighborD.row > rowIndex {
		neighborD = g.oneUpStep(neighborD)
	}

	result := neighborU.edges
	result.AddAll(mapToSlice(neighborD.edges))
	return result
}

func (g *EdgesInRowGenerator) getNeighborU(rowIndex int) *graphEdges {
	upNeighborIndex := (rowIndex / blockSize) * blockSize
	if ge, ok := g.cacheNU[upNeighborIndex]; ok {
		return ge.copyInstance()
	}
	ge := g.getUCorrectEdges(upNeighborIndex)
	g.cacheNU[upNeighborIndex] = ge
	return ge.copyInstance()
}

func (g *EdgesInRowGenerator) getNeighborD(rowIndex int) *graphEdges {
	downNeighborIndex := (rowIndex/blockSize)*blockSize + blockSize
	if downNeighborIndex >= g.graph.NodesCount() {
		return &graphEdges{
			edges: make(EdgeMap),
			row:   g.graph.NodesCount() - 1,
		}
	}

	if ge, ok := g.cacheND[downNeighborIndex]; ok {
		return ge.copyInstance()
	}
	ge := g.getDCorrectEdges(downNeighborIndex)
	g.cacheND[downNeighborIndex] = ge
	return ge.copyInstance()
}

func (g *EdgesInRowGenerator) getUCorrectEdges(rowIndex int) *graphEdges {
	startCalculateIndex := rowIndex - g.walkSize
	if startCalculateIndex < 0 {
		startCalculateIndex = 0
	}
	ge := &graphEdges{edges: make(EdgeMap), row: startCalculateIndex}
	for i := startCalculateIndex; i < rowIndex; i++ {
		ge = g.oneDownStep(ge)
	}
	return ge
}

func (g *EdgesInRowGenerator) getDCorrectEdges(rowIndex int) *graphEdges {
	endCalculateIndex := rowIndex + g.walkSize
	if max := g.graph.NodesCount() - 1; endCalculateIndex > max {
		endCalculateIndex = max
	}
	ge := &graphEdges{edges: make(EdgeMap), row: endCalculateIndex}
	for i := endCalculateIndex; i > rowIndex; i-- {
		ge = g.oneUpStep(ge)
	}
	return ge
}

func (g *EdgesInRowGenerator) oneDownStep(ge *graphEdges) *graphEdges {
	currentRow := ge.row
	edges := ge.edges
	edges.AddAll(g.graph.GetAdjacentEdges(currentRow, graph.NormalDown))
	edges.RemoveAll(g.graph.GetAdjacentEdges(currentRow+1, graph.NormalUp))
	return &graphEdges{edges: edges, row: currentRow + 1}
}

func (g *EdgesInRowGenerator) oneUpStep(ge *graphEdges) *graphEdges {
	currentRow := ge.row
	edges := ge.edges
	edges.AddAll(g.graph.GetAdjacentEdges(currentRow, graph.NormalUp))
	edges.RemoveAll(g.graph.GetAdjacentEdges(currentRow-1, graph.NormalDown))
	return &graphEdges{edges: edges, row: currentRow - 1}
}

func mapToSlice(m EdgeMap) []graph.GraphEdge {
	res := make([]graph.GraphEdge, 0, len(m))
	for e := range m {
		res = append(res, e)
	}
	return res
}
