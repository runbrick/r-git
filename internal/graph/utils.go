package graph

func GetUpNodes(graph LinearGraph, nodeIndex int) []int {
	var filter EdgeFilter = NormalUp
	var buf [8]GraphEdge // stack allocate for typical case
	edges := graph.AppendAdjacentEdges(buf[:0], nodeIndex, filter)
	nodes := make([]int, 0, len(edges))
	for i := range edges {
		nodes = append(nodes, int(edges[i].UpNodeIndex))
	}
	return nodes
}

func GetDownNodes(graph LinearGraph, nodeIndex int) []int {
	var filter EdgeFilter = NormalDown
	var buf [8]GraphEdge // stack allocate for typical case
	edges := graph.AppendAdjacentEdges(buf[:0], nodeIndex, filter)
	nodes := make([]int, 0, len(edges))
	for i := range edges {
		nodes = append(nodes, int(edges[i].DownNodeIndex))
	}
	return nodes
}
