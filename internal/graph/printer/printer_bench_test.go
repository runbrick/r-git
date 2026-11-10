package printer

import (
	"testing"

	"github.com/runbrick/r-git/internal/graph"
)

func BenchmarkQuery50Rows100k(b *testing.B) {
	const size = 100_000
	nodes := make([]graph.Node, size)
	for i := range nodes {
		nodes[i].Id = int32(i)
		if i+1 < size {
			nodes[i].Parents = []int32{int32(i + 1)}
		}
		if i%100 == 0 && i+100 < size {
			nodes[i].Parents = append(nodes[i].Parents, int32(i+100))
		}
	}
	lg, err := graph.NewLinearGraph(nodes)
	if err != nil {
		b.Fatal(err)
	}
	layout := graph.BuildLayout(lg, func(a, c int32) int { return int(a - c) })
	pm := &PresentationManager{
		ColorGetter: NewGraphColorGetterByHead(layout, lg, &dummyColorManager{}),
		LinearGraph: lg,
		Layout:      layout,
	}
	prepared := NewPrintElementGenerator(lg, pm, true).PrepareRows()
	buf := prepared.NewBuffer()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := (i * 50) % (size - 50)
		for row := start; row < start+50; row++ {
			prepared.GetRow(row, buf)
		}
	}
}
