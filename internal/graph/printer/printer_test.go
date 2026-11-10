package printer

import (
	"testing"

	"github.com/runbrick/r-git/internal/graph"
)

type dummyColorManager struct{}

func (d *dummyColorManager) GetColor(headCommit int32, headFragmentIndex int, fragmentIndex int) int {
	return 1
}

func TestPrintElementGenerator(t *testing.T) {
	nodes := []graph.Node{
		{Id: 1, Parents: []int32{2, 3}},
		{Id: 2, Parents: []int32{4}},
		{Id: 3, Parents: []int32{4}},
		{Id: 4, Parents: []int32{5}},
		{Id: 5, Parents: []int32{}},
	}
	lg, err := graph.NewLinearGraph(nodes)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	layout := graph.BuildLayout(lg, func(id1, id2 int32) int { return int(id1 - id2) })
	cm := &dummyColorManager{}
	colorGetter := NewGraphColorGetterByHead(layout, lg, cm)

	pm := &PresentationManager{
		ColorGetter: colorGetter,
		LinearGraph: lg,
		Layout:      layout,
	}

	gen := NewPrintElementGenerator(lg, pm, true)
	var buf RowBuffer
	gen.GetRow(0, &buf)

	if len(buf.Nodes) == 0 {
		t.Errorf("expected nodes in row 0")
	}
}
