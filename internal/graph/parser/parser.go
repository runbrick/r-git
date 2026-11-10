package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/runbrick/r-git/internal/graph"
)

type ParsedLine struct {
	NodeId int32
	Edges  []string
}

func ParseLine(line string) (*ParsedLine, error) {
	sepIndex := strings.Index(line, "|-")
	if sepIndex == -1 {
		return nil, fmt.Errorf("separator not found in line: %s", line)
	}

	left := line[:sepIndex]
	nodeId, err := ParseNodeId(left)
	if err != nil {
		return nil, err
	}

	right := line[sepIndex+2:]
	var edges []string
	if strings.TrimSpace(right) != "" {
		edges = strings.Fields(right)
	}

	return &ParsedLine{
		NodeId: nodeId,
		Edges:  edges,
	}, nil
}

func ParseNodeId(in string) (int32, error) {
	if idx := strings.IndexByte(in, '_'); idx != -1 {
		in = in[:idx]
	}
	id, err := strconv.ParseInt(in, 16, 32)
	if err != nil {
		return 0, err
	}
	return int32(id), nil
}

// ParseNodes parses input strings into graph.Node representations.
func ParseNodes(lines string) ([]graph.Node, error) {
	var nodes []graph.Node
	for _, line := range strings.Split(lines, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		parsed, err := ParseLine(line)
		if err != nil {
			return nil, err
		}
		
		node := graph.Node{Id: parsed.NodeId}
		for _, edge := range parsed.Edges {
			id, err := ParseNodeId(edge)
			if err != nil {
				return nil, err
			}
			node.Parents = append(node.Parents, id)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// LinearGraph parsing for strings like `0_U|-1_U 3_D`

type TestLinearGraphWithElementsInfo struct {
	nodes      []graph.Node // Actually we need graph elements, but we can implement LinearGraph
	nodesCount int
	upEdges    map[int][]graph.GraphEdge
	downEdges  map[int][]graph.GraphEdge
	nodeIdMap  map[int]int32
	nodeIdxMap map[int32]int
}

func (g *TestLinearGraphWithElementsInfo) NodesCount() int {
	return g.nodesCount
}

func (g *TestLinearGraphWithElementsInfo) GetAdjacentEdges(nodeIndex int, filter graph.EdgeFilter) []graph.GraphEdge {
	var result []graph.GraphEdge
	if filter.UpNormal || filter.Special {
		for _, e := range g.upEdges[nodeIndex] {
			if e.Type == graph.Usual && filter.UpNormal {
				result = append(result, e)
			}
			if e.Type != graph.Usual && filter.Special {
				result = append(result, e)
			}
		}
	}
	if filter.DownNormal || filter.Special {
		for _, e := range g.downEdges[nodeIndex] {
			if e.Type == graph.Usual && filter.DownNormal {
				result = append(result, e)
			}
			if e.Type != graph.Usual && filter.Special {
				result = append(result, e)
			}
		}
	}
	return result
}

func (g *TestLinearGraphWithElementsInfo) AppendAdjacentEdges(dst []graph.GraphEdge, nodeIndex int, filter graph.EdgeFilter) []graph.GraphEdge {
	return append(dst, g.GetAdjacentEdges(nodeIndex, filter)...)
}

func (g *TestLinearGraphWithElementsInfo) GetNodeId(nodeIndex int) int32 {
	return g.nodeIdMap[nodeIndex]
}

func (g *TestLinearGraphWithElementsInfo) GetNodeIndex(nodeId int32) (int, bool) {
	idx, ok := g.nodeIdxMap[nodeId]
	return idx, ok
}

func parseEdgeType(c byte) graph.EdgeType {
	switch c {
	case 'U', 'D', 'P', 'O':
		// In Java, DOTTED, DOTTED_ARROW_UP etc exist, but in Go we just have Usual and NotLoadCommit
		// For our tests, normal edge in Go includes what they call DOTTED maybe? Let's check linear.go
		return graph.Usual
	case 'N':
		return graph.NotLoadCommit
	default:
		return graph.Usual
	}
}

// ParseLinearGraph parses LinearGraph representations with types like `0_U|-1_U 3_D`
func ParseLinearGraph(lines string) (graph.LinearGraph, error) {
	linesArr := strings.Split(lines, "\n")
	
	nodeIdToNodeIndex := make(map[int32]int)
	
	type parsedEdge struct {
		nodeId int32
		char   byte
	}
	
	edges := make([][]parsedEdge, 0)
	nodeIds := make([]int32, 0)
	
	for _, line := range linesArr {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		sepIndex := strings.Index(line, "|-")
		if sepIndex == -1 {
			return nil, fmt.Errorf("separator not found in line: %s", line)
		}
		
		left := line[:sepIndex]
		charVal := left[len(left)-1]
		nodeId, err := ParseNodeId(left[:len(left)-2])
		if err != nil {
			return nil, err
		}
		
		nodeIdToNodeIndex[nodeId] = len(nodeIds)
		nodeIds = append(nodeIds, nodeId)
		
		right := line[sepIndex+2:]
		var normalEdges []parsedEdge
		if strings.TrimSpace(right) != "" {
			for _, s := range strings.Fields(right) {
				ec := s[len(s)-1]
				en, err := ParseNodeId(s[:len(s)-2])
				if err != nil {
					return nil, err
				}
				normalEdges = append(normalEdges, parsedEdge{en, ec})
			}
		}
		edges = append(edges, normalEdges)
		_ = charVal
	}
	
	upEdges := make(map[int][]graph.GraphEdge)
	downEdges := make(map[int][]graph.GraphEdge)
	
	for nodeIndex, nodeEdges := range edges {
		for _, parsedEdge := range nodeEdges {
			edgeType := parseEdgeType(parsedEdge.char)
			var edge graph.GraphEdge
			
			if edgeType == graph.Usual {
				downNodeIndex := nodeIdToNodeIndex[parsedEdge.nodeId]
				edge = graph.GraphEdge{
					UpNodeIndex:   int32(nodeIndex),
					DownNodeIndex: int32(downNodeIndex),
					Type:          edgeType,
				}
			} else {
				edge = graph.CreateEdgeWithTargetId(int32(nodeIndex), parsedEdge.nodeId, edgeType)
			}
			
			if edge.UpNodeIndex >= 0 {
				downEdges[int(edge.UpNodeIndex)] = append(downEdges[int(edge.UpNodeIndex)], edge)
			}
			if edge.DownNodeIndex >= 0 && edge.Type == graph.Usual {
				upEdges[int(edge.DownNodeIndex)] = append(upEdges[int(edge.DownNodeIndex)], edge)
			}
		}
	}
	
	g := &TestLinearGraphWithElementsInfo{
		nodesCount: len(nodeIds),
		upEdges:    upEdges,
		downEdges:  downEdges,
		nodeIdMap:  make(map[int]int32),
		nodeIdxMap: nodeIdToNodeIndex,
	}
	for idx, id := range nodeIds {
		g.nodeIdMap[idx] = id
	}
	
	return g, nil
}
