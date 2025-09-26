package constellation

import "testing"

func TestComputeLouvainClusters_SimpleCommunities(t *testing.T) {
	slugs := []string{"a", "b", "c", "d", "e", "f"}
	edges := []Edge{
		{Source: "a", Target: "b"},
		{Source: "b", Target: "c"},
		{Source: "c", Target: "a"},
		{Source: "d", Target: "e"},
		{Source: "e", Target: "f"},
		{Source: "f", Target: "d"},
	}

	clusters := computeLouvainClusters(slugs, edges)

	setAB := clusters["a"]
	if clusters["b"] != setAB || clusters["c"] != setAB {
		t.Fatalf("expected a/b/c to share cluster, got %v", clusters)
	}

	setDE := clusters["d"]
	if clusters["e"] != setDE || clusters["f"] != setDE {
		t.Fatalf("expected d/e/f to share cluster, got %v", clusters)
	}

	if setAB == setDE {
		t.Fatalf("expected distinct clusters for clique groups, got %v", clusters)
	}
}

func TestComputeLouvainClusters_IsolatedNodes(t *testing.T) {
	slugs := []string{"solo", "alone"}
	clusters := computeLouvainClusters(slugs, nil)

	if len(clusters) != len(slugs) {
		t.Fatalf("expected %d clusters, got %d", len(slugs), len(clusters))
	}

	if clusters["solo"] == clusters["alone"] {
		t.Fatalf("expected isolates to form separate clusters, got %v", clusters)
	}
}
