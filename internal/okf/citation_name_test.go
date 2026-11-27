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

import "testing"

// §5.1 `title` is "a label for humans": the cited work's name. A citation
// entry also carries authors, venue, year, identifiers, URLs, and
// verification tags; none of those belong in the title. citationName finds
// the name only where the entry's own markup marks it, and otherwise
// reports no name rather than guessing.

// Positive control: the name shapes the real corpus uses.
func TestConformance_CitationName_Section5_1_FindsTheWorkName(t *testing.T) {
	for entry, want := range map[string]string{
		"- **[1] [VERIFIED — PEER-REVIEWED]** Y. Jiang et al., *FollowBench: A Multi-level Fine-grained Constraints Following Benchmark for Large Language Models*, ACL 2024 main. arXiv:2310.20410 — https://arxiv.org/abs/2310.20410 — GPT-4 HSR 84.7%→62.4%.": "FollowBench: A Multi-level Fine-grained Constraints Following Benchmark for Large Language Models",
		"[4] VERIFIED — Apple, *CMAltimeter*: `startRelativeAltitudeUpdates(to:withHandler:)`. <https://developer.apple.com/documentation/coremotion/cmaltimeter>":                                                                                               "CMAltimeter",
		"[11] VERIFIED — Apple, *MLActivityClassifier* (Create ML, macOS 10.15+): \"A model you train\". <https://developer.apple.com/x>":                                                                                                                        "MLActivityClassifier",
		"[6] Dykes, T., \"Personas Make Users Memorable,\" Nielsen Norman Group, 3 October 2025. https://www.nngroup.com/articles/persona/ — **VERIFIED**":                                                                                                       "Personas Make Users Memorable",
		"[7] ISO/IEC 17024:2012, \"Conformity assessment — General requirements for bodies operating certification of persons.\" https://en.wikipedia.org/wiki/ISO/IEC_17024":                                                                                    "Conformity assessment — General requirements for bodies operating certification of persons",
		"[1] VERIFIED (Wayback OP) — r/AppleWatch, \"Anyone here use Redpoint for indoor climbing\" — reports the app logging 694 ft. <https://www.reddit.com/r/x>":                                                                                              "Anyone here use Redpoint for indoor climbing",
		"[1] [The agentic-era team operating model](/research/agentic-era-team-operating-model.md) — the parent paradigm node (authority NUANCED)":                                                                                                               "The agentic-era team operating model",
		"https://support.google.com/a/answer/2685650 — Email routing and delivery options: Default routing vs Routing. **[PRIMARY — §1 routing]**":                                                                                                               "Email routing and delivery options",
		"https://support.apple.com/guide/deployment/x/web — \"Device management restrictions available in Apple Configurator for Mac\" (Configurator 2.18+): the list":                                                                                           "Device management restrictions available in Apple Configurator for Mac",
		// Italics mark the container, quotes the piece inside it: the piece is
		// the cited work.
		"- [1] Elizabeth Stone (CTO, Netflix), *Lenny's Podcast*, \"How Netflix builds a culture of excellence,\" published 2025. https://www.youtube.com/x": "How Netflix builds a culture of excellence",
	} {
		got, ok := citationName(entry)
		if !ok || got != want {
			t.Errorf("citationName(%q)\n got %q ok=%v\nwant %q", entry, got, ok, want)
		}
	}
}

// Negative control: emphasis or quotes that mark a claim, a term, or a
// search string are not a name, and an entry with no marked name has none.
func TestConformance_CitationName_Section5_1_NoNameWhenNotMarked(t *testing.T) {
	for _, entry := range []string{
		"[2] VERIFIED — HTTP 401 Unauthorized is the *authentication* failure status (RFC 9110 §15.5.2). <https://www.rfc-editor.org/rfc/rfc9110>",
		"[4] VERIFIED — USPTO wordmark search for \"cairn\" on tmsearch.uspto.gov (2026-10-12): **99 total results**. <https://tmsearch.uspto.gov/>",
		"[9] VERIFIED — Google Cloud Cloud DPA, definition of **\"Supervisory Authority\"**: \"(a) a 'supervisory authority'\". <https://cloud.google.com/terms/data-processing-addendum>",
		"[2] VERIFIED — Same page, \"Partner model features\" + \"Set a policy for partner model features\": the constraint. <https://cloud.google.com/x>",
		"[2] VERIFIED (framing) — the three-category factor taxonomy (know / have / are). https://pages.nist.gov/800-63-3/",
		"[1] Color theory sources — see [/design/color-theory.md](/design/color-theory.md) (Newton 1704).",
		"https://docs.controld.com/docs/ios-platform — `.mobileconfig` install; \"Prevent Disablement (Org Accounts Only)\". **[PRIMARY]**",
		"- https://wiki.acme/x",
		"[8] **[INDUSTRY-REPORT — primary-verified]** Stack Overflow 2025 Developer Survey, AI section. <https://survey.stackoverflow.co/2025/ai>",
		// A quoted excerpt after a URL is evidence, not the page's name: it
		// runs on into `;` or ends the entry.
		"- https://support.apple.com/guide/apple-configurator-mac/x/mac — \"Supervision generally denotes that the device is owned by the organization\"; \"You can … set up devices\"",
		"- https://docs.controld.com/docs/ios-platform — \"Prevent Disablement (Org Accounts Only) … supervised mode.\" **[PRIMARY — §2 corroboration]**",
		// An ellipsis marks a truncated quotation, never a whole name.
		"[2] VERIFIED — Acme, \"Prevent Disablement … supervised mode\", 2025. https://a.example/",
	} {
		if got, ok := citationName(entry); ok {
			t.Errorf("citationName(%q) = %q, want no name", entry, got)
		}
	}
}
