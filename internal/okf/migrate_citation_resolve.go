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
	"net/url"
	"sort"
	"strings"
)

// A prose-citation item names no resource the tool can read (§5.1), so the
// tool never makes one up. The person resolving the item can: §5.1 allows a
// `resource` to be a concrete artifact (an absolute URL or a bundle path) or
// a population or scope descriptor a consumer cannot follow. They write it on
// the item in the plan file (`resource`, and optionally `title`), and the
// next plan reads those items back (MigrateOptions.Resolutions). Each
// resolved entry then converts like any other: it gets a source with that
// resource and an id, its full text becomes the footnote [^id], and body [k]
// references become [^id] (§4.2).
//
// A resolution names its entry by the item's path and context, exactly as
// the plan wrote them. One that matches no prose-citation item the plan
// finds fails the plan: a stale resolution dropped without a word would
// leave the author thinking the entry converted.
//
// Once applied, the resolved resource lives only in frontmatter, so a later
// plan must recognise the entry without the resolution. An entry that names
// no resource is held when the footnote definition of a source with an id
// holds its text (heldSource, the same test citationListLoss uses to remove
// a list). Such an entry is attached to that source, never reported as a
// prose-citation again, and its footnote label is its own.

// resolutionKey names one citation entry: its node and its text as the plan
// wrote it in the item's context.
type resolutionKey struct{ path, context string }

// resolutionIndex keys the resolutions that carry a resource. An item with no
// resource is not a resolution and is skipped. One that carries a resource on
// any kind but prose-citation, or two that resolve one entry differently, is
// an error.
func resolutionIndex(rs []JudgmentItem) (map[resolutionKey]JudgmentItem, error) {
	out := map[resolutionKey]JudgmentItem{}
	for _, r := range rs {
		r.Resource = strings.TrimSpace(r.Resource)
		r.Title = strings.TrimSpace(r.Title)
		if r.Resource == "" {
			continue
		}
		if r.Kind != JudgmentProseCitation {
			return nil, fmt.Errorf("resolution for %s: only a %s item takes a resource, not %s", r.Path, JudgmentProseCitation, r.Kind)
		}
		k := resolutionKey{r.Path, r.Context}
		if prev, ok := out[k]; ok && (prev.Resource != r.Resource || prev.Title != r.Title) {
			return nil, fmt.Errorf("resolution for %s: two resolutions for one entry: %q", r.Path, r.Context)
		}
		out[k] = r
	}
	return out, nil
}

// resolutionWrote reports whether have, the resource of the source holding a
// resolved entry, is what the resolution want writes. A root-relative bundle
// path is written `/`-rooted when the §12.1 fallback reads it from the root
// (rooted). Once the bundle declares v0.3 the fallback no longer applies and
// rooted leaves want as it is, so the `/`-rooted form of want is matched too.
func resolutionWrote(rooted func(string) string, want, have string) bool {
	if have == want || have == rooted(want) {
		return true
	}
	if !strings.HasPrefix(have, "/") || strings.HasPrefix(want, "/") ||
		strings.ContainsAny(want, " \t\r\n") || hasURIScheme(want) {
		return false
	}
	p := want
	if i := strings.IndexAny(p, "#?"); i >= 0 {
		p = p[:i]
	}
	if dec, err := url.PathUnescape(p); err == nil {
		p = dec
	}
	rootRel, escaped := joinWithin("", p)
	return !escaped && rootedURL(want, rootRel) == have
}

// rootedIn returns the §6.2/§12.1 rewrite for a resource named in node rel:
// a bare resource that resolves only from the bundle root is written
// `/`-rooted, the form its body link is rewritten to, so it resolves in the
// v0.3 bundle and a re-plan finds it. Anything else is returned unchanged.
func rootedIn(b *Bundle, rel string) func(string) string {
	return func(res string) string {
		if r := b.ResolvePathField(rel, res); r.Kind == RefBundle && r.Form == FormRootFallback {
			return rootedURL(res, r.Path)
		}
		return res
	}
}

