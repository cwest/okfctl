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
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// LintFinding is one judgment-worthy observation about bundle health. Unlike a
// validate Finding (a spec-floor violation), a lint finding is curation
// guidance — never a format failure.
type LintFinding struct {
	Check   string `json:"check"` // "orphan" | "missing-xref" | "coverage-gap" | "type-hygiene" | "broken-link"
	Path    string `json:"path"`  // node path the finding is about ("" for bundle-level findings)
	Message string `json:"message"`
}

// LintOptions configures the deterministic structural checks.
type LintOptions struct {
	// CoverageThreshold is the number of distinct nodes that must mention a
	// term (with no node of its own) before it is reported as a coverage gap.
	// Zero means the default (3).
	CoverageThreshold int
}

const defaultCoverageThreshold = 3

// Lint runs the deterministic, stdlib-only structural checks over a bundle and
// returns findings sorted by path then check. It never mutates the bundle.
func Lint(b *Bundle, opts LintOptions) []LintFinding {
	threshold := opts.CoverageThreshold
	if threshold <= 0 {
		threshold = defaultCoverageThreshold
	}

	var findings []LintFinding
	findings = append(findings, lintOrphans(b)...)
	findings = append(findings, lintMissingXrefs(b)...)
	findings = append(findings, lintBrokenLinks(b)...)
	findings = append(findings, lintCoverageGaps(b, threshold)...)
	findings = append(findings, lintTypeHygiene(b)...)

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Check < findings[j].Check
	})
	return findings
}

// inboundCounts returns, for every concept node, the number of DISTINCT sources
// that link to it — including the reserved index.md (the bundle's front door).
func inboundCounts(b *Bundle) map[string]int {
	// src -> set of resolved targets, so a source linking the same target twice
	// counts once.
	counts := map[string]int{}
	for target := range b.Nodes {
		counts[target] = 0
	}

	seen := map[string]map[string]bool{} // target -> set of sources
	add := func(src, target string) {
		if _, ok := b.Nodes[target]; !ok {
			return // only count inbound to concept nodes
		}
		if src == target {
			return // a self-link does not rescue a node from orphanhood
		}
		if seen[target] == nil {
			seen[target] = map[string]bool{}
		}
		if !seen[target][src] {
			seen[target][src] = true
			counts[target]++
		}
	}

	for src, n := range b.Nodes {
		dir := filepath.Dir(src)
		for _, l := range scanNodeLinks(b, dir, n.Body) {
			if l.resolved != "" {
				add(src, l.resolved)
			}
		}
	}
	// Reserved files (index.md especially) also confer reachability.
	for src, n := range b.Reserved {
		dir := filepath.Dir(src)
		for _, l := range scanNodeLinks(b, dir, n.Body) {
			if l.resolved != "" {
				add(src, l.resolved)
			}
		}
	}
	return counts
}

func lintOrphans(b *Bundle) []LintFinding {
	counts := inboundCounts(b)
	var out []LintFinding
	for path := range b.Nodes {
		if counts[path] == 0 {
			out = append(out, LintFinding{
				Check:   "orphan",
				Path:    path,
				Message: fmt.Sprintf("orphan: %s has no inbound links (unreachable by traversal)", path),
			})
		}
	}
	return out
}

// proseBody returns the node's prose body with any leading YAML frontmatter
// fence removed. For a well-formed node the loader already stripped frontmatter,
// so Body is prose-only and this is a no-op. But when a node's frontmatter fails
// to parse, the loader preserves the whole file as Body (so validate can flag
// it); this strips a leading `---` ... `---` fence from that raw text so the
// lint checks scan PROSE only — frontmatter metadata (type/title/status/aliases
// values) is never a concept mention. Uses the same splitter as the loader so
// the fence detection can never diverge.
func proseBody(n *Node) string {
	if s, ok := splitFrontmatter([]byte(n.Body)); ok {
		return string(s.body)
	}
	return n.Body
}

// linkedTargets returns the set of node paths a given node already links to.
func linkedTargets(b *Bundle, path string, n *Node) map[string]bool {
	out := map[string]bool{}
	dir := filepath.Dir(path)
	for _, l := range scanNodeLinks(b, dir, n.Body) {
		if l.resolved != "" {
			out[l.resolved] = true
		}
	}
	return out
}

