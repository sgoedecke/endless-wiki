package constellation

import (
	"hash/fnv"
	"math/rand"
	"sort"
)

type neighbor struct {
	node   int
	weight float64
}

type louvainGraph struct {
	adjacency   [][]neighbor
	degree      []float64
	loops       []float64
	totalWeight float64
}

type louvainStatus struct {
	nodeCommunity     []int
	communityDegree   []float64
	communityInternal []float64
	nodeDegree        []float64
	loops             []float64
	totalWeight       float64
}

type neighborAccumulator struct {
	weights map[int]float64
	keys    []int
}

var rng = rand.New(rand.NewSource(42))

const targetClusterCount = 20

var resolutionCandidates = []float64{3.4, 2.6, 2.0, 1.6, 1.3, 1.0}

func computeLouvainClusters(slugs []string, edges []Edge) map[string]int {
	if len(slugs) == 0 {
		return map[string]int{}
	}

	indexBySlug := make(map[string]int, len(slugs))
	for i, slug := range slugs {
		indexBySlug[slug] = i
	}

	graph := buildGraph(len(slugs), indexBySlug, edges)

	desiredClusters := targetClusterCount
	if len(slugs) < desiredClusters {
		desiredClusters = len(slugs) / 3
		if desiredClusters < 2 {
			desiredClusters = 2
		}
		if desiredClusters > len(slugs) {
			desiredClusters = len(slugs)
		}
	}

	resolutions := resolutionCandidates
	if len(slugs) < 500 {
		resolutions = []float64{1.0, 0.8, 0.6}
	}

	var partition []int
	unique := 0
	for i, resolution := range resolutions {
		partition = louvain(graph, resolution)
		if len(partition) == 0 {
			break
		}
		unique = countUnique(partition)
		if unique >= desiredClusters || i == len(resolutions)-1 {
			break
		}
	}

	if len(partition) > 0 && unique < desiredClusters {
		resolution := resolutions[len(resolutions)-1]
		for attempts := 0; attempts < 4 && unique < desiredClusters; attempts++ {
			resolution *= 1.5
			partition = louvain(graph, resolution)
			if len(partition) == 0 {
				break
			}
			unique = countUnique(partition)
		}
	}

	if len(partition) == 0 || unique < desiredClusters {
		return fallbackClusters(indexBySlug, desiredClusters)
	}

	result := make(map[string]int, len(slugs))
	for slug, idx := range indexBySlug {
		result[slug] = partition[idx]
	}
	return result
}

func buildGraph(nodeCount int, indexBySlug map[string]int, edges []Edge) louvainGraph {
	adjTemp := make([]map[int]float64, nodeCount)
	degree := make([]float64, nodeCount)
	loops := make([]float64, nodeCount)
	var totalWeight float64

	for _, edge := range edges {
		src, okSrc := indexBySlug[edge.Source]
		dst, okDst := indexBySlug[edge.Target]
		if !okSrc || !okDst {
			continue
		}
		if src == dst {
			loops[src]++
			totalWeight++
			continue
		}

		if adjTemp[src] == nil {
			adjTemp[src] = make(map[int]float64)
		}
		if adjTemp[dst] == nil {
			adjTemp[dst] = make(map[int]float64)
		}

		adjTemp[src][dst]++
		adjTemp[dst][src]++
		degree[src]++
		degree[dst]++
		totalWeight++
	}

	adjacency := make([][]neighbor, nodeCount)
	for node, neighbors := range adjTemp {
		if len(neighbors) == 0 {
			continue
		}
		adjList := make([]neighbor, 0, len(neighbors))
		for target, weight := range neighbors {
			adjList = append(adjList, neighbor{node: target, weight: weight})
		}
		adjacency[node] = adjList
	}

	return louvainGraph{
		adjacency:   adjacency,
		degree:      degree,
		loops:       loops,
		totalWeight: totalWeight,
	}
}

