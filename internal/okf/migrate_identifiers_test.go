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
	"os/exec"
	"strings"
	"testing"
)

// Spec-conformance suite for identifier-built sources (OKF v0.3 §5.1,
// Appendix B). A v0.1 `# Citations` entry that names no URL but carries one
// DOI, arXiv id, PubMed id or ISBN gets its §5.1 `resource` built offline from
// that identifier. The identifier is in the author's own text, so the resource
// is derived, not invented.

// TestConformance_MigrateIdentifier_Section5_1_EachKindBuildsResource is the
// positive control, one case per identifier kind and spelling.
func TestConformance_MigrateIdentifier_Section5_1_EachKindBuildsResource(t *testing.T) {
	for entry, want := range map[string]string{
		"[11] Bessen 2015, SSRN, DOI 10.2139/ssrn.2690435.":                                      "https://doi.org/10.2139/ssrn.2690435",
		"[5] Ford et al. 1997, CHI EA, doi:10.1145/1120212.1120387":                              "https://doi.org/10.1145/1120212.1120387",
		"- **Deflated Sharpe Ratio** — Bailey, DOI `10.2139/ssrn.2460551` (JPM).":                "https://doi.org/10.2139/ssrn.2460551",
		"[2] Cleveland & McGill (JASA 1984, 10.1080/01621459.1984.10478080).":                    "https://doi.org/10.1080/01621459.1984.10478080",
		"[2] Chen et al., \"Dense X Retrieval\", arXiv:2312.06648 (2023-12-11).":                 "https://arxiv.org/abs/2312.06648",
		"[3] Old-style preprint, arXiv:hep-th/9901001v2.":                                        "https://arxiv.org/abs/hep-th/9901001v2",
		"[4] Maguire et al. (2000), PNAS, PMID: 10716738.":                                       "https://pubmed.ncbi.nlm.nih.gov/10716738/",
		"[6] Aho, Lam, Sethi, Ullman, *Compilers*, ISBN 978-0-321-48681-3.":                      "https://openlibrary.org/isbn/9780321486813",
		"[9] Lieberman 2013, *The Story of the Human Body*, ISBN 9780307741806":                  "https://openlibrary.org/isbn/9780307741806",
		"[7] Itten, *The Art of Color* (1961), ISBN 0-471-28929-9.":                              "https://openlibrary.org/isbn/0471289299",
		"[8] A book with an X check digit, ISBN 0-8044-2957-X.":                                  "https://openlibrary.org/isbn/080442957X",
		"[10] Same DOI twice, doi:10.1000/ABC and again DOI 10.1000/abc.":                        "https://doi.org/10.1000/ABC",
		"[12] DOI in parens (Journal, doi:10.1016/j.arr.2006.03.003).":                           "https://doi.org/10.1016/j.arr.2006.03.003",
		"[13] Parenthesized DOI that holds parens: doi:10.1002/(SICI)1097-4571(199806)49:8.":     "https://doi.org/10.1002/(SICI)1097-4571(199806)49:8",
		"[14] VERIFIED — Raichlen et al. (2017), *Am J Hum Biol*, doi:10.1002/ajhb.22919; read.": "https://doi.org/10.1002/ajhb.22919",
	} {
		s, ok := citationSource(entry)
		if !ok {
			t.Errorf("§5.1: entry with one identifier was not converted: %q", entry)
			continue
		}
		if s.Resource != want {
			t.Errorf("§5.1: entry %q resource = %q, want %q", entry, s.Resource, want)
		}
		// Casey's rule: a title is the work's name only, never the full
		// citation. An identifier-built source is titled by citationName
		// like any other, so it is never the entry text.
		if s.Title == entryText(entry) {
			t.Errorf("§5.1: identifier-built source must not carry the entry text as title, got %q", s.Title)
		}
	}
}

