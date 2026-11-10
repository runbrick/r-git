package printer

import (
	"math"
	"sort"

	"github.com/runbrick/r-git/internal/graph"
)

type ElementType int

const (
	NodeElement ElementType = iota
	EdgeElement
)

type LineStyle int

const (
	LineSolid LineStyle = iota
	LineDashed
	LineDotted
)

type EdgePrintType int

const (
	EdgeUp EdgePrintType = iota
	EdgeDown
)

type NodeType int

const (
	NodeFill NodeType = iota
	NodeOutline
	NodeOutlineAndFill
)

type PrintElement struct {
	Type                 ElementType
	RowIndex             int
	PositionInCurrentRow int
	ColorId              int
	IsSelected           bool

	// Node specific
	NodeIndex int32
	NodeType  NodeType

	// Edge specific
	PositionInOtherRow int
	EdgePrintType      EdgePrintType
	LineStyle          LineStyle
	HasArrow           bool
	GraphEdge          graph.GraphEdge
}

type RowBuffer struct {
	Nodes []PrintElement
	Edges []PrintElement
}

func (rb *RowBuffer) Clear() {
	rb.Nodes = rb.Nodes[:0]
	rb.Edges = rb.Edges[:0]
}

const (
	VeryLongEdgeSize     = 1000
	LongEdgeSize         = 30
	VeryLongEdgePartSize = 250
	LongEdgePartSize     = 1
	CacheSize            = 100
)

type ColorGetter interface {
	GetNodeColor(nodeId int32, layoutIndex int) int
}

type PresentationManager struct {
	ColorGetter ColorGetter
	LinearGraph graph.LinearGraph
	Layout      graph.Layout
}

func (pm *PresentationManager) GetColorIdNode(nodeIndex int32) int {
	nodeId := pm.LinearGraph.GetNodeId(int(nodeIndex))
	return pm.ColorGetter.GetNodeColor(nodeId, pm.getLayoutIndex(nodeId))
}

func (pm *PresentationManager) GetColorIdEdge(edge graph.GraphEdge) int {
	if edge.Type != graph.Usual {
		nodeIndex := edge.UpNodeIndex
		if nodeIndex < 0 {
			nodeIndex = edge.DownNodeIndex
		}
		nodeId := pm.LinearGraph.GetNodeId(int(nodeIndex))
		return pm.ColorGetter.GetNodeColor(nodeId, pm.getLayoutIndex(nodeId))
	}
	upNodeId := pm.LinearGraph.GetNodeId(int(edge.UpNodeIndex))
	downNodeId := pm.LinearGraph.GetNodeId(int(edge.DownNodeIndex))
	upLayoutIndex := pm.getLayoutIndex(upNodeId)
	downLayoutIndex := pm.getLayoutIndex(downNodeId)
	if upLayoutIndex >= downLayoutIndex {
		return pm.ColorGetter.GetNodeColor(upNodeId, upLayoutIndex)
	}
	return pm.ColorGetter.GetNodeColor(downNodeId, downLayoutIndex)
}

func (pm *PresentationManager) getLayoutIndex(nodeId int32) int {
	if nodeId < 0 {
		return int(nodeId)
	}
	nodeIndex, ok := pm.LinearGraph.GetNodeIndex(nodeId)
	if !ok {
		return 0
	}
	return pm.Layout.GetLayoutIndex(nodeIndex)
}

type PrintElementGenerator struct {
	linearGraph         graph.LinearGraph
	presentationManager *PresentationManager
	longEdgeSize        int
	visiblePartSize     int
	edgeWithArrowSize   int
	edgesInRowGen       *EdgesInRowGenerator

	cache map[int][]GraphElementWithIndex
}

type GraphElementWithIndex struct {
	IsNode    bool
	NodeIndex int             // valid if IsNode
	Edge      graph.GraphEdge // valid if !IsNode
}

func NewPrintElementGenerator(
	g graph.LinearGraph,
	pm *PresentationManager,
	showLongEdges bool,
) *PrintElementGenerator {
	longSize := LongEdgeSize
	partSize := LongEdgePartSize
	arrowSize := math.MaxInt32
	if showLongEdges {
		longSize = VeryLongEdgeSize
		partSize = VeryLongEdgePartSize
		arrowSize = LongEdgeSize
	}

	return &PrintElementGenerator{
		linearGraph:         g,
		presentationManager: pm,
		longEdgeSize:        longSize,
		visiblePartSize:     partSize,
		edgeWithArrowSize:   arrowSize,
		edgesInRowGen:       NewEdgesInRowGenerator(g, 1000),
		cache:               make(map[int][]GraphElementWithIndex),
	}
}