func lintMissingXrefs(b *Bundle) []LintFinding {
	// Map every concept node's title (lowercased) to its path. A title shared by
	// more than one node is AMBIGUOUS: a bare mention cannot be attributed to a
	// single node, so it is dropped (recorded as "" and skipped below) rather
	// than resolved to whichever node happened to load last.
	titleToPath := map[string]string{}
	ambiguous := map[string]bool{}
	for path, n := range b.Nodes {
		t := strings.ToLower(strings.TrimSpace(nodeTitle(n)))
		if t == "" {
			continue
		}
		if _, seen := titleToPath[t]; seen {
			ambiguous[t] = true
			continue
		}
		titleToPath[t] = path
	}

	var out []LintFinding
	for path, n := range b.Nodes {
		body := strings.ToLower(proseBody(n))
		linked := linkedTargets(b, path, n)
		// Deterministic order over titles.
		var titles []string
		for t := range titleToPath {
			titles = append(titles, t)
		}
		sort.Strings(titles)
		for _, title := range titles {
			if ambiguous[title] {
				continue // cannot resolve a shared title to one node
			}
			target := titleToPath[title]
			if target == path {
				continue // a node mentioning its own title is not a missing xref
			}
			if linked[target] {
				continue // already links to it
			}
			if containsWord(body, title) {
				out = append(out, LintFinding{
					Check:   "missing-xref",
					Path:    path,
					Message: fmt.Sprintf("missing-xref: %s mentions %q but does not link to %s", path, nodeTitle(b.Nodes[target]), target),
				})
			}
		}
	}
	return out
}

// containsWord reports whether needle appears in haystack as a whole-phrase
// occurrence (bounded by non-alphanumeric chars or string edges). Both args are
// expected lowercased by the caller.
func containsWord(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	from := 0
	for {
		i := strings.Index(haystack[from:], needle)
		if i < 0 {
			return false
		}
		start := from + i
		end := start + len(needle)
		leftOK := start == 0 || !isWordByte(haystack[start-1])
		rightOK := end == len(haystack) || !isWordByte(haystack[end])
		if leftOK && rightOK {
			return true
		}
		from = start + 1
	}
}

func isWordByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// lintBrokenLinks reports a dangling internal link ONLY when it looks like a
// DEFECT rather than a genuine coverage gap. The discriminator: a node with the
// same basename exists elsewhere in the bundle. That catches moved and mistyped
// paths — the migration hazard behind any `node mv`, directory reorg, or
// wikilink-to-Markdown rewrite — and stays quiet for a genuinely unwritten
// concept (which has no node anywhere, so nothing shares its basename). The gap
// case remains analyze's advisory coverage_gaps.dangling_links; this only ADDS a
// gate, it does not move that signal.
//
// The finding names BOTH the bad target and the resolved candidate path, so the
// fix is obvious without a second lookup. If several nodes share the basename,
// all candidates are listed (sorted) rather than guessing one.
func lintBrokenLinks(b *Bundle) []LintFinding {
	// basename -> sorted node paths carrying that basename.
	byBase := map[string][]string{}
	for p := range b.Nodes {
		base := path.Base(p)
		byBase[base] = append(byBase[base], p)
	}
	for base := range byBase {
		sort.Strings(byBase[base])
	}

	var out []LintFinding
	for _, p := range sortedNodePaths(b) {
		n := b.Nodes[p]
		for _, tgt := range danglingTargets(b, p, n) {
			// Strip any anchor, then take the basename of the target path.
			link := tgt
			if i := strings.IndexByte(link, '#'); i >= 0 {
				link = link[:i]
			}
			base := path.Base(link)
			candidates := byBase[base]
			if len(candidates) == 0 {
				continue // genuinely unwritten: a coverage gap, not a defect
			}
			var hint string
			if len(candidates) == 1 {
				hint = fmt.Sprintf("did you mean %s?", candidates[0])
			} else {
				hint = fmt.Sprintf("did you mean one of: %s?", strings.Join(candidates, ", "))
			}
			out = append(out, LintFinding{
				Check:   "broken-link",
				Path:    p,
				Message: fmt.Sprintf("broken-link: %s links to %s which resolves to no node; %s", p, tgt, hint),
			})
		}
	}
	return out
}

