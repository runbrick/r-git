package graph

import (
	"fmt"
	"sort"
)

type bitSetFlags []uint64

func newBitSetFlags(size int) bitSetFlags {
	return make(bitSetFlags, (size+63)/64)
}

func (f bitSetFlags) get(index int) bool {
	return (f[index/64] & (1 << (index % 64))) != 0
}

func (f bitSetFlags) set(index int, value bool) {
	if value {
		f[index/64] |= (1 << (index % 64))
	} else {
		f[index/64] &= ^(1 << (index % 64))
	}
}

type permanentLinearGraphImpl struct {
	simpleNodes     bitSetFlags
	nodesCount      int
	nodeToEdgeIndex []int32
	longEdges       []int32
}

func (g *permanentLinearGraphImpl) NodesCount() int {
	return g.nodesCount
}

func (g *permanentLinearGraphImpl) GetAdjacentEdges(nodeIndex int, filter EdgeFilter) []GraphEdge {
	return g.AppendAdjacentEdges(nil, nodeIndex, filter)
}

func (g *permanentLinearGraphImpl) AppendAdjacentEdges(result []GraphEdge, nodeIndex int, filter EdgeFilter) []GraphEdge {

	hasUpSimpleEdge := nodeIndex != 0 && g.simpleNodes.get(nodeIndex-1)
	if hasUpSimpleEdge && filter.UpNormal {
		result = append(result, GraphEdge{UpNodeIndex: int32(nodeIndex - 1), DownNodeIndex: int32(nodeIndex), Type: Usual})
	}

	for i := g.nodeToEdgeIndex[nodeIndex]; i < g.nodeToEdgeIndex[nodeIndex+1]; i++ {
		adjacentNode := g.longEdges[i]

		if adjacentNode < 0 && filter.Special {
			result = append(result, CreateEdgeWithTargetId(int32(nodeIndex), adjacentNode, NotLoadCommit))
		}
		if adjacentNode < 0 {
			continue
		}

		if int32(nodeIndex) > adjacentNode && filter.UpNormal {
			result = append(result, GraphEdge{UpNodeIndex: adjacentNode, DownNodeIndex: int32(nodeIndex), Type: Usual})
		}
		if int32(nodeIndex) < adjacentNode && filter.DownNormal {
			result = append(result, GraphEdge{UpNodeIndex: int32(nodeIndex), DownNodeIndex: adjacentNode, Type: Usual})
		}
	}

	if g.simpleNodes.get(nodeIndex) && filter.DownNormal {
		result = append(result, GraphEdge{UpNodeIndex: int32(nodeIndex), DownNodeIndex: int32(nodeIndex + 1), Type: Usual})
	}

	return result
}

func (g *permanentLinearGraphImpl) GetNodeId(nodeIndex int) int32 {
	if nodeIndex < 0 || nodeIndex >= g.nodesCount {
		panic("Bad nodeIndex")
	}
	return int32(nodeIndex)
}

func (g *permanentLinearGraphImpl) GetNodeIndex(nodeId int32) (int, bool) {
	if nodeId >= 0 && int(nodeId) < g.nodesCount {
		return int(nodeId), true
	}
	return 0, false
}

func (g *permanentLinearGraphImpl) forEachParentNode(nodeIndex int, handler func(int)) {
	for i := g.nodeToEdgeIndex[nodeIndex]; i < g.nodeToEdgeIndex[nodeIndex+1]; i++ {
		adjacentNode := g.longEdges[i]
		if adjacentNode < 0 || int32(nodeIndex) < adjacentNode {
			handler(int(adjacentNode))
		}
	}
	if g.simpleNodes.get(nodeIndex) {
		handler(nodeIndex + 1)
	}
}

func fixDuplicateParentCommits(nodes []Node) []Node {
	fixed := make([]Node, len(nodes))
	for i, node := range nodes {
		parents := node.Parents
		if len(parents) <= 1 {
			fixed[i] = node
			continue
		}

		if len(parents) == 2 {
			if parents[0] == parents[1] {
				fixed[i] = Node{Id: node.Id, Parents: []int32{parents[0]}}
				continue
			}
			fixed[i] = node
			continue
		}

		allParents := make(map[int32]bool)
		for _, p := range parents {
			allParents[p] = true
		}

		if len(parents) == len(allParents) {
			fixed[i] = node
			continue
		}

		var correctParents []int32
		for _, p := range parents {
			if allParents[p] {
				correctParents = append(correctParents, p)
				delete(allParents, p)
			}
		}

		fixed[i] = Node{Id: node.Id, Parents: correctParents}
	}
	return fixed
}

type permanentLinearGraphBuilder struct {
	nodes           []Node
	simpleNodes     bitSetFlags
	nodesCount      int
	nodeToEdgeIndex []int32
	longEdges       []int32
	upAdjacentNodes map[int32][]int
}

