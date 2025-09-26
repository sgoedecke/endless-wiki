package constellation

import (
	"math"
	"math/rand"
	"sort"
)

type louvainPair struct {
	a int
	b int
}

type louvainGraph struct {
	nodes       []int
	edges       map[louvainPair]float64
	adjacency   map[int]map[int]float64
	degree      map[int]float64
	loops       map[int]float64
	totalWeight float64
}

type louvainStatus struct {
	nodeCommunity     map[int]int
	communityDegree   map[int]float64
	communityInternal map[int]float64
	nodeDegree        map[int]float64
	loops             map[int]float64
	totalWeight       float64
}

func computeLouvainClusters(slugs []string, edges []Edge) map[string]int {
	if len(slugs) == 0 {
		return map[string]int{}
	}

	indexBySlug := make(map[string]int, len(slugs))
	nodes := make([]int, len(slugs))
	for i, slug := range slugs {
		indexBySlug[slug] = i
		nodes[i] = i
	}

	edgeMap := make(map[louvainPair]float64)
	for _, e := range edges {
		src, okSrc := indexBySlug[e.Source]
		dst, okDst := indexBySlug[e.Target]
		if !okSrc || !okDst {
			continue
		}
		if src == dst {
			continue
		}
		a, b := src, dst
		if a > b {
			a, b = b, a
		}
		key := louvainPair{a: a, b: b}
		edgeMap[key] += 1
	}

	graph := buildLouvainGraph(nodes, edgeMap)
	clusters := louvain(graph)

	result := make(map[string]int, len(slugs))
	for slug, idx := range indexBySlug {
		result[slug] = clusters[idx]
	}
	return result
}

func buildLouvainGraph(nodes []int, edges map[louvainPair]float64) louvainGraph {
	adjacency := make(map[int]map[int]float64, len(nodes))
	degree := make(map[int]float64, len(nodes))
	loops := make(map[int]float64)
	totalWeight := 0.0

	for _, node := range nodes {
		adjacency[node] = make(map[int]float64)
		degree[node] = 0
	}

	for edge, weight := range edges {
		totalWeight += weight
		if edge.a == edge.b {
			adjacency[edge.a][edge.a] += weight
			loops[edge.a] += weight
			degree[edge.a] += 2 * weight
			continue
		}
		adjacency[edge.a][edge.b] += weight
		adjacency[edge.b][edge.a] += weight
		degree[edge.a] += weight
		degree[edge.b] += weight
	}

	return louvainGraph{
		nodes:       append([]int(nil), nodes...),
		edges:       edges,
		adjacency:   adjacency,
		degree:      degree,
		loops:       loops,
		totalWeight: totalWeight,
	}
}

func louvain(graph louvainGraph) map[int]int {
	if len(graph.nodes) == 0 {
		return map[int]int{}
	}

	status := initStatus(graph)
	dendrogram := make([]map[int]int, 0)

	for {
		moved := oneLevel(graph, status)
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
	nodeCommunity := make(map[int]int, len(graph.nodes))
	communityDegree := make(map[int]float64, len(graph.nodes))
	communityInternal := make(map[int]float64, len(graph.nodes))

	for _, node := range graph.nodes {
		nodeCommunity[node] = node
		communityDegree[node] = graph.degree[node]
		communityInternal[node] = graph.loops[node]
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

func oneLevel(graph louvainGraph, status *louvainStatus) bool {
	if len(graph.nodes) == 0 {
		return false
	}
	nodes := append([]int(nil), graph.nodes...)
	shuffle(nodes)

	movedAny := false
	improved := true

	for improved {
		improved = false
		for _, node := range nodes {
			currentCommunity := status.nodeCommunity[node]
			nodeDegree := status.nodeDegree[node]
			neighComWeights := neighborCommunities(node, graph, status)
			weightInCurrent := neighComWeights[currentCommunity]
			status.remove(node, currentCommunity, weightInCurrent)

			bestCommunity := currentCommunity
			bestIncrease := 0.0
			m2 := 2 * status.totalWeight

			if nodeDegree == 0 || m2 == 0 {
				// Isolated node, stay in place
			} else {
				for community, weight := range neighComWeights {
					increase := weight - (status.communityDegree[community]*nodeDegree)/m2
					if increase > bestIncrease {
						bestIncrease = increase
						bestCommunity = community
					}
				}
			}

			status.insert(node, bestCommunity, neighComWeights[bestCommunity])
			if bestCommunity != currentCommunity {
				improved = true
				movedAny = true
			}
		}
	}

	return movedAny
}

func neighborCommunities(node int, graph louvainGraph, status *louvainStatus) map[int]float64 {
	result := make(map[int]float64)
	for neighbor, weight := range graph.adjacency[node] {
		if neighbor == node {
			continue
		}
		community := status.nodeCommunity[neighbor]
		result[community] += weight
	}
	return result
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

func renumber(partition map[int]int) map[int]int {
	mapping := make(map[int]int)
	next := 0
	result := make(map[int]int, len(partition))
	keys := make([]int, 0, len(partition))
	for node := range partition {
		keys = append(keys, node)
	}
	sort.Ints(keys)
	for _, node := range keys {
		community := partition[node]
		id, ok := mapping[community]
		if !ok {
			id = next
			mapping[community] = id
			next++
		}
		result[node] = id
	}
	return result
}

func inducedGraph(partition map[int]int, graph louvainGraph) louvainGraph {
	numCommunities := 0
	for _, community := range partition {
		if community+1 > numCommunities {
			numCommunities = community + 1
		}
	}
	nodes := make([]int, numCommunities)
	for i := 0; i < numCommunities; i++ {
		nodes[i] = i
	}

	newEdges := make(map[louvainPair]float64)
	for edge, weight := range graph.edges {
		communityA := partition[edge.a]
		communityB := partition[edge.b]
		if communityA > communityB {
			communityA, communityB = communityB, communityA
		}
		key := louvainPair{a: communityA, b: communityB}
		newEdges[key] += weight
	}

	return buildLouvainGraph(nodes, newEdges)
}

func partitionAtLevel(dendrogram []map[int]int, level int) map[int]int {
	if len(dendrogram) == 0 || level < 0 {
		return map[int]int{}
	}
	level = int(math.Min(float64(level), float64(len(dendrogram)-1)))

	result := make(map[int]int)
	for node, community := range dendrogram[0] {
		result[node] = community
	}

	for i := 1; i <= level; i++ {
		next := dendrogram[i]
		for node := range result {
			result[node] = next[result[node]]
		}
	}
	return result
}

func shuffle(values []int) {
	rng := rand.New(rand.NewSource(42))
	for i := len(values) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		values[i], values[j] = values[j], values[i]
	}
}
