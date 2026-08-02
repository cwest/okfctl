// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package okf

import (
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// AnalyzeOptions configures the proactive curation report. Zero values are
// filled with defaults by DefaultAnalyzeOptions / the command layer.
type AnalyzeOptions struct {
	// StaleDays: age (days) past which a node's freshness basis date is stale.
	StaleDays int
	// TimeSensitiveFraction: a time-sensitive node surfaces once its age is
	// >= TimeSensitiveFraction * StaleDays; undated marked nodes always surface.
	TimeSensitiveFraction float64
	// ThinLines: body line count (blank lines excluded) below which a node is
	// "thin".
	ThinLines int
	// ClusterMin: minimum nodes sharing a tag to flag a synthesis cluster.
	ClusterMin int
	// CoverageThreshold is passed through to Lint's coverage-gap check.
	CoverageThreshold int
}

// DefaultAnalyzeOptions returns the report defaults, mirroring the reference
// okf_analyze.py: 180-day staleness, 0.5 time-sensitive fraction, 15-line thin
// threshold, 3-node cluster minimum.
func DefaultAnalyzeOptions() AnalyzeOptions {
	return AnalyzeOptions{
		StaleDays:             180,
		TimeSensitiveFraction: 0.5,
		ThinLines:             15,
		ClusterMin:            3,
		CoverageThreshold:     defaultCoverageThreshold,
	}
}

func (o AnalyzeOptions) withDefaults() AnalyzeOptions {
	d := DefaultAnalyzeOptions()
	if o.StaleDays <= 0 {
		o.StaleDays = d.StaleDays
	}
	if o.TimeSensitiveFraction <= 0 {
		o.TimeSensitiveFraction = d.TimeSensitiveFraction
	}
	if o.ThinLines <= 0 {
		o.ThinLines = d.ThinLines
	}
	if o.ClusterMin <= 0 {
		o.ClusterMin = d.ClusterMin
	}
	if o.CoverageThreshold <= 0 {
		o.CoverageThreshold = d.CoverageThreshold
	}
	return o
}

// AnalyzeReport is the structured curation report across the five dimensions.
// It is JSON-serializable for the machine path (the curation sweep files
// research cards from it) and consumed by the human renderer.
type AnalyzeReport struct {
	Summary      AnalyzeSummary     `json:"summary"`
	Coverage     CoverageReport     `json:"coverage_gaps"`
	Freshness    FreshnessReport    `json:"freshness"`
	Connectivity ConnectivityReport `json:"connectivity"`
	Clusters     []ClusterFinding   `json:"clusters"`
	Structure    StructureReport    `json:"structure"`
}

// AnalyzeSummary carries corpus-level counts and the thresholds in effect.
type AnalyzeSummary struct {
	Nodes              int `json:"nodes"`
	TotalInternalLinks int `json:"total_internal_links"`
	StaleThresholdDays int `json:"stale_threshold_days"`
}

// AnalyzeNodeRef is a bare reference to a node by path (used where a finding
// carries no extra fields).
type AnalyzeNodeRef struct {
	Path string `json:"path"`
}

// CoverageReport groups the coverage / gap findings.
type CoverageReport struct {
	DanglingLinks  []DanglingLink   `json:"dangling_links"`
	ThinNodes      []ThinNode       `json:"thin_nodes"`
	Uncited        []AnalyzeNodeRef `json:"uncited"`
	SingleCitation []AnalyzeNodeRef `json:"single_citation"`
	KnownGaps      []string         `json:"known_gaps"`
}

// DanglingLink is a body link whose .md target resolves to no node.
type DanglingLink struct {
	From   string `json:"from"`
	Target string `json:"target"`
}

// ThinNode is a node whose body is below the thin-lines threshold.
type ThinNode struct {
	Path      string `json:"path"`
	BodyLines int    `json:"body_lines"`
}

// FreshnessReport groups the freshness findings.
type FreshnessReport struct {
	Stale         []StaleNode         `json:"stale"`
	TimeSensitive []TimeSensitiveNode `json:"time_sensitive"`
}

// StaleNode is a node whose freshness basis date is older than the threshold,
// or that carries no basis date at all (AgeDays nil, Basis "(none)").
type StaleNode struct {
	Path    string `json:"path"`
	AgeDays *int   `json:"age_days"`
	Basis   string `json:"basis"` // the date string compared, or "(none)"
}

// TimeSensitiveNode is a marked node aged past the time-sensitive gate.
type TimeSensitiveNode struct {
	Path    string   `json:"path"`
	AgeDays *int     `json:"age_days"`
	Markers []string `json:"markers"`
}

// ConnectivityReport groups the connectivity findings.
type ConnectivityReport struct {
	Orphans      []AnalyzeNodeRef `json:"orphans"`
	WeaklyLinked []WeaklyLinked   `json:"weakly_linked"`
}

// WeaklyLinked is a node with exactly one total in-bundle link.
type WeaklyLinked struct {
	Path string `json:"path"`
	In   int    `json:"in"`
	Out  int    `json:"out"`
}

// ClusterFinding is a tag shared by >= ClusterMin nodes with no synthesis node.
type ClusterFinding struct {
	Tag   string   `json:"tag"`
	Nodes []string `json:"nodes"`
}

// StructureReport groups the structural findings.
type StructureReport struct {
	DuplicateTitles    []DuplicateGroup `json:"duplicate_titles"`
	NearDuplicateSlugs []SlugPair       `json:"near_duplicate_slugs"`
}

// DuplicateGroup is a set of node paths whose titles fold to one key.
type DuplicateGroup struct {
	Members []string `json:"members"`
}

// SlugPair is two node paths whose base names are within edit-distance 1.
type SlugPair struct {
	A string `json:"a"`
	B string `json:"b"`
}

// timeSensitiveRe mirrors the reference analyzer's heuristic markers that a
// node's knowledge is time-sensitive.
var timeSensitiveRe = regexp.MustCompile(`(?i)\b(latest|current|as of|recently|today|this year|deprecated|beta|preview|roadmap|upcoming|pricing|version \d)`)

// danglingLinkRe matches CommonMark inline links; unlike scanNodeLinks (which
// only returns links that RESOLVE to a node), this finds .md targets that do
// NOT resolve — referenced-but-unwritten knowledge (OKF §5).
var danglingLinkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

// Analyze runs the five-dimension proactive curation report over a loaded
// bundle. It is READ-ONLY, pure (aside from the package clock for freshness),
// and deterministic (all output ordered by sort). It NEVER mutates the bundle
// and NEVER fails on findings — the caller decides exit semantics (report, not
// gate: exit 0 on a successful analysis regardless of finding count).
func Analyze(b *Bundle, opts AnalyzeOptions) AnalyzeReport {
	opts = opts.withDefaults()
	rep := AnalyzeReport{}
	rep.Summary = analyzeSummary(b, opts)
	rep.Coverage = analyzeCoverage(b, opts)
	rep.Freshness = analyzeFreshness(b, opts)
	rep.Connectivity = analyzeConnectivity(b)
	rep.Clusters = analyzeClusters(b, opts)
	rep.Structure = analyzeStructure(b)
	rep.normalizeSlices()
	return rep
}

// normalizeSlices replaces nil slices with empty slices so the JSON contract is
// stable: every dimension serializes as an array ([]), never null. The machine
// consumer (curation sweep) relies on iterable arrays.
func (r *AnalyzeReport) normalizeSlices() {
	if r.Coverage.DanglingLinks == nil {
		r.Coverage.DanglingLinks = []DanglingLink{}
	}
	if r.Coverage.ThinNodes == nil {
		r.Coverage.ThinNodes = []ThinNode{}
	}
	if r.Coverage.Uncited == nil {
		r.Coverage.Uncited = []AnalyzeNodeRef{}
	}
	if r.Coverage.SingleCitation == nil {
		r.Coverage.SingleCitation = []AnalyzeNodeRef{}
	}
	if r.Coverage.KnownGaps == nil {
		r.Coverage.KnownGaps = []string{}
	}
	if r.Freshness.Stale == nil {
		r.Freshness.Stale = []StaleNode{}
	}
	if r.Freshness.TimeSensitive == nil {
		r.Freshness.TimeSensitive = []TimeSensitiveNode{}
	}
	if r.Connectivity.Orphans == nil {
		r.Connectivity.Orphans = []AnalyzeNodeRef{}
	}
	if r.Connectivity.WeaklyLinked == nil {
		r.Connectivity.WeaklyLinked = []WeaklyLinked{}
	}
	if r.Clusters == nil {
		r.Clusters = []ClusterFinding{}
	}
	if r.Structure.DuplicateTitles == nil {
		r.Structure.DuplicateTitles = []DuplicateGroup{}
	}
	if r.Structure.NearDuplicateSlugs == nil {
		r.Structure.NearDuplicateSlugs = []SlugPair{}
	}
}

func analyzeSummary(b *Bundle, opts AnalyzeOptions) AnalyzeSummary {
	total := 0
	for p := range b.Nodes {
		total += len(b.OutboundLinks(p))
	}
	return AnalyzeSummary{
		Nodes:              len(b.Nodes),
		TotalInternalLinks: total,
		StaleThresholdDays: opts.StaleDays,
	}
}

func analyzeCoverage(b *Bundle, opts AnalyzeOptions) CoverageReport {
	out := CoverageReport{}
	for _, p := range sortedNodePaths(b) {
		n := b.Nodes[p]
		// Dangling forward-links: .md targets that resolve to no node.
		for _, tgt := range danglingTargets(b, p, n) {
			out.DanglingLinks = append(out.DanglingLinks, DanglingLink{From: p, Target: tgt})
		}
		// Thin nodes: non-blank body line count below the threshold.
		if bl := bodyLineCount(n.Body); bl < opts.ThinLines {
			out.ThinNodes = append(out.ThinNodes, ThinNode{Path: p, BodyLines: bl})
		}
		// Citation strength.
		switch citationCount(n.Body) {
		case 0:
			out.Uncited = append(out.Uncited, AnalyzeNodeRef{Path: p})
		case 1:
			out.SingleCitation = append(out.SingleCitation, AnalyzeNodeRef{Path: p})
		}
	}
	// Known coverage gaps: delegate to lint (no reimplementation).
	for _, f := range lintCoverageGaps(b, opts.CoverageThreshold) {
		out.KnownGaps = append(out.KnownGaps, f.Message)
	}
	sort.Strings(out.KnownGaps)
	return out
}

// danglingTargets returns the sorted, de-duped set of .md link targets in a
// node body that do NOT resolve to a known node (via any of the three forms
// scanNodeLinks resolves: "/"-absolute bundle-root relative, root-relative, or
// dir-relative) and are not reserved index/log links.
func danglingTargets(b *Bundle, nodePath string, n *Node) []string {
	dir := filepath.Dir(nodePath)
	seen := map[string]bool{}
	var out []string
	body := n.Body
	for _, loc := range danglingLinkRe.FindAllStringSubmatchIndex(body, -1) {
		if loc[0] > 0 && body[loc[0]-1] == '!' {
			continue // image, not a link
		}
		raw := body[loc[2]:loc[3]]
		fields := strings.Fields(raw)
		if len(fields) == 0 {
			continue
		}
		url := fields[0]
		if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "#") {
			continue
		}
		// Only .md targets are candidate concept references (strip any anchor).
		link := url
		if i := strings.IndexByte(link, '#'); i >= 0 {
			link = link[:i]
		}
		if !strings.HasSuffix(link, ".md") {
			continue
		}
		// A link to a reserved generated artifact is a navigational edge, never
		// a dangling concept reference.
		if IsReservedPath(link) {
			continue
		}
		// Resolves to a real node? -> not dangling. Three forms, matching the
		// shared edge-builder: "/"-absolute (bundle-root relative, OKF §5.1),
		// root-relative, then dir-relative against the linking node's dir.
		if strings.HasPrefix(link, "/") {
			if abs := filepath.ToSlash(filepath.Clean(strings.TrimPrefix(link, "/"))); b.Nodes[abs] != nil {
				continue
			}
		} else {
			if rootRel := filepath.ToSlash(filepath.Clean(link)); b.Nodes[rootRel] != nil {
				continue
			}
			if dirRel := filepath.ToSlash(filepath.Clean(filepath.Join(dir, link))); b.Nodes[dirRel] != nil {
				continue
			}
		}
		if !seen[url] {
			seen[url] = true
			out = append(out, url)
		}
	}
	sort.Strings(out)
	return out
}