// lintCoverageGaps reports KNOWN concept terms — terms some node declares as a
// title or alias — that have no node of their own yet are referenced by
// `threshold` or more DISTINCT nodes (via a plain-text mention or another node's
// alias declaration).
//
// Precision rules (increment 8): a candidate must be a real concept, not prose.
//   - A term is "covered" (not a gap) when it equals an existing node's title.
//   - A term is a gap CANDIDATE only when it is "known": declared as a title or
//     alias somewhere in the corpus. Bare capitalized prose words/phrases that
//     no node ever names as a concept (sentence-initial "The"/"This", ALLCAPS
//     values like "VERIFIED", passing proper nouns like "Google Cloud") are not
//     concepts and are never reported.
//   - Single capitalized words are dropped unless declared as an alias — the bar
//     is "multiword OR a known/declared term".
//
// This targets the real corpus: matching capitalized prose surfaces produced
// 2,276 findings dominated by stopwords, ALLCAPS status values, and proper
// nouns; keying on declared concepts keeps only defensibly-real gaps.
func lintCoverageGaps(b *Bundle, threshold int) []LintFinding {
	// A concept is "covered" when a node's TITLE names it: either the title
	// equals the term, or the term is a whole-phrase prefix of the title. The
	// prefix case is what credits a node whose title elaborates on the concept
	// it is about — e.g. a node titled "Block Buzz vs. Discord as the Hermes
	// channel" IS the home for "Block Buzz", so cross-linking "Block Buzz" from
	// other nodes must not report it as an uncovered gap. Aliases remain a
	// candidate/known-concept signal below (a term some node declares as an
	// alias is a KNOWN concept), but do not by themselves confer a home node —
	// an alias on an unrelated node still leaves the aliased concept without its
	// own node, which is a real gap.
	covered := map[string]bool{}
	for _, n := range b.Nodes {
		t := strings.ToLower(strings.TrimSpace(nodeTitle(n)))
		if t == "" {
			continue
		}
		covered[t] = true
		for _, term := range titlePrefixTerms(t) {
			covered[term] = true
		}
	}

	// Known concept terms declared as an alias by some node, keyed lowercased to
	// a canonical display form (first seen). A title that has a node is covered
	// above; a title without a node is unusual but still a known concept.
	declared := map[string]string{}
	for _, n := range b.Nodes {
		for _, a := range nodeAliases(n) {
			key := strings.ToLower(strings.TrimSpace(a))
			if key == "" || covered[key] {
				continue
			}
			if _, ok := declared[key]; !ok {
				declared[key] = strings.TrimSpace(a)
			}
		}
	}

	// Count distinct nodes referencing each known term — a plain-text multiword
	// mention in the body, or the term declared as an alias by that node.
	mentions := map[string]map[string]bool{}
	note := func(key, path string) {
		if mentions[key] == nil {
			mentions[key] = map[string]bool{}
		}
		mentions[key][path] = true
	}
	for path, n := range b.Nodes {
		// Alias declarations by this node.
		for _, a := range nodeAliases(n) {
			key := strings.ToLower(strings.TrimSpace(a))
			if key != "" && !covered[key] {
				note(key, path)
			}
		}
		// Plain-text multiword concept mentions in this node's body.
		for _, term := range candidateTerms(proseBody(n)) {
			key := strings.ToLower(term)
			if covered[key] {
				continue // already has a node
			}
			if _, ok := declared[key]; !ok {
				continue // not a known/declared concept — prose noise, skip
			}
			note(key, path)
		}
	}

	var out []LintFinding
	keys := make([]string, 0, len(mentions))
	for k := range mentions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if len(mentions[k]) >= threshold {
			out = append(out, LintFinding{
				Check:   "coverage-gap",
				Path:    "",
				Message: fmt.Sprintf("coverage-gap: %q is referenced by %d nodes but has no node of its own", declared[k], len(mentions[k])),
			})
		}
	}
	return out
}

// titlePrefixTerms returns leading whole-word prefixes of a (lowercased) node
// title, so a concept term that a title elaborates on is credited as covered.
// A title like "block buzz vs. discord as the hermes channel" yields
// {"block buzz vs", "block buzz", "block"} — the first run of words before a
// hard boundary (sentence/clause punctuation or a comparison connective like
// "vs"/"versus"), accumulated word by word. This credits a node whose title
// leads with the concept it is about (so "Block Buzz" is covered by that node)
// without crediting an unrelated tail phrase. Single-word prefixes are included
// so a one-word concept declared as an alias and leading its own node's title
// is covered too. Only multi-word terms and declared aliases are ever matched
// against this set (see lintCoverageGaps / candidateTerms), so admitting the
// single-word prefix here cannot resurface bare-prose noise.
func titlePrefixTerms(title string) []string {
	fields := strings.Fields(title)
	var lead []string
	for _, w := range fields {
		trimmed := strings.Trim(w, ".,;:!?()[]\"'`*_")
		// Stop at a clause/sentence boundary or a comparison connective: the
		// words after it describe a different or contrasted concept, not the
		// leading one this node is named for.
		if trimmed == "" || trimmed == "vs" || trimmed == "versus" || trimmed == "and" || trimmed == "or" {
			break
		}
		lead = append(lead, trimmed)
		// A trailing punctuation mark on the original word ends the run.
		if strings.ContainsAny(w[len(w)-1:], ".,;:!?") {
			break
		}
	}
	var out []string
	for i := 1; i <= len(lead); i++ {
		out = append(out, strings.Join(lead[:i], " "))
	}
	return out
}

