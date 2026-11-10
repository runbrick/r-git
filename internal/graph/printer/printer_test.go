package printer

import (
	"reflect"
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

func TestPreparedRowsMatchesGeneratorWithoutQueryAllocations(t *testing.T) {
	nodes := []graph.Node{
		{Id: 0, Parents: []int32{1, 2}},
		{Id: 1, Parents: []int32{3}},
		{Id: 2, Parents: []int32{3}},
		{Id: 3},
	}
	lg, err := graph.NewLinearGraph(nodes)
	if err != nil {
		t.Fatal(err)
	}
	layout := graph.BuildLayout(lg, func(a, b int32) int { return int(a - b) })
	pm := &PresentationManager{
		ColorGetter: NewGraphColorGetterByHead(layout, lg, &dummyColorManager{}),
		LinearGraph: lg,
		Layout:      layout,
	}
	gen := NewPrintElementGenerator(lg, pm, true)
	prepared := gen.PrepareRows()
	buf := prepared.NewBuffer()
	var expected RowBuffer
	for row := 0; row < lg.NodesCount(); row++ {
		gen.GetRow(row, &expected)
		prepared.GetRow(row, buf)
		if !reflect.DeepEqual(*buf, expected) {
			t.Fatalf("row %d differs: got %#v, want %#v", row, *buf, expected)
		}
	}
	if allocs := testing.AllocsPerRun(100, func() { prepared.GetRow(1, buf) }); allocs != 0 {
		t.Fatalf("prepared row query allocated %v times", allocs)
	}
}