// bodyLineCount counts non-blank lines in a body (matching the reference's
// body_lines: blank lines excluded).
func bodyLineCount(body string) int {
	n := 0
	for _, ln := range strings.Split(body, "\n") {
		if strings.TrimSpace(ln) != "" {
			n++
		}
	}
	return n
}

// citationEntryRe / citationOrderedRe / citationLegendRe mirror the reference
// corpus.py citation markers: bracketed keys ([1], [S1], [VERIFIED], bullet/bold
// variants), ordered-list entries (1., 2.), minus status-key legend lines.
var (
	citationEntryRe   = regexp.MustCompile(`^\s*(?:[-*]\s*)?(?:\*\*)?\[[A-Za-z0-9]+\]`)
	citationOrderedRe = regexp.MustCompile(`^\s*(?:[-*]\s*)?\d+\.\s+\S`)
	citationLegendRe  = regexp.MustCompile(`^\s*(?:[-*]\s*)?(?:\*\*)?\[[A-Za-z0-9]+\](?:\*\*)?\s*=`)
	citationsHeadRe   = regexp.MustCompile(`(?im)^#+\s*Citations\s*$`)
)

// citationCount counts distinct entry lines under a "# Citations" heading. A
// status-key legend line ([key] = ...) is not an entry. Counting is per line so
// a line matching multiple styles counts once.
func citationCount(body string) int {
	loc := citationsHeadRe.FindStringIndex(body)
	if loc == nil {
		return 0
	}
	section := body[loc[1]:]
	count := 0
	for _, line := range strings.Split(section, "\n") {
		if citationLegendRe.MatchString(line) {
			continue
		}
		if citationEntryRe.MatchString(line) || citationOrderedRe.MatchString(line) {
			count++
		}
	}
	return count
}