func louvain(graph louvainGraph, resolution float64) []int {
	if len(graph.adjacency) == 0 {
		return []int{}
	}

	status := initStatus(graph)
	dendrogram := make([][]int, 0, 4)
	acc := newNeighborAccumulator()

	for {
		moved := oneLevel(graph, status, acc, resolution)
		partition := renumber(status.nodeCommunity)
		dendrogram = append(dendrogram, partition)
		if !moved {
			break
		}
		graph = inducedGraph(partition, graph)
		status = initStatus(graph)
	}

	final := partitionAtLevel(dendrogram, len(dendrogram)-1)
	return renumber(final)
}

func initStatus(graph louvainGraph) *louvainStatus {
	n := len(graph.adjacency)
	nodeCommunity := make([]int, n)
	communityDegree := make([]float64, n)
	communityInternal := make([]float64, n)

	for i := 0; i < n; i++ {
		nodeCommunity[i] = i
		communityDegree[i] = graph.degree[i]
		communityInternal[i] = graph.loops[i]
	}

	return &louvainStatus{
		nodeCommunity:     nodeCommunity,
		communityDegree:   communityDegree,
		communityInternal: communityInternal,
		nodeDegree:        graph.degree,
		loops:             graph.loops,
		totalWeight:       graph.totalWeight,
	}
}

func oneLevel(graph louvainGraph, status *louvainStatus, acc *neighborAccumulator, resolution float64) bool {
	n := len(graph.adjacency)
	if n == 0 {
		return false
	}

	nodes := make([]int, n)
	for i := 0; i < n; i++ {
		nodes[i] = i
	}
	rng.Shuffle(n, func(i, j int) {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	})

	movedAny := false
	improved := true

	for improved {
		improved = false
		for _, node := range nodes {
			currentCommunity := status.nodeCommunity[node]
			nodeDegree := status.nodeDegree[node]
			neighborWeights := neighborCommunities(node, graph, status, acc)
			weightInCurrent := neighborWeights.get(currentCommunity)
			status.remove(node, currentCommunity, weightInCurrent)

			bestCommunity := currentCommunity
			bestIncrease := 0.0
			m2 := 2 * status.totalWeight

			if nodeDegree == 0 || m2 == 0 {
				// isolated node stays put
			} else {
				for _, community := range neighborWeights.keys {
					weight := neighborWeights.weights[community]
					increase := weight - (resolution*status.communityDegree[community]*nodeDegree)/m2
					if increase > bestIncrease {
						bestIncrease = increase
						bestCommunity = community
					}
				}
			}

			status.insert(node, bestCommunity, neighborWeights.get(bestCommunity))
			if bestCommunity != currentCommunity {
				improved = true
				movedAny = true
			}
		}
	}

	return movedAny
}

func neighborCommunities(node int, graph louvainGraph, status *louvainStatus, acc *neighborAccumulator) *neighborAccumulator {
	acc.reset()
	for _, nb := range graph.adjacency[node] {
		if nb.node == node {
			continue
		}
		community := status.nodeCommunity[nb.node]
		if community == -1 {
			continue
		}
		acc.add(community, nb.weight)
	}
	return acc
}

func (s *louvainStatus) remove(node, community int, weightInCommunity float64) {
	s.communityDegree[community] -= s.nodeDegree[node]
	s.communityInternal[community] -= 2*weightInCommunity + s.loops[node]
	s.nodeCommunity[node] = -1
}

func (s *louvainStatus) insert(node, community int, weightInCommunity float64) {
	s.nodeCommunity[node] = community
	s.communityDegree[community] += s.nodeDegree[node]
	s.communityInternal[community] += 2*weightInCommunity + s.loops[node]
}

func renumber(partition []int) []int {
	mapping := make(map[int]int, len(partition))
	result := make([]int, len(partition))
	next := 0
	for idx, community := range partition {
		id, ok := mapping[community]
		if !ok {
			id = next
			mapping[community] = id
			next++
		}
		result[idx] = id
	}
	return result
}

