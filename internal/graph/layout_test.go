package graph_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runbrick/r-git/internal/graph"
	"github.com/runbrick/r-git/internal/graph/parser"
)

func TestGraphLayoutBuilder(t *testing.T) {
	testcases := []string{
		"headsOrder",
		"manyNodes",
		"notFullGraph",
		"oneNode",
		"oneNodeNotFullGraph",
	}

	for _, tc := range testcases {
		t.Run(tc, func(t *testing.T) {
			inPath := filepath.Join("testdata", "layoutBuilder", tc+"_in.txt")
			outPath := filepath.Join("testdata", "layoutBuilder", tc+"_out.txt")

			inBytes, err := os.ReadFile(inPath)
			if err != nil {
				t.Fatalf("failed to read %s: %v", inPath, err)
			}
			outBytes, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatalf("failed to read %s: %v", outPath, err)
			}

			inStr := strings.TrimSpace(string(inBytes))
			outStr := strings.TrimSpace(string(outBytes))

			// Parse nodes
			nodes, err := parser.ParseNodes(inStr)
			if err != nil {
				t.Fatalf("failed to parse nodes: %v", err)
			}

			lg, err := graph.NewLinearGraph(nodes)
			if err != nil {
				t.Fatalf("failed to create linear graph: %v", err)
			}

			layout := graph.BuildLayout(lg, func(id1, id2 int32) int {
				s1 := fmt.Sprintf("%x", id1)
				s2 := fmt.Sprintf("%x", id2)
				if tc == "headsOrder" {
					fmt.Printf("id1=%d(hex=%s) id2=%d(hex=%s) cmp=%d\n", id1, s1, id2, s2, strings.Compare(s1, s2))
				}
				return strings.Compare(s1, s2)
			})

			// Compare output
			var sb strings.Builder
			for i := 0; i < lg.NodesCount(); i++ {
				if i > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(fmt.Sprintf("%d|-%d", layout.GetLayoutIndex(i), layout.GetOneOfHeadNodeIndex(i)))
			}
			gotStr := sb.String()

			if gotStr != outStr {
				t.Errorf("mismatch for %s.\nExpected:\n%s\nGot:\n%s", tc, outStr, gotStr)
			}
		})
	}
}