func analyzeFreshness(b *Bundle, opts AnalyzeOptions) FreshnessReport {
	out := FreshnessReport{}
	now := nowUTC()
	gate := opts.TimeSensitiveFraction * float64(opts.StaleDays)
	for _, p := range sortedNodePaths(b) {
		n := b.Nodes[p]
		basisStr, basisTime, dated := freshnessBasis(n)
		var agePtr *int
		if dated {
			age := int(now.Sub(basisTime).Hours() / 24)
			agePtr = &age
			if age > opts.StaleDays {
				out.Stale = append(out.Stale, StaleNode{Path: p, AgeDays: agePtr, Basis: basisStr})
			}
		} else {
			// No basis date -> can't assess freshness; flag softly.
			out.Stale = append(out.Stale, StaleNode{Path: p, AgeDays: nil, Basis: "(none)"})
		}
		// Time-sensitive: marker present AND (undated OR aged past the gate).
		if markers := timeSensitiveMarkers(n.Body); len(markers) > 0 {
			if agePtr == nil || float64(*agePtr) >= gate {
				out.TimeSensitive = append(out.TimeSensitive, TimeSensitiveNode{
					Path: p, AgeDays: agePtr, Markers: markers,
				})
			}
		}
	}
	return out
}

// freshnessBasis returns the basis date string, its parsed time, and whether a
// usable date was found. Resolution order:
//  1. n.Generated() — the spec's canonical `generated.at` (§5.2) with the legacy
//     `timestamp` fallback (§13.1). Generated() returns ok=false when neither
//     yields a usable date, so a zero-value `generated` mapping (an empty/absent
//     `at` and no legacy timestamp) is never read as a valid basis.
//  2. modified -> created — okfctl-native compatibility, unchanged. These keys
//     are not spec-defined; they carry our own v0.1 corpus, which records no
//     `generated`/`timestamp`.
func freshnessBasis(n *Node) (string, time.Time, bool) {
	// §5.2 / §13.1: prefer the spec fields via the shared provenance reader.
	// Generated() reports ok on `by` alone (§5.2 requires `by`), which leaves a
	// zero `At`; freshness needs a real DATE, so require a non-zero time here.
	if g, ok := n.Generated(); ok && !g.At.IsZero() {
		return g.At.Format("2006-01-02"), g.At, true
	}
	for _, key := range []string{"modified", "created"} {
		if raw, ok := n.Frontmatter[key]; ok {
			if t, ok := frontmatterTime(raw); ok {
				return frontmatterTimeString(raw), t, true
			}
		}
	}
	return "", time.Time{}, false
}