// replayedResolutions settles the resolutions the citation pass did not use
// against the sources their nodes already hold. An applied resolution whose
// entry is held may no longer sit in any list: the apply removes a list once
// every entry in it is held. When the source holding the entry carries the
// resource the resolution writes, the resolution is a replay and is marked
// used. When it carries another, the returned map records that resource, so
// the error can say the entry is already resolved rather than missing.
func replayedResolutions(b *Bundle, idx map[resolutionKey]JudgmentItem, used map[resolutionKey]bool) map[resolutionKey]string {
	conflicts := map[resolutionKey]string{}
	for k, r := range idx {
		if used[k] {
			continue
		}
		n := b.Nodes[k.path]
		if n == nil || n.Frontmatter == nil {
			continue
		}
		s, held := heldSource(frontmatterSources(n.Frontmatter), footnoteDefsOutsideCitations(n.Body), k.context)
		if !held {
			continue
		}
		if resolutionWrote(rootedIn(b, k.path), r.Resource, s.resource) {
			used[k] = true
			continue
		}
		conflicts[k] = s.resource
	}
	return conflicts
}

// unmatchedResolutions is an error naming every resolution the plan did not
// use, or nil when it used them all. A resolution whose entry is already held
// by a source with another resource (conflicts) says so; any other matches no
// item.
func unmatchedResolutions(idx map[resolutionKey]JudgmentItem, used map[resolutionKey]bool, conflicts map[resolutionKey]string) error {
	var miss, clash []string
	for k, r := range idx {
		if used[k] {
			continue
		}
		if have, ok := conflicts[k]; ok {
			clash = append(clash, fmt.Sprintf("%s: %q is already resolved to %q, not %q", k.path, k.context, have, r.Resource))
			continue
		}
		miss = append(miss, fmt.Sprintf("%s: %q", k.path, k.context))
	}
	var errs []string
	if len(clash) > 0 {
		sort.Strings(clash)
		errs = append(errs, fmt.Sprintf("%d resolution(s) name a different resource than the source already holding the entry (edit the source in frontmatter to change it): %s",
			len(clash), strings.Join(clash, "; ")))
	}
	if len(miss) > 0 {
		sort.Strings(miss)
		errs = append(errs, fmt.Sprintf("%d resolution(s) match no %s item in the bundle (the entry changed, or the path is wrong): %s",
			len(miss), JudgmentProseCitation, strings.Join(miss, "; ")))
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(errs, "; and "))
}

// footnoteDefs maps each footnote label in text (folded) to its definition's
// first line, whitespace collapsed. The first definition of a label wins.
func footnoteDefs(text string) map[string]string {
	defs := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		loc := footnoteDefRe.FindStringSubmatchIndex(line)
		if loc == nil {
			continue
		}
		l := strings.ToLower(line[loc[2]:loc[3]])
		if _, dup := defs[l]; !dup {
			defs[l] = collapse(line[loc[1]:])
		}
	}
	return defs
}

// footnoteDefsOutsideCitations is footnoteDefs over body without its
// `# Citations` section.
func footnoteDefsOutsideCitations(body string) map[string]string {
	if s, e, ok := citationSectionSpan(body); ok {
		body = body[:s] + body[e:]
	}
	return footnoteDefs(body)
}

// heldSource returns the source whose footnote definition holds entry's text
// (holdsEntry on footnoteText): a source with an id and a resource, in
// frontmatter order. ok is false when no footnote of such a source holds it.
func heldSource(srcs []noteSource, defs map[string]string, entry string) (noteSource, bool) {
	for _, s := range srcs {
		if s.id == "" || s.resource == "" {
			continue
		}
		if d, ok := defs[strings.ToLower(s.id)]; ok && holdsEntry(d, collapse(footnoteText(entry, s.id))) {
			return s, true
		}
	}
	return noteSource{}, false
}
