package graph

import (
	"sort"
)

type Layout interface {
	GetLayoutIndex(nodeIndex int) int
	GetOneOfHeadNodeIndex(nodeIndex int) int
	GetHeadNodeIndex() []int
}

type layoutImpl struct {
	layoutIndex             []int
	headNodes               []int
	layoutIndexForHeadNodes []int
}

func (l *layoutImpl) GetLayoutIndex(nodeIndex int) int {
	return l.layoutIndex[nodeIndex]
}

func (l *layoutImpl) GetOneOfHeadNodeIndex(nodeIndex int) int {
	return l.headNodes[l.getHeadOrder(l.GetLayoutIndex(nodeIndex))]
}

func (l *layoutImpl) GetHeadNodeIndex() []int {
	return l.headNodes
}

func (l *layoutImpl) getHeadOrder(layoutIndex int) int {
	i := sort.SearchInts(l.layoutIndexForHeadNodes, layoutIndex)
	if i < len(l.layoutIndexForHeadNodes) && l.layoutIndexForHeadNodes[i] == layoutIndex {
		return i
	}
	if i > 0 {
		return i - 1
	}
	return 0
}

func getLayoutIndexesForHeads(layoutIndexes []int, headNodeIndexes []int) []int {
	result := make([]int, len(headNodeIndexes))
	for i, headIndex := range headNodeIndexes {
		result[i] = layoutIndexes[headIndex]
	}
	return result
}

func getHeads(graph LinearGraph) []int {
	var heads []int
	for i := 0; i < graph.NodesCount(); i++ {
		if len(GetUpNodes(graph, i)) == 0 {
			heads = append(heads, i)
		}
	}
	return heads
}

func BuildLayout(graph LinearGraph, headsComparator func(int32, int32) int) Layout {
	return buildLayout(graph, nil, headsComparator)
}

func BuildLayoutWithBranches(graph LinearGraph, branches map[int]struct{}, headsComparator func(int32, int32) int) Layout {
	return buildLayout(graph, branches, headsComparator)
}

func buildLayout(graph LinearGraph, branches map[int]struct{}, headsComparator func(int32, int32) int) Layout {
	allHeadsMap := make(map[int]struct{})
	for b := range branches {
		allHeadsMap[b] = struct{}{}
	}
	for _, h := range getHeads(graph) {
		allHeadsMap[h] = struct{}{}
	}
	var sortedHeads []int
	for h := range allHeadsMap {
		sortedHeads = append(sortedHeads, h)
	}

	sort.Slice(sortedHeads, func(i, j int) bool {
		id1 := graph.GetNodeId(sortedHeads[i])
		id2 := graph.GetNodeId(sortedHeads[j])
		return headsComparator(id1, id2) < 0
	})

	return doBuildLayout(graph, sortedHeads)
}

func doBuildLayout(graph LinearGraph, sortedHeads []int) Layout {
	layoutIndex := make([]int, graph.NodesCount())
	var importantHeads []int

	currentLayoutIndex := 1
	for _, head := range sortedHeads {
		if layoutIndex[head] != 0 {
			continue
		}

		importantHeads = append(importantHeads, head)

		walk(head, func(currentNode int) int {
			firstVisit := layoutIndex[currentNode] == 0
			if firstVisit {
				layoutIndex[currentNode] = currentLayoutIndex
			}

			downNodes := GetDownNodes(graph, currentNode)
			var childWithoutLayoutIndex = -1
			for _, child := range downNodes {
				if layoutIndex[child] == 0 {
					childWithoutLayoutIndex = child
					break
				}
			}

			if childWithoutLayoutIndex == -1 {
				if firstVisit {
					currentLayoutIndex++
				}
				return -1 // Dfs.NextNode.NODE_NOT_FOUND
			}
			return childWithoutLayoutIndex
		})
	}

	return &layoutImpl{
		layoutIndex:             layoutIndex,
		headNodes:               importantHeads,
		layoutIndexForHeadNodes: getLayoutIndexesForHeads(layoutIndex, importantHeads),
	}
}
