package runner

// Plan builder: flattens the collection tree the UI/store produces into an
// ordered list of run steps.

import (
	"strings"

	"github.com/friesipayung/rapi/internal/model"
)

// PlanFromTree walks the tree nodes DFS in tree order and collects only
// HTTP request nodes as steps. When filterRel is non-empty, only nodes
// whose ID equals filterRel or lies under filterRel+"/" are collected
// (run a whole collection, one folder, or a single request). Ancestors of
// filterRel are still traversed so their matching descendants are reached.
func PlanFromTree(nodes []*model.TreeNode, filterRel string) []Step {
	var steps []Step
	var walk func(n *model.TreeNode)
	walk = func(n *model.TreeNode) {
		if n == nil {
			return
		}
		if filterRel != "" &&
			n.ID != filterRel &&
			!strings.HasPrefix(n.ID, filterRel+"/") && // inside the filtered subtree
			!strings.HasPrefix(filterRel, n.ID+"/") { // ancestor of the filtered node
			return
		}
		if n.Type == model.NodeHTTP &&
			(filterRel == "" || n.ID == filterRel || strings.HasPrefix(n.ID, filterRel+"/")) {
			steps = append(steps, Step{RequestID: n.ID, Name: n.Name})
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return steps
}