func newPermanentLinearGraphBuilder(nodes []Node) *permanentLinearGraphBuilder {
	nodes = fixDuplicateParentCommits(nodes)
	nodesCount := len(nodes)
	simpleNodes := newBitSetFlags(nodesCount)

	longEdgesCount := 0

	for nodeIndex, node := range nodes {
		var nextCommitHashIndex int32 = -1
		if nodeIndex < nodesCount-1 {
			nextCommitHashIndex = nodes[nodeIndex+1].Id
		}

		parents := node.Parents
		if len(parents) == 1 && parents[0] == nextCommitHashIndex {
			simpleNodes.set(nodeIndex, true)
		} else {
			longEdgesCount += len(parents)
		}
	}

	return &permanentLinearGraphBuilder{
		nodes:           nodes,
		simpleNodes:     simpleNodes,
		nodesCount:      nodesCount,
		nodeToEdgeIndex: make([]int32, nodesCount+1),
		longEdges:       make([]int32, 2*longEdgesCount),
		upAdjacentNodes: make(map[int32][]int),
	}
}

func (b *permanentLinearGraphBuilder) addUnderdoneEdge(upNodeIndex int, downCommitId int32) {
	b.upAdjacentNodes[downCommitId] = append(b.upAdjacentNodes[downCommitId], upNodeIndex)
}

func (b *permanentLinearGraphBuilder) fixUnderdoneEdge(upNodeIndex int, downNodeIndex int, downCommitId int32) {
	end := b.nodeToEdgeIndex[upNodeIndex+1]

	upCommit := b.nodes[upNodeIndex]
	parents := upCommit.Parents

	for i, parentId := range parents {
		if parentId == downCommitId {
			offset := len(parents) - i
			edgeIndex := end - int32(offset)

			if b.longEdges[edgeIndex] == -1 {
				b.longEdges[edgeIndex] = int32(downNodeIndex)
				return
			} else {
				panic(fmt.Sprintf("Edge was set early!. Up node: %d, down node: %d", upNodeIndex, downNodeIndex))
			}
		}
	}
	panic(fmt.Sprintf("Not found underdone edges for node: %d. Adjacent down node: %d", upNodeIndex, downNodeIndex))
}

func (b *permanentLinearGraphBuilder) doStep(nodeIndex int) {
	node := b.nodes[nodeIndex]

	upNodes := b.upAdjacentNodes[node.Id]
	delete(b.upAdjacentNodes, node.Id)

	edgeIndex := b.nodeToEdgeIndex[nodeIndex]
	for _, upNodeIndex := range upNodes {
		b.fixUnderdoneEdge(upNodeIndex, nodeIndex, node.Id)
		b.longEdges[edgeIndex] = int32(upNodeIndex)
		edgeIndex++
	}

	if !b.simpleNodes.get(nodeIndex) {
		for _, downCommitId := range node.Parents {
			b.addUnderdoneEdge(nodeIndex, downCommitId)
			b.longEdges[edgeIndex] = -1
			edgeIndex++
		}
	}

	b.nodeToEdgeIndex[nodeIndex+1] = edgeIndex
}

func (b *permanentLinearGraphBuilder) fixUnderdoneEdgeForNotLoadCommit(upNodeIndex int, notLoadId int32) {
	for edgeIndex := b.nodeToEdgeIndex[upNodeIndex]; edgeIndex < b.nodeToEdgeIndex[upNodeIndex+1]; edgeIndex++ {
		if b.longEdges[edgeIndex] == -1 {
			b.longEdges[edgeIndex] = notLoadId
			return
		}
	}
	panic(fmt.Sprintf("Not found underdone edge to not load commit for node: %d", upNodeIndex))
}

func (b *permanentLinearGraphBuilder) fixUnderdoneEdges(notLoadedCommitToId func(int32) int32) {
	var commitIds []int32
	for id := range b.upAdjacentNodes {
		commitIds = append(commitIds, id)
	}

	sort.Slice(commitIds, func(i, j int) bool {
		minI := -1
		for _, n := range b.upAdjacentNodes[commitIds[i]] {
			if minI == -1 || n < minI {
				minI = n
			}
		}
		minJ := -1
		for _, n := range b.upAdjacentNodes[commitIds[j]] {
			if minJ == -1 || n < minJ {
				minJ = n
			}
		}
		return minI < minJ
	})

	for _, notLoadCommit := range commitIds {
		notLoadId := notLoadedCommitToId(notLoadCommit)
		for _, upNodeIndex := range b.upAdjacentNodes[notLoadCommit] {
			b.fixUnderdoneEdgeForNotLoadCommit(upNodeIndex, notLoadId)
		}
	}
}

func NewLinearGraph(nodes []Node) (LinearGraph, error) {
	return NewLinearGraphWithNotLoadedFunc(nodes, func(id int32) int32 { return -2147483648 })
}

func NewLinearGraphWithNotLoadedFunc(nodes []Node, notLoadedCommitToId func(int32) int32) (LinearGraph, error) {
	builder := newPermanentLinearGraphBuilder(nodes)
	for i := 0; i < builder.nodesCount; i++ {
		builder.doStep(i)
	}

	builder.fixUnderdoneEdges(notLoadedCommitToId)

	return &permanentLinearGraphImpl{
		simpleNodes:     builder.simpleNodes,
		nodesCount:      builder.nodesCount,
		nodeToEdgeIndex: builder.nodeToEdgeIndex,
		longEdges:       builder.longEdges,
	}, nil
}