func convertToLineStyle(edgeType graph.EdgeType) LineStyle {
	if edgeType == graph.Usual || edgeType == graph.NotLoadCommit {
		return LineSolid
	}
	// Add dotted if added to EdgeType
	return LineDashed
}

func (gen *PrintElementGenerator) GetRow(rowIndex int, buf *RowBuffer) {
	buf.Clear()
	visibleElements := gen.getSortedVisibleElementsInRow(rowIndex)

	upPosition := gen.createEndPositionFunction(rowIndex-1, true)
	downPosition := gen.createEndPositionFunction(rowIndex+1, false)
	var adjBuf [8]graph.GraphEdge

	for position, element := range visibleElements {
		if element.IsNode {
			nodeIndex := element.NodeIndex
			colorId := gen.presentationManager.GetColorIdNode(int32(nodeIndex))
			buf.Nodes = append(buf.Nodes, PrintElement{
				Type:                 NodeElement,
				RowIndex:             rowIndex,
				PositionInCurrentRow: position,
				ColorId:              colorId,
				NodeIndex:            int32(nodeIndex),
				NodeType:             NodeFill,
			})
			adjEdges := gen.linearGraph.AppendAdjacentEdges(adjBuf[:0], nodeIndex, graph.All)
			for _, edge := range adjEdges {
				arrowType, hasArrowType := gen.getArrowType(edge, rowIndex)
				down, downOk := downPosition(edge)
				up, upOk := upPosition(edge)

				lineStyle := convertToLineStyle(edge.Type)
				edgeColorId := gen.presentationManager.GetColorIdEdge(edge)

				if downOk {
					hasArrow := hasArrowType && arrowType == EdgeDown
					buf.Edges = append(buf.Edges, PrintElement{
						Type:                 EdgeElement,
						RowIndex:             rowIndex,
						PositionInCurrentRow: position,
						PositionInOtherRow:   down,
						ColorId:              edgeColorId,
						EdgePrintType:        EdgeDown,
						LineStyle:            lineStyle,
						HasArrow:             hasArrow,
						GraphEdge:            edge,
					})
				}
				if upOk {
					hasArrow := hasArrowType && arrowType == EdgeUp
					buf.Edges = append(buf.Edges, PrintElement{
						Type:                 EdgeElement,
						RowIndex:             rowIndex,
						PositionInCurrentRow: position,
						PositionInOtherRow:   up,
						ColorId:              edgeColorId,
						EdgePrintType:        EdgeUp,
						LineStyle:            lineStyle,
						HasArrow:             hasArrow,
						GraphEdge:            edge,
					})
				}
			}
		} else {
			elementEdge := element.Edge
			down, downOk := downPosition(elementEdge)
			up, upOk := upPosition(elementEdge)
			arrowType, hasArrowType := gen.getArrowType(elementEdge, rowIndex)

			lineStyle := convertToLineStyle(elementEdge.Type)
			edgeColorId := gen.presentationManager.GetColorIdEdge(elementEdge)

			if downOk {
				hasArrow := hasArrowType && arrowType == EdgeDown
				buf.Edges = append(buf.Edges, PrintElement{
					Type:                 EdgeElement,
					RowIndex:             rowIndex,
					PositionInCurrentRow: position,
					PositionInOtherRow:   down,
					ColorId:              edgeColorId,
					EdgePrintType:        EdgeDown,
					LineStyle:            lineStyle,
					HasArrow:             hasArrow,
					GraphEdge:            elementEdge,
				})
			} else if hasArrowType && arrowType == EdgeDown {
				buf.Edges = append(buf.Edges, PrintElement{
					Type:                 EdgeElement,
					RowIndex:             rowIndex,
					PositionInCurrentRow: position,
					PositionInOtherRow:   position, // TerminalEdgePrintElement uses same position
					ColorId:              edgeColorId,
					EdgePrintType:        EdgeDown,
					LineStyle:            lineStyle,
					HasArrow:             true,
					GraphEdge:            elementEdge,
				})
			}
			if upOk {
				hasArrow := hasArrowType && arrowType == EdgeUp
				buf.Edges = append(buf.Edges, PrintElement{
					Type:                 EdgeElement,
					RowIndex:             rowIndex,
					PositionInCurrentRow: position,
					PositionInOtherRow:   up,
					ColorId:              edgeColorId,
					EdgePrintType:        EdgeUp,
					LineStyle:            lineStyle,
					HasArrow:             hasArrow,
					GraphEdge:            elementEdge,
				})
			} else if hasArrowType && arrowType == EdgeUp {
				buf.Edges = append(buf.Edges, PrintElement{
					Type:                 EdgeElement,
					RowIndex:             rowIndex,
					PositionInCurrentRow: position,
					PositionInOtherRow:   position,
					ColorId:              edgeColorId,
					EdgePrintType:        EdgeUp,
					LineStyle:            lineStyle,
					HasArrow:             true,
					GraphEdge:            elementEdge,
				})
			}
		}
	}
}