func inducedGraph(partition []int, graph louvainGraph) louvainGraph {
	if len(partition) == 0 {
		return louvainGraph{}
	}

	numCommunities := 0
	for _, community := range partition {
		if community+1 > numCommunities {
			numCommunities = community + 1
		}
	}

	adjTemp := make([]map[int]float64, numCommunities)
	degree := make([]float64, numCommunities)
	loops := make([]float64, numCommunities)
	var totalWeight float64

	for node, neighbors := range graph.adjacency {
		commA := partition[node]
		for _, nb := range neighbors {
			if nb.node < node {
				continue
			}
			commB := partition[nb.node]
			weight := nb.weight

			if commA == commB {
				loops[commA] += weight
				degree[commA] += 2 * weight
			} else {
				if adjTemp[commA] == nil {
					adjTemp[commA] = make(map[int]float64)
				}
				if adjTemp[commB] == nil {
					adjTemp[commB] = make(map[int]float64)
				}
				adjTemp[commA][commB] += weight
				adjTemp[commB][commA] += weight
				degree[commA] += weight
				degree[commB] += weight
			}
			totalWeight += weight
		}
	}

	adjacency := make([][]neighbor, numCommunities)
	for community, neighbors := range adjTemp {
		if len(neighbors) == 0 {
			continue
		}
		adjList := make([]neighbor, 0, len(neighbors))
		for target, weight := range neighbors {
			adjList = append(adjList, neighbor{node: target, weight: weight})
		}
		adjacency[community] = adjList
	}

	return louvainGraph{
		adjacency:   adjacency,
		degree:      degree,
		loops:       loops,
		totalWeight: totalWeight,
	}
}

func partitionAtLevel(dendrogram [][]int, level int) []int {
	if len(dendrogram) == 0 || level < 0 {
		return []int{}
	}
	if level >= len(dendrogram) {
		level = len(dendrogram) - 1
	}

	result := append([]int(nil), dendrogram[0]...)
	for i := 1; i <= level; i++ {
		next := dendrogram[i]
		for idx := range result {
			result[idx] = next[result[idx]]
		}
	}
	return result
}

func newNeighborAccumulator() *neighborAccumulator {
	return &neighborAccumulator{
		weights: make(map[int]float64, 8),
		keys:    make([]int, 0, 8),
	}
}

func (acc *neighborAccumulator) reset() {
	for _, key := range acc.keys {
		delete(acc.weights, key)
	}
	acc.keys = acc.keys[:0]
}

func (acc *neighborAccumulator) add(key int, weight float64) {
	if _, ok := acc.weights[key]; !ok {
		acc.weights[key] = weight
		acc.keys = append(acc.keys, key)
		return
	}
	acc.weights[key] += weight
}

func (acc *neighborAccumulator) get(key int) float64 {
	if v, ok := acc.weights[key]; ok {
		return v
	}
	return 0
}

func countUnique(values []int) int {
	if len(values) == 0 {
		return 0
	}
	seen := make(map[int]struct{}, len(values))
	for _, v := range values {
		seen[v] = struct{}{}
	}
	return len(seen)
}

func fallbackClusters(indexBySlug map[string]int, desired int) map[string]int {
	size := len(indexBySlug)
	result := make(map[string]int, size)
	if size == 0 {
		return result
	}

	clusterCount := size / 5000
	if clusterCount < desired {
		clusterCount = desired
	}
	if clusterCount > 64 {
		clusterCount = 64
	}
	if clusterCount > size {
		clusterCount = size
	}
	if clusterCount < 2 {
		clusterCount = 2
	}

	slugs := make([]string, 0, size)
	for slug := range indexBySlug {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	if clusterCount == size {
		for idx, slug := range slugs {
			result[slug] = idx
		}
		return result
	}

	hasher := fnv.New32a()
	for _, slug := range slugs {
		hasher.Reset()
		_, _ = hasher.Write([]byte(slug))
		bucket := int(hasher.Sum32() % uint32(clusterCount))
		result[slug] = bucket
	}

	// Normalize cluster ids to keep them dense starting from zero.
	return renumberMap(result)
}

func renumberMap(clusters map[string]int) map[string]int {
	mapping := make(map[int]int, len(clusters))
	next := 0
	for slug, cluster := range clusters {
		id, ok := mapping[cluster]
		if !ok {
			id = next
			mapping[cluster] = id
			next++
		}
		clusters[slug] = id
	}
	return clusters
}
