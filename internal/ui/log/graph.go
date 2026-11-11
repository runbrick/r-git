package log

import (
	"fmt"
	"unicode/utf16"

	"github.com/runbrick/r-git/internal/git/model"
	"github.com/runbrick/r-git/internal/git/repo"
	"github.com/runbrick/r-git/internal/graph"
	"github.com/runbrick/r-git/internal/graph/printer"
)

// GraphRows owns the graph data and reusable query buffer for a loaded log.
type GraphRows struct {
	rows   *printer.PreparedRows
	buffer *printer.RowBuffer
	count  int
	width  float32
}

type themeColors map[int32]string

func (refs themeColors) GetColor(headCommit int32, headFragmentIndex, fragmentIndex int) int {
	if headFragmentIndex != fragmentIndex {
		return fragmentIndex
	}
	// Java String.hashCode operates on UTF-16 code units, with signed Int overflow.
	var hash int32
	for _, unit := range utf16.Encode([]rune(refs[headCommit])) {
		hash = hash*31 + int32(unit)
	}
	return int(hash)
}

// NewGraphRows prepares graph rows from commits in newest-first order.
func NewGraphRows(ordered []repo.CommitParents, refs ...repo.Reference) (*GraphRows, map[model.Hash]int, error) {
	if len(ordered) == 0 {
		return &GraphRows{}, map[model.Hash]int{}, nil
	}
	indexes := make(map[model.Hash]int, len(ordered))
	for i := range ordered {
		indexes[ordered[i].Commit] = i
	}
	nodes := make([]graph.Node, len(ordered))
	for i := range ordered {
		nodes[i].Id = int32(i)
		for _, parent := range ordered[i].Parents {
			if index, ok := indexes[parent]; ok {
				nodes[i].Parents = append(nodes[i].Parents, int32(index))
			} else {
				nodes[i].Parents = append(nodes[i].Parents, -1)
			}
		}
	}
	linear, err := graph.NewLinearGraph(nodes)
	if err != nil {
		return nil, nil, fmt.Errorf("prepare commit graph: %w", err)
	}
	layout := graph.BuildLayout(linear, func(a, b int32) int {
		switch {
		case a < b:
			return -1
		case a > b:
			return 1
		default:
			return 0
		}
	})
	colors := themeColors{}
	best := make(map[int32]model.Ref)
	for _, ref := range refs {
		if index, ok := indexes[ref.Hash]; ok {
			key := int32(index)
			old, exists := best[key]
			if !exists || refPriority(ref.Ref) < refPriority(old) || refPriority(ref.Ref) == refPriority(old) && model.NaturalCompare(ref.Ref.Name(), old.Name()) < 0 {
				best[key] = ref.Ref
				colors[key] = ref.Ref.Name()
			}
		}
	}
	getter := printer.NewGraphColorGetterByHead(layout, linear, colors)
	generator := printer.NewPrintElementGenerator(linear, &printer.PresentationManager{ColorGetter: getter, LinearGraph: linear, Layout: layout}, false)
	prepared := generator.PrepareRows()
	buffer := prepared.NewBuffer()
	maxLane := 0
	for row := range ordered {
		prepared.GetRow(row, buffer)
		for _, node := range buffer.Nodes {
			maxLane = max(maxLane, node.PositionInCurrentRow)
		}
		for _, edge := range buffer.Edges {
			maxLane = max(maxLane, edge.PositionInCurrentRow, edge.PositionInOtherRow)
		}
	}
	return &GraphRows{rows: prepared, buffer: buffer, count: len(ordered), width: float32(maxLane+1) * 16}, indexes, nil
}

// Row copies the prepared row into the caller-owned buffer.
func (g *GraphRows) Row(index int) *printer.RowBuffer {
	if g == nil || g.rows == nil || index < 0 || index >= g.rowsCount() {
		return nil
	}
	g.rows.GetRow(index, g.buffer)
	return g.buffer
}

func (g *GraphRows) rowsCount() int {
	return g.count
}

// GitRefManager.GitBranchLayoutComparator prefers tracked heads, then main
// branches, other local branches and tags, independently of enumeration order.
func refPriority(ref model.Ref) int {
	switch ref.Kind() {
	case model.KindRemoteBranch:
		if ref.Name() == "origin/main" || ref.Name() == "origin/master" {
			return 0
		}
		return 1
	case model.KindLocalBranch:
		if ref.Name() == "main" || ref.Name() == "master" {
			return 2
		}
		return 3
	case model.KindTag:
		return 4
	case model.KindHead:
		return 6
	default:
		return 7
	}
}