// nodeAliases returns the node's declared aliases (frontmatter `aliases:` list),
// as strings. A non-list or absent value yields nil.
func nodeAliases(n *Node) []string {
	raw, ok := n.Frontmatter["aliases"]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, v := range list {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

// candidateTerms extracts multiword Title-Case concept phrases from a body (e.g.
// "Malolactic Fermentation"). A phrase is a run of two or more consecutive
// capitalized words. Runs break at common English stopwords so that a
// sentence-initial "The"/"This"/"A"/"An" or a connective ("Of", "And") never
// starts or joins a phrase — that noise class (single capitalized words and
// stopword-led fragments) is what made the check unusable on the real corpus.
// Single-word candidates are intentionally excluded here; a single-word concept
// is admitted only when it is a declared alias (see lintCoverageGaps).
func candidateTerms(body string) []string {
	var terms []string
	seen := map[string]bool{}
	var cur []string
	flush := func() {
		if len(cur) >= 2 {
			term := strings.Join(cur, " ")
			if !seen[term] {
				seen[term] = true
				terms = append(terms, term)
			}
		}
		cur = nil
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue // skip headings (a node's own title heading is noise)
		}
		for _, w := range strings.Fields(line) {
			trimmed := strings.Trim(w, ".,;:!?()[]\"'`*_")
			if isCapitalizedWord(trimmed) && !isStopword(trimmed) {
				cur = append(cur, trimmed)
			} else {
				flush()
			}
		}
		flush()
	}
	return terms
}

// stopwords are common English function words that, when capitalized, signal
// sentence-initial position or a connective rather than a concept boundary.
var stopwords = map[string]bool{
	"a": true, "an": true, "the": true, "this": true, "that": true,
	"these": true, "those": true, "it": true, "its": true, "and": true,
	"or": true, "but": true, "nor": true, "for": true, "so": true,
	"yet": true, "as": true, "at": true, "by": true, "in": true,
	"of": true, "on": true, "to": true, "up": true, "off": true,
	"out": true, "over": true, "under": true, "with": true, "from": true,
	"into": true, "onto": true, "upon": true, "about": true, "above": true,
	"below": true, "after": true, "before": true, "between": true,
	"during": true, "through": true, "because": true, "while": true,
	"when": true, "where": true, "why": true, "how": true, "what": true,
	"which": true, "who": true, "whom": true, "whose": true, "if": true,
	"then": true, "else": true, "than": true, "too": true, "very": true,
	"just": true, "not": true, "no": true, "yes": true, "all": true,
	"any": true, "both": true, "each": true, "few": true, "more": true,
	"most": true, "other": true, "some": true, "such": true, "only": true,
	"own": true, "same": true, "here": true, "there": true, "now": true,
	"also": true, "however": true, "thus": true, "hence": true,
	"therefore": true, "instead": true, "rather": true, "once": true,
	"every": true, "our": true, "their": true, "his": true, "her": true,
	"my": true, "your": true, "us": true, "them": true, "me": true,
	"him": true, "is": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true, "has": true, "have": true,
	"had": true, "do": true, "does": true, "did": true, "will": true,
	"would": true, "can": true, "could": true, "should": true, "may": true,
	"might": true, "must": true, "shall": true,
}

func isStopword(w string) bool { return stopwords[strings.ToLower(w)] }

func isCapitalizedWord(w string) bool {
	if w == "" {
		return false
	}
	if w[0] < 'A' || w[0] > 'Z' {
		return false
	}
	for i := 1; i < len(w); i++ {
		c := w[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}
	return true
}

// lintTypeHygiene warns when two or more distinct `type` values fold to the same
// canonical form (case + trailing-'s' plural), which usually signals accidental
// drift. Anti-taxonomy stands: genuinely distinct types are never flagged.
func lintTypeHygiene(b *Bundle) []LintFinding {
	// canonical -> set of raw type values.
	groups := map[string]map[string]bool{}
	for _, n := range b.Nodes {
		raw := strings.TrimSpace(n.Type())
		if raw == "" {
			continue
		}
		c := canonType(raw)
		if groups[c] == nil {
			groups[c] = map[string]bool{}
		}
		groups[c][raw] = true
	}

	var out []LintFinding
	canons := make([]string, 0, len(groups))
	for c := range groups {
		canons = append(canons, c)
	}
	sort.Strings(canons)
	for _, c := range canons {
		if len(groups[c]) > 1 {
			variants := make([]string, 0, len(groups[c]))
			for v := range groups[c] {
				variants = append(variants, v)
			}
			sort.Strings(variants)
			out = append(out, LintFinding{
				Check:   "type-hygiene",
				Path:    "",
				Message: fmt.Sprintf("type-hygiene: near-duplicate type values likely refer to one type: %s", strings.Join(variants, ", ")),
			})
		}
	}
	return out
}

// canonType folds a type value to case-insensitive + singular (drop a single
// trailing 's') for near-duplicate grouping.
func canonType(s string) string {
	c := strings.ToLower(strings.TrimSpace(s))
	if len(c) > 1 && strings.HasSuffix(c, "s") {
		c = c[:len(c)-1]
	}
	return c
}
