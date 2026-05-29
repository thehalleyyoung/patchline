package provenance

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thehalleyyoung/patchline/internal/canonical"
)

const (
	CauseReportVersion        = "patchline.provenance-cause/v1"
	MinimalExplanationVersion = "patchline.minimal-explanation/v1"
	DiffReportVersion         = "patchline.provenance-diff/v1"
	ArchiveReportVersion      = "patchline.incident-archive/v1"
	CausalCertificateVersion  = "patchline.causal-certificate/v1"
)

type EvidenceValue string

const (
	EvidenceValueAbsent      EvidenceValue = "absent"
	EvidenceValueWeak        EvidenceValue = "weak"
	EvidenceValueStrong      EvidenceValue = "strong"
	EvidenceValueExact       EvidenceValue = "exact"
	EvidenceValueConflicting EvidenceValue = "conflicting"
	EvidenceValueRedacted    EvidenceValue = "redacted"
)

type EntitySummary struct {
	ID         string            `json:"id"`
	Kind       EntityKind        `json:"kind"`
	Name       string            `json:"name,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type EdgeSummary struct {
	From     string        `json:"from"`
	To       string        `json:"to"`
	Kind     EdgeKind      `json:"kind"`
	Evidence EvidenceLevel `json:"evidence"`
}

type CauseQueryOptions struct {
	Starts      []string      `json:"starts"`
	MinEvidence EvidenceLevel `json:"min_evidence"`
	CauseKinds  []EntityKind  `json:"cause_kinds"`
	MaxDepth    int           `json:"max_depth"`
	StopKinds   []EntityKind  `json:"stop_kinds,omitempty"`
	Allowed     []EdgeKind    `json:"allowed_edges,omitempty"`
	Observes    []EntityKind  `json:"observed_kinds,omitempty"`
}

type CauseReport struct {
	Version              string                  `json:"version"`
	Options              CauseQueryOptions       `json:"options"`
	MinimalCauses        []EntitySummary         `json:"minimal_causes"`
	AllCauseCandidates   []EntitySummary         `json:"all_cause_candidates"`
	CommonAncestors      []EntitySummary         `json:"common_ancestors"`
	AffectedObservations []EntitySummary         `json:"affected_observations"`
	RepairLineage        []EdgeSummary           `json:"repair_lineage"`
	Semiring             EvidenceSemiringSummary `json:"semiring"`
	MinimalExplanation   MinimalExplanation      `json:"minimal_explanation"`
	BlastRadius          BlastRadius             `json:"blast_radius"`
	ReportHash           string                  `json:"report_hash"`
}

type EvidenceSemiringSummary struct {
	Definition   string             `json:"definition"`
	PathCombine  string             `json:"path_combine"`
	FactJoin     string             `json:"fact_join"`
	Counts       map[string]int     `json:"counts"`
	Conflicts    []EvidenceConflict `json:"conflicts,omitempty"`
	WeakestValue EvidenceValue      `json:"weakest_value"`
}

type EvidenceConflict struct {
	Fact     string          `json:"fact"`
	Values   []EvidenceLevel `json:"values"`
	EdgeKeys []string        `json:"edge_keys"`
}

type MissingEvidence struct {
	Entity string `json:"entity"`
	Rule   string `json:"rule"`
	Need   string `json:"need"`
}

type MinimalExplanation struct {
	Version     string            `json:"version"`
	Start       []string          `json:"start"`
	Causes      []EntitySummary   `json:"causes"`
	Entities    []EntitySummary   `json:"entities"`
	Edges       []EdgeSummary     `json:"edges"`
	Missing     []MissingEvidence `json:"missing_evidence,omitempty"`
	EntityHash  string            `json:"entity_hash"`
	EdgeHash    string            `json:"edge_hash"`
	Explanation string            `json:"definition"`
	Hash        string            `json:"hash"`
}

type BlastRadius struct {
	Causes       []EntitySummary `json:"causes"`
	EntityCounts []KindCount     `json:"entity_counts"`
	Tables       []Count         `json:"tables,omitempty"`
	Records      []EntitySummary `json:"records,omitempty"`
	Reports      []EntitySummary `json:"reports,omitempty"`
	Services     []EntitySummary `json:"services,omitempty"`
	Hash         string          `json:"hash"`
}

type KindCount struct {
	Kind  EntityKind `json:"kind"`
	Count int        `json:"count"`
}

type Count struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type DiffReport struct {
	Version      string   `json:"version"`
	Equivalent   bool     `json:"equivalent"`
	LeftHash     string   `json:"left_hash"`
	RightHash    string   `json:"right_hash"`
	SharedShapes []string `json:"shared_shapes,omitempty"`
	LeftOnly     []string `json:"left_only,omitempty"`
	RightOnly    []string `json:"right_only,omitempty"`
	ReportHash   string   `json:"report_hash"`
}

type IncidentArchiveReport struct {
	Version   string         `json:"version"`
	Incidents []IncidentItem `json:"incidents"`
	Buckets   []ShapeBucket  `json:"buckets"`
	Hash      string         `json:"hash"`
}

type IncidentItem struct {
	Path      string `json:"path"`
	ShapeHash string `json:"shape_hash"`
}

type ShapeBucket struct {
	ShapeHash string   `json:"shape_hash"`
	Count     int      `json:"count"`
	Incidents []string `json:"incidents"`
}

type CausalCertificate struct {
	Version     string                  `json:"version"`
	Start       []string                `json:"start"`
	Claims      []string                `json:"claims"`
	Explanation MinimalExplanation      `json:"explanation"`
	Semiring    EvidenceSemiringSummary `json:"semiring"`
	BlastRadius BlastRadius             `json:"blast_radius"`
	Missing     []MissingEvidence       `json:"missing_evidence,omitempty"`
	Certificate string                  `json:"certificate"`
	Hash        string                  `json:"hash"`
}

func DefaultCauseOptions(starts []string) CauseQueryOptions {
	return CauseQueryOptions{
		Starts:      append([]string(nil), starts...),
		MinEvidence: EvidenceStrong,
		CauseKinds:  []EntityKind{KindSQLMutation, KindMigration, KindDeploy, KindCommit},
		MaxDepth:    12,
		Allowed:     []EdgeKind{EdgeDeployedCommit, EdgeExecuted, EdgeCaused, EdgeMutated, EdgeDerivedInto, EdgeObserved, EdgeRepaired},
		Observes:    []EntityKind{KindRecord, KindReport, KindTrace, KindService},
	}
}

func (g *Graph) CauseReport(opts CauseQueryOptions) (CauseReport, error) {
	opts = normalizeCauseOptions(opts)
	ancestorsByStart := map[string]map[string]bool{}
	allCandidates := map[string]Entity{}
	for _, start := range opts.Starts {
		if _, ok := g.Entity(start); !ok {
			return CauseReport{}, fmt.Errorf("start entity %s does not exist", start)
		}
		ancestors := g.ancestors(start, opts)
		ancestorsByStart[start] = ancestors
		for id := range ancestors {
			entity, _ := g.Entity(id)
			if containsKind(opts.CauseKinds, entity.Kind) {
				allCandidates[id] = entity
			}
		}
	}
	minimalCauses := g.minimalCauseSet(allCandidates, opts)
	report := CauseReport{
		Version:              CauseReportVersion,
		Options:              opts,
		MinimalCauses:        summarizeEntitiesMap(minimalCauses),
		AllCauseCandidates:   summarizeEntitiesMap(allCandidates),
		CommonAncestors:      g.commonAncestors(ancestorsByStart),
		AffectedObservations: g.affectedObservations(minimalCauses, opts),
		RepairLineage:        g.repairLineage(opts.MinEvidence),
		Semiring:             g.EvidenceSemiring(opts.MinEvidence),
	}
	explanation, err := g.MinimalExplanation(opts)
	if err != nil {
		return CauseReport{}, err
	}
	report.MinimalExplanation = explanation
	report.BlastRadius = g.BlastRadius(minimalCauses, opts)
	report.ReportHash = canonical.Hash(struct {
		Version              string                  `json:"version"`
		Options              CauseQueryOptions       `json:"options"`
		MinimalCauses        []EntitySummary         `json:"minimal_causes"`
		CommonAncestors      []EntitySummary         `json:"common_ancestors"`
		AffectedObservations []EntitySummary         `json:"affected_observations"`
		RepairLineage        []EdgeSummary           `json:"repair_lineage"`
		Semiring             EvidenceSemiringSummary `json:"semiring"`
		ExplanationHash      string                  `json:"explanation_hash"`
		BlastHash            string                  `json:"blast_hash"`
	}{
		Version:              report.Version,
		Options:              report.Options,
		MinimalCauses:        report.MinimalCauses,
		CommonAncestors:      report.CommonAncestors,
		AffectedObservations: report.AffectedObservations,
		RepairLineage:        report.RepairLineage,
		Semiring:             report.Semiring,
		ExplanationHash:      report.MinimalExplanation.Hash,
		BlastHash:            report.BlastRadius.Hash,
	})
	return report, nil
}

func (g *Graph) MinimalExplanation(opts CauseQueryOptions) (MinimalExplanation, error) {
	opts = normalizeCauseOptions(opts)
	causeReportAncestors := map[string]Entity{}
	for _, start := range opts.Starts {
		if _, ok := g.Entity(start); !ok {
			return MinimalExplanation{}, fmt.Errorf("start entity %s does not exist", start)
		}
		for id := range g.ancestors(start, opts) {
			entity, _ := g.Entity(id)
			if containsKind(opts.CauseKinds, entity.Kind) {
				causeReportAncestors[id] = entity
			}
		}
	}
	causes := g.minimalCauseSet(causeReportAncestors, opts)
	entities := map[string]Entity{}
	edges := map[string]Edge{}
	for _, start := range opts.Starts {
		entity, _ := g.Entity(start)
		entities[start] = entity
		for cause := range causes {
			pathEdges, ok := g.shortestBackwardPath(start, cause, opts)
			if !ok {
				continue
			}
			for _, edge := range pathEdges {
				edges[edgeKey(edge)] = edge
				if from, ok := g.Entity(edge.From); ok {
					entities[edge.From] = from
				}
				if to, ok := g.Entity(edge.To); ok {
					entities[edge.To] = to
				}
			}
		}
	}
	out := MinimalExplanation{
		Version:     MinimalExplanationVersion,
		Start:       append([]string(nil), opts.Starts...),
		Causes:      summarizeEntitiesMap(causes),
		Entities:    summarizeEntitiesMap(entities),
		Edges:       summarizeEdgesMap(edges),
		Missing:     missingEvidence(sortedEntityMap(entities), sortedEdgeMap(edges)),
		Explanation: "smallest deterministic union of shortest evidence paths from each start to the closest cause candidates",
	}
	sort.Strings(out.Start)
	out.EntityHash = canonical.Hash(out.Entities)
	out.EdgeHash = canonical.Hash(out.Edges)
	out.Hash = canonical.Hash(struct {
		Version     string            `json:"version"`
		Start       []string          `json:"start"`
		Causes      []EntitySummary   `json:"causes"`
		EntityHash  string            `json:"entity_hash"`
		EdgeHash    string            `json:"edge_hash"`
		Missing     []MissingEvidence `json:"missing_evidence,omitempty"`
		Explanation string            `json:"definition"`
	}{
		Version:     out.Version,
		Start:       out.Start,
		Causes:      out.Causes,
		EntityHash:  out.EntityHash,
		EdgeHash:    out.EdgeHash,
		Missing:     out.Missing,
		Explanation: out.Explanation,
	})
	return out, nil
}

func (g *Graph) EvidenceSemiring(min EvidenceLevel) EvidenceSemiringSummary {
	counts := map[string]int{
		string(EvidenceValueAbsent):      0,
		string(EvidenceValueWeak):        0,
		string(EvidenceValueStrong):      0,
		string(EvidenceValueExact):       0,
		string(EvidenceValueConflicting): 0,
		string(EvidenceValueRedacted):    0,
	}
	weakest := EvidenceValueExact
	facts := map[string]map[EvidenceLevel][]string{}
	for _, edge := range g.Edges() {
		value := semiringValue(edge.Evidence)
		counts[string(value)]++
		if evidenceValueRank(value) < evidenceValueRank(weakest) {
			weakest = value
		}
		key := edge.From + "->" + edge.To + ":" + string(edge.Kind)
		if facts[key] == nil {
			facts[key] = map[EvidenceLevel][]string{}
		}
		facts[key][edge.Evidence] = append(facts[key][edge.Evidence], edgeKey(edge))
	}
	var conflicts []EvidenceConflict
	for fact, values := range facts {
		if len(values) < 2 {
			continue
		}
		conflict := EvidenceConflict{Fact: fact}
		for value, edgeKeys := range values {
			conflict.Values = append(conflict.Values, value)
			conflict.EdgeKeys = append(conflict.EdgeKeys, edgeKeys...)
		}
		sort.Slice(conflict.Values, func(i, j int) bool {
			return evidenceRank(conflict.Values[i]) > evidenceRank(conflict.Values[j])
		})
		sort.Strings(conflict.EdgeKeys)
		conflicts = append(conflicts, conflict)
	}
	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].Fact < conflicts[j].Fact
	})
	counts[string(EvidenceValueConflicting)] = len(conflicts)
	if len(g.Edges()) == 0 || !meetsEvidence(levelFromValue(weakest), min) {
		weakest = EvidenceValueAbsent
	}
	return EvidenceSemiringSummary{
		Definition:   "evidence semiring over provenance facts",
		PathCombine:  "times is minimum evidence rank along a path",
		FactJoin:     "plus is strongest supporting evidence unless duplicate fact support disagrees, then conflicting",
		Counts:       counts,
		Conflicts:    conflicts,
		WeakestValue: weakest,
	}
}

func (g *Graph) BlastRadius(causes map[string]Entity, opts CauseQueryOptions) BlastRadius {
	entities := map[string]Entity{}
	for cause := range causes {
		for _, entity := range g.forwardReachable(cause, opts) {
			entities[entity.ID] = entity
		}
	}
	kindCounts := map[EntityKind]int{}
	tableCounts := map[string]int{}
	var records, reports, services []EntitySummary
	for _, entity := range sortedEntityMap(entities) {
		kindCounts[entity.Kind]++
		switch entity.Kind {
		case KindRecord:
			records = append(records, summarizeEntity(entity))
			if table := recordTable(entity.ID); table != "" {
				tableCounts[table]++
			}
		case KindReport:
			reports = append(reports, summarizeEntity(entity))
		case KindService:
			services = append(services, summarizeEntity(entity))
		}
	}
	out := BlastRadius{
		Causes:       summarizeEntitiesMap(causes),
		EntityCounts: sortedKindCounts(kindCounts),
		Tables:       sortedCounts(tableCounts),
		Records:      records,
		Reports:      reports,
		Services:     services,
	}
	out.Hash = canonical.Hash(struct {
		Causes       []EntitySummary `json:"causes"`
		EntityCounts []KindCount     `json:"entity_counts"`
		Tables       []Count         `json:"tables,omitempty"`
		Records      []EntitySummary `json:"records,omitempty"`
		Reports      []EntitySummary `json:"reports,omitempty"`
		Services     []EntitySummary `json:"services,omitempty"`
	}{
		Causes:       out.Causes,
		EntityCounts: out.EntityCounts,
		Tables:       out.Tables,
		Records:      out.Records,
		Reports:      out.Reports,
		Services:     out.Services,
	})
	return out
}

func DiffGraphs(left, right *Graph) DiffReport {
	leftShapes := graphShapes(left)
	rightShapes := graphShapes(right)
	var shared, leftOnly, rightOnly []string
	for shape := range leftShapes {
		if rightShapes[shape] {
			shared = append(shared, shape)
		} else {
			leftOnly = append(leftOnly, shape)
		}
	}
	for shape := range rightShapes {
		if !leftShapes[shape] {
			rightOnly = append(rightOnly, shape)
		}
	}
	sort.Strings(shared)
	sort.Strings(leftOnly)
	sort.Strings(rightOnly)
	report := DiffReport{
		Version:      DiffReportVersion,
		Equivalent:   len(leftOnly) == 0 && len(rightOnly) == 0,
		LeftHash:     canonical.Hash(sortedKeys(leftShapes)),
		RightHash:    canonical.Hash(sortedKeys(rightShapes)),
		SharedShapes: shared,
		LeftOnly:     leftOnly,
		RightOnly:    rightOnly,
	}
	report.ReportHash = canonical.Hash(struct {
		Version    string   `json:"version"`
		Equivalent bool     `json:"equivalent"`
		LeftHash   string   `json:"left_hash"`
		RightHash  string   `json:"right_hash"`
		LeftOnly   []string `json:"left_only,omitempty"`
		RightOnly  []string `json:"right_only,omitempty"`
	}{
		Version:    report.Version,
		Equivalent: report.Equivalent,
		LeftHash:   report.LeftHash,
		RightHash:  report.RightHash,
		LeftOnly:   report.LeftOnly,
		RightOnly:  report.RightOnly,
	})
	return report
}

func IncidentArchive(items []IncidentItem) IncidentArchiveReport {
	bucketMap := map[string][]string{}
	for _, item := range items {
		bucketMap[item.ShapeHash] = append(bucketMap[item.ShapeHash], item.Path)
	}
	var buckets []ShapeBucket
	for hash, incidents := range bucketMap {
		sort.Strings(incidents)
		buckets = append(buckets, ShapeBucket{ShapeHash: hash, Count: len(incidents), Incidents: incidents})
	}
	sort.Slice(buckets, func(i, j int) bool {
		if buckets[i].Count != buckets[j].Count {
			return buckets[i].Count > buckets[j].Count
		}
		return buckets[i].ShapeHash < buckets[j].ShapeHash
	})
	sort.Slice(items, func(i, j int) bool {
		return items[i].Path < items[j].Path
	})
	report := IncidentArchiveReport{
		Version:   ArchiveReportVersion,
		Incidents: append([]IncidentItem(nil), items...),
		Buckets:   buckets,
	}
	report.Hash = canonical.Hash(report)
	return report
}

func (g *Graph) CausalCertificate(opts CauseQueryOptions) (CausalCertificate, error) {
	report, err := g.CauseReport(opts)
	if err != nil {
		return CausalCertificate{}, err
	}
	var claims []string
	for _, cause := range report.MinimalCauses {
		claims = append(claims, fmt.Sprintf("%s is a minimal sufficient cause for %s", cause.ID, strings.Join(report.Options.Starts, ",")))
	}
	if len(claims) == 0 {
		claims = append(claims, "no minimal cause satisfied the configured evidence threshold")
	}
	out := CausalCertificate{
		Version:     CausalCertificateVersion,
		Start:       append([]string(nil), report.Options.Starts...),
		Claims:      claims,
		Explanation: report.MinimalExplanation,
		Semiring:    report.Semiring,
		BlastRadius: report.BlastRadius,
		Missing:     report.MinimalExplanation.Missing,
		Certificate: "review artifact: trace slice + semiring evidence + blast radius + proof holes",
	}
	sort.Strings(out.Start)
	sort.Strings(out.Claims)
	out.Hash = canonical.Hash(struct {
		Version         string                  `json:"version"`
		Start           []string                `json:"start"`
		Claims          []string                `json:"claims"`
		ExplanationHash string                  `json:"explanation_hash"`
		Semiring        EvidenceSemiringSummary `json:"semiring"`
		BlastHash       string                  `json:"blast_hash"`
		Missing         []MissingEvidence       `json:"missing_evidence,omitempty"`
		Certificate     string                  `json:"certificate"`
	}{
		Version:         out.Version,
		Start:           out.Start,
		Claims:          out.Claims,
		ExplanationHash: out.Explanation.Hash,
		Semiring:        out.Semiring,
		BlastHash:       out.BlastRadius.Hash,
		Missing:         out.Missing,
		Certificate:     out.Certificate,
	})
	return out, nil
}

func ShapeHash(g *Graph) string {
	return canonical.Hash(sortedKeys(graphShapes(g)))
}

func normalizeCauseOptions(opts CauseQueryOptions) CauseQueryOptions {
	if len(opts.Starts) == 0 {
		opts.Starts = []string{"record:invoices/inv_1002"}
	}
	opts.Starts = append([]string(nil), opts.Starts...)
	sort.Strings(opts.Starts)
	if opts.MinEvidence == "" {
		opts.MinEvidence = EvidenceStrong
	}
	if opts.MaxDepth == 0 {
		opts.MaxDepth = 12
	}
	if len(opts.CauseKinds) == 0 {
		opts.CauseKinds = []EntityKind{KindSQLMutation, KindMigration, KindDeploy, KindCommit}
	}
	if len(opts.Allowed) == 0 {
		opts.Allowed = []EdgeKind{EdgeDeployedCommit, EdgeExecuted, EdgeCaused, EdgeMutated, EdgeDerivedInto, EdgeObserved, EdgeRepaired}
	}
	if len(opts.Observes) == 0 {
		opts.Observes = []EntityKind{KindRecord, KindReport, KindTrace, KindService}
	}
	return opts
}

func (g *Graph) ancestors(start string, opts CauseQueryOptions) map[string]bool {
	out := map[string]bool{start: true}
	queue := []searchNode{{ID: start}}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.Depth >= opts.MaxDepth {
			continue
		}
		for _, edge := range g.Incoming(current.ID) {
			if !containsEdge(opts.Allowed, edge.Kind) || !meetsEvidence(edge.Evidence, opts.MinEvidence) {
				continue
			}
			if out[edge.From] {
				continue
			}
			out[edge.From] = true
			queue = append(queue, searchNode{ID: edge.From, Depth: current.Depth + 1})
		}
	}
	return out
}

func (g *Graph) minimalCauseSet(candidates map[string]Entity, opts CauseQueryOptions) map[string]Entity {
	out := map[string]Entity{}
	for id, entity := range candidates {
		out[id] = entity
	}
	for left := range candidates {
		for right := range candidates {
			if left == right {
				continue
			}
			if g.forwardReachableID(left, right, opts) {
				delete(out, left)
				break
			}
		}
	}
	return out
}

func (g *Graph) commonAncestors(byStart map[string]map[string]bool) []EntitySummary {
	common := map[string]bool{}
	first := true
	for _, ancestors := range byStart {
		if first {
			for id := range ancestors {
				common[id] = true
			}
			first = false
			continue
		}
		for id := range common {
			if !ancestors[id] {
				delete(common, id)
			}
		}
	}
	entities := map[string]Entity{}
	for id := range common {
		if entity, ok := g.Entity(id); ok {
			entities[id] = entity
		}
	}
	return summarizeEntitiesMap(entities)
}

func (g *Graph) affectedObservations(causes map[string]Entity, opts CauseQueryOptions) []EntitySummary {
	out := map[string]Entity{}
	for id := range causes {
		for _, entity := range g.forwardReachable(id, opts) {
			if containsKind(opts.Observes, entity.Kind) {
				out[entity.ID] = entity
			}
		}
	}
	return summarizeEntitiesMap(out)
}

func (g *Graph) repairLineage(min EvidenceLevel) []EdgeSummary {
	edges := map[string]Edge{}
	for _, edge := range g.Edges() {
		if edge.Kind == EdgeRepaired && meetsEvidence(edge.Evidence, min) {
			edges[edgeKey(edge)] = edge
		}
	}
	return summarizeEdgesMap(edges)
}

func (g *Graph) shortestBackwardPath(start, target string, opts CauseQueryOptions) ([]Edge, bool) {
	queue := []pathNode{{ID: start}}
	visited := map[string]bool{start: true}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.ID == target {
			return current.Path, true
		}
		if current.Depth >= opts.MaxDepth {
			continue
		}
		for _, edge := range g.Incoming(current.ID) {
			if !containsEdge(opts.Allowed, edge.Kind) || !meetsEvidence(edge.Evidence, opts.MinEvidence) || visited[edge.From] {
				continue
			}
			visited[edge.From] = true
			nextPath := append(append([]Edge(nil), current.Path...), edge)
			queue = append(queue, pathNode{ID: edge.From, Depth: current.Depth + 1, Path: nextPath})
		}
	}
	return nil, false
}

func (g *Graph) forwardReachable(start string, opts CauseQueryOptions) []Entity {
	var out []Entity
	queue := []searchNode{{ID: start}}
	visited := map[string]bool{start: true}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.Depth >= opts.MaxDepth {
			continue
		}
		for _, edge := range g.Outgoing(current.ID) {
			if !containsEdge(opts.Allowed, edge.Kind) || !meetsEvidence(edge.Evidence, opts.MinEvidence) || visited[edge.To] {
				continue
			}
			entity, ok := g.Entity(edge.To)
			if !ok {
				continue
			}
			visited[edge.To] = true
			out = append(out, entity)
			queue = append(queue, searchNode{ID: edge.To, Depth: current.Depth + 1})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (g *Graph) forwardReachableID(start, target string, opts CauseQueryOptions) bool {
	for _, entity := range g.forwardReachable(start, opts) {
		if entity.ID == target {
			return true
		}
	}
	return false
}

type searchNode struct {
	ID    string
	Depth int
}

type pathNode struct {
	ID    string
	Depth int
	Path  []Edge
}

func missingEvidence(entities []Entity, edges []Edge) []MissingEvidence {
	incoming := map[string]map[EdgeKind]bool{}
	for _, edge := range edges {
		if incoming[edge.To] == nil {
			incoming[edge.To] = map[EdgeKind]bool{}
		}
		incoming[edge.To][edge.Kind] = true
	}
	var missing []MissingEvidence
	for _, entity := range entities {
		switch entity.Kind {
		case KindDeploy:
			addMissing(&missing, incoming, entity.ID, EdgeDeployedCommit, "deploy without deployed commit evidence")
		case KindMigration:
			addMissing(&missing, incoming, entity.ID, EdgeExecuted, "migration without execution evidence")
		case KindTrace:
			addMissing(&missing, incoming, entity.ID, EdgeObserved, "trace without observation evidence")
		case KindSQLMutation:
			addMissing(&missing, incoming, entity.ID, EdgeCaused, "sql mutation without causal trace evidence")
		case KindRecord:
			addMissing(&missing, incoming, entity.ID, EdgeMutated, "record without mutation evidence")
		case KindReport:
			addMissing(&missing, incoming, entity.ID, EdgeDerivedInto, "report without derivation evidence")
		}
	}
	sort.Slice(missing, func(i, j int) bool {
		if missing[i].Entity != missing[j].Entity {
			return missing[i].Entity < missing[j].Entity
		}
		return missing[i].Rule < missing[j].Rule
	})
	return missing
}

func addMissing(out *[]MissingEvidence, incoming map[string]map[EdgeKind]bool, id string, edge EdgeKind, rule string) {
	if incoming[id][edge] {
		return
	}
	*out = append(*out, MissingEvidence{Entity: id, Rule: rule, Need: string(edge)})
}

func summarizeEntitiesMap(in map[string]Entity) []EntitySummary {
	return summarizeEntities(sortedEntityMap(in))
}

func summarizeEntities(in []Entity) []EntitySummary {
	out := make([]EntitySummary, 0, len(in))
	for _, entity := range in {
		out = append(out, summarizeEntity(entity))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func summarizeEntity(entity Entity) EntitySummary {
	return EntitySummary{ID: entity.ID, Kind: entity.Kind, Name: entity.Name, Attributes: copyMap(entity.Attributes)}
}

func summarizeEdgesMap(in map[string]Edge) []EdgeSummary {
	edges := sortedEdgeMap(in)
	out := make([]EdgeSummary, 0, len(edges))
	for _, edge := range edges {
		out = append(out, EdgeSummary{From: edge.From, To: edge.To, Kind: edge.Kind, Evidence: edge.Evidence})
	}
	return out
}

func sortedEntityMap(in map[string]Entity) []Entity {
	out := make([]Entity, 0, len(in))
	for _, entity := range in {
		entity.Attributes = copyMap(entity.Attributes)
		out = append(out, entity)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func sortedKindCounts(in map[EntityKind]int) []KindCount {
	out := make([]KindCount, 0, len(in))
	for kind, count := range in {
		out = append(out, KindCount{Kind: kind, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Kind < out[j].Kind
	})
	return out
}

func sortedCounts(in map[string]int) []Count {
	out := make([]Count, 0, len(in))
	for value, count := range in {
		out = append(out, Count{Value: value, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Value < out[j].Value
	})
	return out
}

func recordTable(id string) string {
	if !strings.HasPrefix(id, "record:") {
		return ""
	}
	rest := strings.TrimPrefix(id, "record:")
	if idx := strings.Index(rest, "/"); idx > 0 {
		return rest[:idx]
	}
	return rest
}

func graphShapes(g *Graph) map[string]bool {
	shapes := map[string]bool{}
	for _, edge := range g.Edges() {
		from, _ := g.Entity(edge.From)
		to, _ := g.Entity(edge.To)
		shapes[edgeShape(from, edge, to)] = true
	}
	for _, entity := range g.Entities() {
		shapes["entity:"+entityShape(entity)] = true
	}
	return shapes
}

func entityShape(entity Entity) string {
	switch entity.Kind {
	case KindService:
		return string(entity.Kind) + ":" + stableString(entity.Name, entity.ID)
	case KindSQLMutation:
		return string(entity.Kind) + ":" + stableString(entity.Attributes["fingerprint"], entity.Name, "sql")
	case KindRecord:
		return string(entity.Kind) + ":" + stableString(recordTable(entity.ID), "record")
	case KindReport:
		return string(entity.Kind) + ":" + stableString(entity.Name, strings.TrimPrefix(entity.ID, "report:"))
	case KindMigration:
		return string(entity.Kind) + ":" + stableString(entity.Name, "migration")
	case KindCommit:
		return string(entity.Kind)
	case KindDeploy:
		return string(entity.Kind)
	case KindTrace:
		return string(entity.Kind)
	default:
		return string(entity.Kind)
	}
}

func edgeShape(from Entity, edge Edge, to Entity) string {
	return "edge:" + entityShape(from) + "->" + entityShape(to) + ":" + string(edge.Kind) + ":" + string(semiringValue(edge.Evidence))
}

func stableString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return "unknown"
}

func sortedKeys(in map[string]bool) []string {
	out := make([]string, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func semiringValue(level EvidenceLevel) EvidenceValue {
	switch level {
	case EvidenceExact:
		return EvidenceValueExact
	case EvidenceStrong, EvidenceMedium:
		return EvidenceValueStrong
	case EvidenceWeak, "":
		return EvidenceValueWeak
	default:
		if strings.Contains(strings.ToLower(string(level)), "redacted") {
			return EvidenceValueRedacted
		}
		return EvidenceValueWeak
	}
}

func levelFromValue(value EvidenceValue) EvidenceLevel {
	switch value {
	case EvidenceValueExact:
		return EvidenceExact
	case EvidenceValueStrong:
		return EvidenceStrong
	case EvidenceValueWeak:
		return EvidenceWeak
	default:
		return ""
	}
}

func evidenceValueRank(value EvidenceValue) int {
	switch value {
	case EvidenceValueExact:
		return 4
	case EvidenceValueStrong:
		return 3
	case EvidenceValueWeak:
		return 2
	case EvidenceValueRedacted:
		return 1
	case EvidenceValueAbsent:
		return 0
	case EvidenceValueConflicting:
		return -1
	default:
		return 0
	}
}
