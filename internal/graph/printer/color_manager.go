package printer

import (
	"github.com/runbrick/r-git/internal/graph"
)

type ColorManager interface {
	GetColor(headCommit int32, headFragmentIndex int, fragmentIndex int) int
}

type GraphColorGetterByHead struct {
	graphLayout  graph.Layout
	colorManager ColorManager
	linearGraph  graph.LinearGraph
}

func NewGraphColorGetterByHead(graphLayout graph.Layout, linearGraph graph.LinearGraph, colorManager ColorManager) *GraphColorGetterByHead {
	return &GraphColorGetterByHead{
		graphLayout:  graphLayout,
		colorManager: colorManager,
		linearGraph:  linearGraph,
	}
}

func (g *GraphColorGetterByHead) GetNodeColor(nodeId int32, layoutIndex int) int {
	var headNodeIndex int = 0
	if nodeId >= 0 {
		if nodeIndex, ok := g.linearGraph.GetNodeIndex(nodeId); ok {
			headNodeIndex = g.graphLayout.GetOneOfHeadNodeIndex(nodeIndex)
		}
	}
	headCommitId := g.linearGraph.GetNodeId(headNodeIndex)
	headLayoutIndex := g.graphLayout.GetLayoutIndex(headNodeIndex)
	return g.colorManager.GetColor(headCommitId, headLayoutIndex, layoutIndex)
}