// frontmatterTimeString renders a frontmatter date value back to a short string
// for display, without re-deriving the parse.
func frontmatterTimeString(v any) string {
	switch t := v.(type) {
	case time.Time:
		return t.UTC().Format("2006-01-02")
	case string:
		return t
	}
	return ""
}

// timeSensitiveMarkers returns the sorted, de-duped, lowercased set of
// time-sensitive markers matched in a body (empty if none).
func timeSensitiveMarkers(body string) []string {
	matches := timeSensitiveRe.FindAllString(body, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range matches {
		k := strings.ToLower(m)
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func analyzeConnectivity(b *Bundle) ConnectivityReport {
	out := ConnectivityReport{}
	// inboundCounts is lint's single source of truth for inbound reachability
	// (index.md confers reachability). Reusing it means analyze and lint can
	// never disagree about what is orphaned.
	inbound := inboundCounts(b)
	for _, p := range sortedNodePaths(b) {
		outCount := 0
		for _, tgt := range b.OutboundLinks(p) {
			if _, ok := b.Nodes[tgt]; ok && tgt != p {
				outCount++
			}
		}
		in := inbound[p]
		switch {
		case in == 0 && outCount == 0:
			out.Orphans = append(out.Orphans, AnalyzeNodeRef{Path: p})
		case in+outCount == 1:
			out.WeaklyLinked = append(out.WeaklyLinked, WeaklyLinked{Path: p, In: in, Out: outCount})
		}
	}
	return out
}

func analyzeClusters(b *Bundle, opts AnalyzeOptions) []ClusterFinding {
	byTag := map[string][]string{}
	for _, p := range sortedNodePaths(b) {
		for _, tag := range nodeTags(b.Nodes[p]) {
			key := strings.ToLower(strings.TrimSpace(tag))
			if key == "" {
				continue
			}
			byTag[key] = append(byTag[key], p)
		}
	}
	var out []ClusterFinding
	tags := make([]string, 0, len(byTag))
	for t := range byTag {
		tags = append(tags, t)
	}
	sort.Strings(tags)
	for _, t := range tags {
		members := byTag[t]
		if len(members) >= opts.ClusterMin {
			sort.Strings(members)
			out = append(out, ClusterFinding{Tag: t, Nodes: members})
		}
	}
	return out
}

// nodeTags returns a node's frontmatter tags as strings (nil when absent). A
// scalar tag is treated as a single-element list. Non-string scalar elements
// (YAML parses a bare `403` as an int, `1.5` as a float, `true` as a bool) are
// coerced to their string form — the reference stringifies every tag element, so
// a numeric tag is a real tag and must still cluster.
func nodeTags(n *Node) []string {
	raw, ok := n.Frontmatter["tags"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []any:
		var out []string
		for _, t := range v {
			if s := scalarToString(t); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		if s := scalarToString(v); s != "" {
			return []string{s}
		}
	}
	return nil
}

// scalarToString renders a YAML scalar (string, int, int64, float64, bool) to
// its string form, matching the reference's str(t) coercion. Non-scalar values
// (maps, nested lists) yield "".
func scalarToString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	}
	return ""
}

func analyzeStructure(b *Bundle) StructureReport {
	out := StructureReport{}
	// Duplicate / near-duplicate titles: fold to an alphanumeric-lowercase key.
	byTitle := map[string][]string{}
	for _, p := range sortedNodePaths(b) {
		key := foldTitle(nodeTitle(b.Nodes[p]))
		if key != "" {
			byTitle[key] = append(byTitle[key], p)
		}
	}
	keys := make([]string, 0, len(byTitle))
	for k := range byTitle {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if len(byTitle[k]) > 1 {
			members := append([]string(nil), byTitle[k]...)
			sort.Strings(members)
			out.DuplicateTitles = append(out.DuplicateTitles, DuplicateGroup{Members: members})
		}
	}
	// Near-duplicate slugs: base names (minus .md) within edit distance 1.
	paths := sortedNodePaths(b)
	slugs := make([]string, len(paths))
	for i, p := range paths {
		slugs[i] = strings.TrimSuffix(path.Base(p), ".md")
	}
	for i := 0; i < len(paths); i++ {
		for j := i + 1; j < len(paths); j++ {
			if slugs[i] == slugs[j] {
				continue // identical base name is a slug collision, not near-dup
			}
			if editDistanceWithin1(slugs[i], slugs[j]) {
				out.NearDuplicateSlugs = append(out.NearDuplicateSlugs, SlugPair{A: paths[i], B: paths[j]})
			}
		}
	}
	return out
}

// foldTitle folds a title to a case-insensitive alphanumeric-only key so that
// "Tannin" and "tannin!" collide, matching the reference's duplicate-title fold.
func foldTitle(title string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(title) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// editDistanceWithin1 reports whether a and b differ by at most one single-
// character insertion, deletion, or substitution. It short-circuits on a length
// gap > 1 and never computes a full DP table (O(n) with early exit).
func editDistanceWithin1(a, b string) bool {
	la, lb := len(a), len(b)
	if la == lb {
		diffs := 0
		for i := 0; i < la; i++ {
			if a[i] != b[i] {
				diffs++
				if diffs > 1 {
					return false
				}
			}
		}
		return diffs == 1 // identical handled by caller; here exactly-1 substitution
	}
	// Length differs by exactly 1: check for a single insertion/deletion.
	if la > lb {
		a, b = b, a
		la, lb = lb, la
	}
	if lb-la != 1 {
		return false
	}
	i, j, edits := 0, 0, 0
	for i < la && j < lb {
		if a[i] == b[j] {
			i++
			j++
			continue
		}
		edits++
		if edits > 1 {
			return false
		}
		j++ // skip the extra char in the longer string
	}
	return true
}