// TestConformance_MigrateIdentifier_Section5_1_IdentifierSourceGetsNameOnly:
// a source built from a DOI, arXiv id, PubMed id or ISBN is titled with the
// cited work's name (citationName), the same as one read from a URL. An entry
// that marks no name gets no title.
func TestConformance_MigrateIdentifier_Section5_1_IdentifierSourceGetsNameOnly(t *testing.T) {
	for entry, want := range map[string]SourceEdit{
		"[1] VERIFIED — N. F. Liu et al., *Lost in the Middle: How Language Models Use Long Contexts*, TACL 2023. arXiv:2307.03172.": {
			Resource: "https://arxiv.org/abs/2307.03172",
			Title:    "Lost in the Middle: How Language Models Use Long Contexts",
		},
		"[2] Smith, \"A Quoted Paper Title,\" J. Things 4, 2019. doi:10.1000/xyz123": {
			Resource: "https://doi.org/10.1000/xyz123",
			Title:    "A Quoted Paper Title",
		},
		"[3] Same DOI twice, doi:10.1000/ABC and again DOI 10.1000/abc.": {
			Resource: "https://doi.org/10.1000/ABC",
		},
	} {
		s, ok := citationSource(entry)
		if !ok || s != want {
			t.Errorf("entry %q = %+v ok=%v, want %+v", entry, s, ok, want)
		}
	}
}

// TestConformance_MigrateIdentifier_Section5_1_BadISBNChecksumNotLinked is the
// negative control: an ISBN whose check digit is wrong names no book, so no
// Open Library link is built and the entry stays a judgment item.
func TestConformance_MigrateIdentifier_Section5_1_BadISBNChecksumNotLinked(t *testing.T) {
	for _, entry := range []string{
		"[6] Aho et al., *Compilers*, ISBN 978-0-321-48681-4.", // ISBN-13, last digit off by one
		"[7] Itten, *The Art of Color*, ISBN 0-471-28929-8.",   // ISBN-10, last digit off by one
		"[8] Too short to be an ISBN, ISBN 12345.",
		"[9] Not a 978/979 prefix, ISBN 123-4-567-89012-8.",
	} {
		if s, ok := citationSource(entry); ok {
			t.Errorf("§5.1: invalid ISBN was linked: %q -> %+v", entry, s)
		}
	}
}

// TestConformance_MigrateIdentifier_Section5_1_URLAlreadyPresentNotDuplicated:
// an entry that already names a URL keeps it as its one resource. An
// identifier that repeats the URL (or names another document) adds nothing, so
// the DOI never becomes a second source and the conversion is the one made
// before identifiers were read. The title is the work's name only
// (citationName): none for an entry that marks no name.
func TestConformance_MigrateIdentifier_Section5_1_URLAlreadyPresentNotDuplicated(t *testing.T) {
	for entry, want := range map[string]SourceEdit{
		"[1] Ordóñez & Roggen, *Sensors* 2016, DOI 10.3390/s16010115. <https://doi.org/10.3390/s16010115>": {
			Resource: "https://doi.org/10.3390/s16010115",
		},
		"[2] Paper, arXiv:2310.20410 — https://arxiv.org/abs/2310.20410": {
			Resource: "https://arxiv.org/abs/2310.20410",
		},
		"[3] [The paper](https://example.org/p), doi:10.1000/xyz": {
			Resource: "https://example.org/p",
			Title:    "The paper",
		},
	} {
		s, ok := citationSource(entry)
		if !ok || s != want {
			t.Errorf("entry %q = %+v ok=%v, want %+v", entry, s, ok, want)
		}
	}
}

// TestConformance_MigrateIdentifier_Section5_1_SeveralIdentifiersStayJudgment
// is the second negative control: two different identifiers name two
// documents (or two editions), and §5.1 allows one resource per source, so the
// entry stays a judgment item rather than picking one.
func TestConformance_MigrateIdentifier_Section5_1_SeveralIdentifiersStayJudgment(t *testing.T) {
	for _, entry := range []string{
		"[3] Raichlen et al., doi:10.1002/ajhb.22919, PMID 27723159.",
		"[4] (a) ReAct, arXiv:2210.03629; (b) Reflexion, arXiv:2303.11366.",
		"[12] Womack et al. 1990, ISBN 978-0-89256-350-0; 2007 ed. ISBN 978-0-7432-9979-4.",
		"[6] VERIFIED — on-box probe, no URL, no identifier.",
		"[7] Version 10.2 of the tool, released 2024.",
	} {
		if s, ok := citationSource(entry); ok {
			t.Errorf("§5.1: entry %q was converted to %+v, want judgment", entry, s)
		}
	}
}

