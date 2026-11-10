package graph

func walk(startNode int, step func(currentNode int) int) {
	stack := []int{startNode}
	for len(stack) > 0 {
		currentNode := stack[len(stack)-1]
		nextNode := step(currentNode)
		if nextNode == -1 {
			stack = stack[:len(stack)-1]
		} else {
			stack = append(stack, nextNode)
		}
	}
}
