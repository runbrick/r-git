package graph_test

import (
	"testing"

	"github.com/runbrick/r-git/internal/graph"
)

func benchmarkHistory(size int) []graph.Node {
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
	return nodes
}

func BenchmarkBuild100kLinearMergeHistory(b *testing.B) {
	nodes := benchmarkHistory(100_000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lg, err := graph.NewLinearGraph(nodes)
		if err != nil {
			b.Fatal(err)
		}
		_ = graph.BuildLayout(lg, func(a, c int32) int { return int(a - c) })
	}
}