// TestConformance_MigrateIdentifier_Section5_1_PlanBuildsAndIsIdempotent runs
// the whole plan: the identifier entry becomes a source, and a node whose
// sources already hold the built URL (in any DOI letter case, or via the
// legacy dx.doi.org host) plans nothing for it (§12, re-running is a no-op).
func TestConformance_MigrateIdentifier_Section5_1_PlanBuildsAndIsIdempotent(t *testing.T) {
	dir := mkMigrateBundle(t, map[string]string{
		".okf":     "okf_version: 0.1\n",
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# KB\n",
		"log.md":   "# Log\n",
		"a.md": "---\ntype: Paper\n---\n\n# Def\n\n# Citations\n" +
			"[1] Bessen 2015, SSRN, DOI 10.2139/ssrn.2690435.\n" +
			"[2] Aho et al., *Compilers*, ISBN 978-0-321-48681-3.\n",
		"b.md": "---\ntype: Paper\nsources:\n  - resource: https://dx.doi.org/10.2139/SSRN.2690435\n---\n\n# Def\n\n# Citations\n" +
			"[1] Bessen 2015, SSRN, DOI 10.2139/ssrn.2690435.\n",
	})
	_, plan := planFor(t, dir, "human:casey", false)
	var got []SourceEdit
	for _, nm := range plan.Nodes {
		if nm.Path == "b.md" && len(nm.Sources) > 0 {
			t.Errorf("§12: b.md already cites the DOI; planned %+v", nm.Sources)
		}
		if nm.Path == "a.md" {
			got = nm.Sources
		}
	}
	want := []SourceEdit{
		{Resource: "https://doi.org/10.2139/ssrn.2690435"},
		{Resource: "https://openlibrary.org/isbn/9780321486813", Title: "Compilers"},
	}
	if len(got) != len(want) {
		t.Fatalf("a.md sources = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("a.md source %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	for _, j := range plan.Judgment {
		if j.Kind == JudgmentProseCitation {
			t.Errorf("identifier entry left as judgment: %+v", j)
		}
	}
}

// TestConformance_MigrateIdentifier_Section5_1_CrossReferenceStaysJudgment is
// a negative control from the real corpus: entry [2] cites a README and only
// mentions the paper that entry [1] already cites. The identifier is a cross-
// reference, not [2]'s own document, so [2] must stay a judgment item. It must
// not be folded into [1]'s source as "already migrated", which would drop
// [2] from the plan without a source of its own. The decision reads only the
// body list, so it is the same before and after apply.
func TestConformance_MigrateIdentifier_Section5_1_CrossReferenceStaysJudgment(t *testing.T) {
	files := map[string]string{
		".okf":     "okf_version: 0.1\n",
		"index.md": "---\nokf_version: \"0.1\"\n---\n\n# KB\n",
		"log.md":   "# Log\n",
		"a.md": "---\ntype: Paper\n---\n\n# Def\n\n# Citations\n" +
			"[1] Tallam, *Five-Plane Architecture*, arXiv:2606.12320. https://arxiv.org/abs/2606.12320\n\n" +
			"[2] VERIFIED. CAGE `README.md` at `56b751d`; attribution to arXiv:2606.12320.\n",
	}
	dir := mkMigrateBundle(t, files)
	_, plan := planFor(t, dir, "human:casey", false)
	if !hasJudgment(plan, "a.md", JudgmentProseCitation) {
		t.Fatalf("§5.1: cross-reference entry [2] vanished from the plan; judgment = %+v", plan.Judgment)
	}
	for _, nm := range plan.Nodes {
		for _, s := range nm.Sources {
			if s.Resource != "https://arxiv.org/abs/2606.12320" {
				t.Errorf("unexpected source %+v", s)
			}
		}
	}
}

// TestMigrateIdentifier_NoNetworkInDefaultPath proves the default run makes no
// network call: the package that plans and applies migrations cannot reach
// the network at all, because neither it nor anything it imports links the
// net or net/http packages. net/url and net/netip parse strings only.
func TestMigrateIdentifier_NoNetworkInDefaultPath(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	for _, dep := range strings.Fields(string(out)) {
		switch dep {
		case "net", "net/http", "crypto/tls", "net/rpc", "net/smtp":
			t.Errorf("internal/okf links %s; the default migrate path must stay offline", dep)
		}
	}
}
