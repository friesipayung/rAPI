package runner

import (
	"testing"

	"github.com/friesipayung/rapi/internal/model"
)

func TestPlanFromTree(t *testing.T) {
	// 3 requests across 2 folders, plus non-request nodes that must be
	// skipped by the plan builder.
	tree := []*model.TreeNode{
		{
			ID: "col", Type: model.NodeCollection, Name: "Col",
			Children: []*model.TreeNode{
				{
					ID: "col/folder-a", Type: model.NodeFolder, Name: "Folder A",
					Children: []*model.TreeNode{
						{ID: "col/folder-a/get-users", Type: model.NodeHTTP, Name: "Get Users"},
						{ID: "col/folder-a/create-post", Type: model.NodeHTTP, Name: "Create Post"},
					},
				},
				{
					ID: "col/folder-b", Type: model.NodeFolder, Name: "Folder B",
					Children: []*model.TreeNode{
						{ID: "col/folder-b/delete-post", Type: model.NodeHTTP, Name: "Delete Post"},
					},
				},
			},
		},
	}

	t.Run("full plan in DFS tree order", func(t *testing.T) {
		steps := PlanFromTree(tree, "")
		want := []Step{
			{RequestID: "col/folder-a/get-users", Name: "Get Users"},
			{RequestID: "col/folder-a/create-post", Name: "Create Post"},
			{RequestID: "col/folder-b/delete-post", Name: "Delete Post"},
		}
		if len(steps) != len(want) {
			t.Fatalf("len(steps) = %d, want %d: %+v", len(steps), len(want), steps)
		}
		for i := range want {
			if steps[i] != want[i] {
				t.Errorf("steps[%d] = %+v, want %+v", i, steps[i], want[i])
			}
		}
	})

	t.Run("filter to one folder", func(t *testing.T) {
		steps := PlanFromTree(tree, "col/folder-a")
		want := []string{"col/folder-a/get-users", "col/folder-a/create-post"}
		if len(steps) != 2 {
			t.Fatalf("len(steps) = %d, want 2: %+v", len(steps), steps)
		}
		for i, id := range want {
			if steps[i].RequestID != id {
				t.Errorf("steps[%d] = %s, want %s", i, steps[i].RequestID, id)
			}
		}
	})

	t.Run("filter to a single request", func(t *testing.T) {
		steps := PlanFromTree(tree, "col/folder-b/delete-post")
		if len(steps) != 1 || steps[0].RequestID != "col/folder-b/delete-post" || steps[0].Name != "Delete Post" {
			t.Fatalf("got %+v, want the single Delete Post step", steps)
		}
	})

	t.Run("filter with no matches", func(t *testing.T) {
		if steps := PlanFromTree(tree, "col/missing"); len(steps) != 0 {
			t.Fatalf("got %+v, want no steps", steps)
		}
	})

	t.Run("nil nodes are ignored", func(t *testing.T) {
		if steps := PlanFromTree([]*model.TreeNode{nil}, ""); len(steps) != 0 {
			t.Fatalf("got %+v, want no steps", steps)
		}
	})
}