func (gen *PrintElementGenerator) createEndPositionFunction(visibleRowIndex int, up bool) func(graph.GraphEdge) (int, bool) {
	if visibleRowIndex < 0 || visibleRowIndex >= gen.linearGraph.NodesCount() {
		return func(edge graph.GraphEdge) (int, bool) { return 0, false }
	}

	visibleElementsInNextRow := gen.getSortedVisibleElementsInRow(visibleRowIndex)
	return func(edge graph.GraphEdge) (int, bool) {
		// Search for edge
		for position, element := range visibleElementsInNextRow {
			if !element.IsNode && element.Edge == edge {
				return position, true
			}
		}

		nodeIndex := edge.DownNodeIndex
		if up {
			nodeIndex = edge.UpNodeIndex
		}
		if nodeIndex >= 0 { // >=0 is a valid node index in this context
			for position, element := range visibleElementsInNextRow {
				if element.IsNode && element.NodeIndex == int(nodeIndex) {
					return position, true
				}
			}
		}
		return 0, false
	}
}

func (gen *PrintElementGenerator) getArrowType(edge graph.GraphEdge, rowIndex int) (EdgePrintType, bool) {
	if edge.Type == graph.Usual {
		edgeSize := int(edge.DownNodeIndex - edge.UpNodeIndex)
		upOffset := rowIndex - int(edge.UpNodeIndex)
		downOffset := int(edge.DownNodeIndex) - rowIndex

		if edgeSize >= gen.longEdgeSize {
			if upOffset == gen.visiblePartSize {
				return EdgeDown, true
			}
			if downOffset == gen.visiblePartSize {
				return EdgeUp, true
			}
		}
		if edgeSize >= gen.edgeWithArrowSize {
			if upOffset == 1 {
				return EdgeDown, true
			}
			if downOffset == 1 {
				return EdgeUp, true
			}
		}
		return 0, false
	}

	// special edges
	if edge.Type == graph.NotLoadCommit { // mapped to DOTTED_ARROW_DOWN and NOT_LOAD_COMMIT behavior
		if int(edge.UpNodeIndex) == rowIndex-1 {
			return EdgeDown, true
		}
	}
	// If DOTTED_ARROW_UP added to EdgeType:
	// if edge.Type == graph.DottedArrowUp && int(edge.DownNodeIndex) == rowIndex+1 { return EdgeUp, true }

	return 0, false
}

func (gen *PrintElementGenerator) isEdgeVisibleInRow(edge graph.GraphEdge, visibleRowIndex int) bool {
	if edge.Type != graph.Usual {
		return false
	}
	edgeSize := int(edge.DownNodeIndex - edge.UpNodeIndex)
	if edgeSize < gen.longEdgeSize {
		return true
	}
	attachmentDistance := visibleRowIndex - int(edge.UpNodeIndex)
	if downDist := int(edge.DownNodeIndex) - visibleRowIndex; downDist < attachmentDistance {
		attachmentDistance = downDist
	}
	return attachmentDistance <= gen.visiblePartSize
}

