package log

import (
	"fmt"

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
}

type themeColors struct{}

func (themeColors) GetColor(headCommit int32, headFragmentIndex, fragmentIndex int) int {
	return int(headCommit % 5)
}

// NewGraphRows prepares graph rows from commits in newest-first order.
func NewGraphRows(ordered []repo.CommitParents) (*GraphRows, map[model.Hash]int, error) {
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
	getter := printer.NewGraphColorGetterByHead(layout, linear, themeColors{})
	generator := printer.NewPrintElementGenerator(linear, &printer.PresentationManager{ColorGetter: getter, LinearGraph: linear, Layout: layout}, false)
	prepared := generator.PrepareRows()
	return &GraphRows{rows: prepared, buffer: prepared.NewBuffer(), count: len(ordered)}, indexes, nil
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
