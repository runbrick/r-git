package graph

import (
	"testing"
)

func TestFixDuplicateParentCommits(t *testing.T) {
	nodes := []Node{
		{Id: 1, Parents: []int32{2, 2}},
		{Id: 2, Parents: []int32{3, 4, 3}},
	}
	fixed := fixDuplicateParentCommits(nodes)
	if len(fixed) != 2 {
		t.Fatalf("expected 2 nodes")
	}
	if len(fixed[0].Parents) != 1 || fixed[0].Parents[0] != 2 {
		t.Errorf("expected 1 parent for node 1, got %v", fixed[0].Parents)
	}
	if len(fixed[1].Parents) != 2 || fixed[1].Parents[0] != 3 || fixed[1].Parents[1] != 4 {
		t.Errorf("expected 2 parents for node 2, got %v", fixed[1].Parents)
	}
}

func TestPermanentLinearGraphBuilder(t *testing.T) {
	nodes := []Node{
		{Id: 1, Parents: []int32{2, 3}},
		{Id: 2, Parents: []int32{4}},
		{Id: 3, Parents: []int32{4}},
		{Id: 4, Parents: []int32{5}},
		{Id: 5, Parents: []int32{}},
	}
	graph, err := NewLinearGraph(nodes)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if graph.NodesCount() != 5 {
		t.Errorf("expected 5 nodes, got %v", graph.NodesCount())
	}
	if graph.GetNodeId(0) != 0 {
		t.Errorf("expected id 0 for node 0")
	}
}