func (gen *PrintElementGenerator) getSortedVisibleElementsInRow(rowIndex int) []GraphElementWithIndex {
	if cached, ok := gen.cache[rowIndex]; ok {
		return cached
	}

	result := make([]GraphElementWithIndex, 0)
	result = append(result, GraphElementWithIndex{IsNode: true, NodeIndex: rowIndex})

	edges := gen.edgesInRowGen.GetEdgesInRow(rowIndex)
	for edge := range edges {
		if gen.isEdgeVisibleInRow(edge, rowIndex) {
			result = append(result, GraphElementWithIndex{IsNode: false, Edge: edge})
		}
	}

	if rowIndex > 0 {
		var adjBuf [8]graph.GraphEdge
		adjEdges := gen.linearGraph.AppendAdjacentEdges(adjBuf[:0], rowIndex-1, graph.EdgeFilter{Special: true})
		for _, edge := range adjEdges {
			if int(edge.UpNodeIndex) == rowIndex-1 { // isEdgeDown
				result = append(result, GraphElementWithIndex{IsNode: false, Edge: edge})
			}
		}
	}
	if rowIndex < gen.linearGraph.NodesCount()-1 {
		var adjBuf [8]graph.GraphEdge
		adjEdges := gen.linearGraph.AppendAdjacentEdges(adjBuf[:0], rowIndex+1, graph.EdgeFilter{Special: true})
		for _, edge := range adjEdges {
			if int(edge.DownNodeIndex) == rowIndex+1 { // isEdgeUp
				result = append(result, GraphElementWithIndex{IsNode: false, Edge: edge})
			}
		}
	}

	layout := gen.presentationManager.Layout

	sort.Slice(result, func(i, j int) bool {
		return compareElements(result[i], result[j], layout) < 0
	})

	gen.cache[rowIndex] = result
	return result
}

func compareElements(e1, e2 GraphElementWithIndex, layout graph.Layout) int {
	if !e1.IsNode && !e2.IsNode {
		edge1 := e1.Edge
		edge2 := e2.Edge
		if edge1.Type != graph.Usual && edge2.Type == graph.Usual {
			return -compareEdgeNode(edge2, getNotNullNodeIndex(edge1), layout)
		}
		if edge1.Type == graph.Usual && edge2.Type != graph.Usual {
			return compareEdgeNode(edge1, getNotNullNodeIndex(edge2), layout)
		}
		if edge1.Type != graph.Usual && edge2.Type != graph.Usual {
			return getLayoutIndex(getNotNullNodeIndex(edge1), layout) - getLayoutIndex(getNotNullNodeIndex(edge2), layout)
		}

		if edge1.UpNodeIndex == edge2.UpNodeIndex {
			if edge1.DownNodeIndex < edge2.DownNodeIndex {
				return -compareEdgeNode(edge2, int(edge1.DownNodeIndex), layout)
			} else {
				return compareEdgeNode(edge1, int(edge2.DownNodeIndex), layout)
			}
		}

		if edge1.UpNodeIndex < edge2.UpNodeIndex {
			return compareEdgeNode(edge1, int(edge2.UpNodeIndex), layout)
		} else {
			return -compareEdgeNode(edge2, int(edge1.UpNodeIndex), layout)
		}
	}

	if !e1.IsNode && e2.IsNode {
		return compareEdgeNode(e1.Edge, e2.NodeIndex, layout)
	}

	if e1.IsNode && !e2.IsNode {
		return -compareEdgeNode(e2.Edge, e1.NodeIndex, layout)
	}

	return 0
}

func getNotNullNodeIndex(edge graph.GraphEdge) int {
	if edge.UpNodeIndex >= 0 {
		return int(edge.UpNodeIndex)
	}
	return int(edge.DownNodeIndex)
}

func compareEdgeNode(edge graph.GraphEdge, nodeIndex int, layout graph.Layout) int {
	if edge.Type != graph.Usual {
		return getLayoutIndex(getNotNullNodeIndex(edge), layout) - getLayoutIndex(nodeIndex, layout)
	}

	upEdgeLI := getLayoutIndex(int(edge.UpNodeIndex), layout)
	downEdgeLI := getLayoutIndex(int(edge.DownNodeIndex), layout)
	nodeLI := getLayoutIndex(nodeIndex, layout)

	maxEdgeLI := upEdgeLI
	if downEdgeLI > maxEdgeLI {
		maxEdgeLI = downEdgeLI
	}

	if maxEdgeLI != nodeLI {
		return maxEdgeLI - nodeLI
	}
	return int(edge.UpNodeIndex) - nodeIndex
}

func getLayoutIndex(nodeIndex int, layout graph.Layout) int {
	return layout.GetLayoutIndex(nodeIndex)
}
