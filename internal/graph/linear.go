package graph

type Node struct {
	Id      int32
	Parents []int32
}

type EdgeType int

const (
	Usual EdgeType = iota
	NotLoadCommit
)

type GraphEdge struct {
	UpNodeIndex   int32
	DownNodeIndex int32
	TargetId      int32 // used for NotLoadCommit if DownNodeIndex is missing/negative, else 0.
	Type          EdgeType
}

func CreateEdgeWithTargetId(upNodeIndex int32, targetId int32, edgeType EdgeType) GraphEdge {
	return GraphEdge{
		UpNodeIndex:   upNodeIndex,
		DownNodeIndex: targetId,
		TargetId:      targetId,
		Type:          edgeType,
	}
}

type EdgeFilter struct {
	UpNormal   bool
	DownNormal bool
	Special    bool
}

var (
	NormalUp   = EdgeFilter{UpNormal: true, DownNormal: false, Special: false}
	NormalDown = EdgeFilter{UpNormal: false, DownNormal: true, Special: false}
	NormalAll  = EdgeFilter{UpNormal: true, DownNormal: true, Special: false}
	All        = EdgeFilter{UpNormal: true, DownNormal: true, Special: true}
)

type LinearGraph interface {
	NodesCount() int
	GetAdjacentEdges(nodeIndex int, filter EdgeFilter) []GraphEdge
	AppendAdjacentEdges(dst []GraphEdge, nodeIndex int, filter EdgeFilter) []GraphEdge
	GetNodeId(nodeIndex int) int32
	GetNodeIndex(nodeId int32) (int, bool)
}

type NodeFilter struct {
	Up         bool
	Down       bool
	EdgeFilter EdgeFilter
}

var (
	Up   = NodeFilter{Up: true, Down: false, EdgeFilter: NormalUp}
	Down = NodeFilter{Up: false, Down: true, EdgeFilter: NormalDown}
)

type LiteLinearGraph interface {
	NodesCount() int
	GetNodes(nodeIndex int, filter NodeFilter) []int
}
