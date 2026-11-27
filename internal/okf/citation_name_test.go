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
		"- [2] Jane Doe, *Lenny's Podcast*, “How Netflix ships,” published 2025. https://www.youtube.com/y":                                                  "How Netflix ships",
	} {
		got, ok := citationName(entry)
		if !ok || got != want {
			t.Errorf("citationName(%q)\n got %q ok=%v\nwant %q", entry, got, ok, want)
		}
	}
}

// Negative control from the real corpus: an entry that leads with a bold
// span marks the work with that span, so a later italic is the venue, never
// the name. Bold is not one of the marks citationName reads, so the entry has
// no name rather than the journal's. A bold grade tag (`**VERIFIED**`) does
// not mark the work and still lets the italic name through.
func TestConformance_CitationName_Section5_1_BoldLeadIsNotTheVenue(t *testing.T) {
	for _, entry := range []string{
		"- **The Probability of Backtest Overfitting** — D. H. Bailey, J. M. Borwein, M. López de Prado, Q. J. Zhu, *Journal of Computational Finance* (2016), DOI `10.21314/jcf.2016.322`.",
		"- **Deflated Sharpe Ratio** — Bailey & López de Prado, *The Journal of Portfolio Management*, 2014. https://example.org/dsr",
	} {
		if got, ok := citationName(entry); ok {
			t.Errorf("citationName(%q) = %q, want no name (the italic is the venue)", entry, got)
		}
	}
	const graded = "- **VERIFIED** — A. Author, *The Real Name*, 2024. https://example.org/x"
	if got, ok := citationName(graded); !ok || got != "The Real Name" {
		t.Errorf("citationName(%q) = %q ok=%v, want \"The Real Name\"", graded, got, ok)
	}
	// Positive controls from the same corpus: a grade tag with a lowercase
	// qualifier is still a tag, and a bold lead followed by a quoted title
	// names the quoted work (quotes mark the piece, italics the venue).
	for entry, want := range map[string]string{
		"[7] **VERIFIED (primary, arXiv)** — Aditya Kusupati et al., \"Matryoshka Representation Learning,\" arXiv:2205.13147 (2022-05-26).":                                                                                                     "Matryoshka Representation Learning",
		"- **Critique conditions, presenter/critiquer roles.** Nielsen Norman Group, Gibbons, S., \"Design Critiques: Encourage a Positive Culture to Improve Products,\" nngroup.com, 2016. https://www.nngroup.com/articles/design-critiques/": "Design Critiques: Encourage a Positive Culture to Improve Products",
	} {
		if got, ok := citationName(entry); !ok || got != want {
			t.Errorf("citationName(%q)\n got %q ok=%v\nwant %q", entry, got, ok, want)
		}
	}
}

// §5.1 `title` names the work. Many entries quote the page instead: a repo
// description, a docs sentence, an error message, a terms-of-service clause.
// That quote is evidence for the claim, not the work's name, so the entry has
// no name and the source stays untitled (reported as source-title-unnamed).
// Each entry is a real corpus shape.
func TestConformance_CitationName_Section5_1_QuotedExcerptIsNotAName(t *testing.T) {
	for _, entry := range []string{
		// Introduced as a description, verbatim text, or error message.
		"[4] VERIFIED — `freqtrade/freqtrade`, license **GPL-3.0** (GitHub spdx_id `GPL-3.0`). Description: \"Free, open source crypto trading bot.\" Last push 2026-06-26. Source: <https://api.github.com/repos/freqtrade/freqtrade>.",
		"[2] VERIFIED — vLLM, GitHub REST API: <https://api.github.com/repos/vllm-project/vllm>. Description: \"A high-throughput and memory-efficient inference and serving engine for LLMs\".",
		"[5] VERIFIED — Amazon EKS, aws.amazon.com/eks/, retrieved 2026-08-18. Verbatim: \"Managed Kubernetes,\" \"certified Kubernetes,\" \"conformant.\" <https://aws.amazon.com/eks/>",
		"[1] VERIFIED — `google/cybernetic-agent-governance-engine` README at `main` HEAD `56b751d` (2026-11-25): description, \"not an officially supported Google product\", Objectives. https://github.com/google/x",
		"[3] VERIFIED — RFC 6750 §3.1. The `insufficient_scope` error: \"The request requires higher privileges than provided by the access token.\" https://www.rfc-editor.org/rfc/rfc6750",
		// Introduced by a backticked ref or a URL and a colon.
		"[25] VERIFIED — Privacy.com API, `https://developers.privacy.com/docs/cards`: card `type` values include `SINGLE_USE`; `spend_limit`: \"Transaction requests above the spend limit will be declined\".",
		"[15] VERIFIED: Mastodon OAuth tokens, https://docs.joinmastodon.org/api/oauth-tokens/: \"Tokens Will Not Expire Automatically.\"",
		// A sentence, or more than one.
		"[8] VERIFIED — Sessionize, DevFest Ottawa 2026: \"Call for Speakers is closed. Submissions are no longer possible\"; event 5 Dec 2026. https://sessionize.com/devfest-ottawa-2026/",
		"[1] VERIFIED — google/elephant-goldfish-model README at HEAD `8b1553f` (2026-11-22): tagline, \"Goldfish reviews require fresh subagent contexts. If your agent cannot create them, EGM stops the review.\" https://github.com/google/elephant-goldfish-model",
		"[P1] VERIFIED — Headless `gcloud auth print-access-token`, prior probe, \"Reauthentication failed. cannot prompt during non-interactive execution.\" Recorded as [P6].",
		// A lowercase clause fragment.
		"[8] VERIFIED [primary] AwardClaw (C6 Labs LLC), Terms of Service §4 (last updated 2026-07-27): \"aggregate publicly available award seat data\". https://www.awardclaw.com/terms",
		"[4] Unicode CLDR — Common Locale Data Repository, https://cldr.unicode.org/ — \"the most trusted and comprehensive repository of locale data,\" 100+ languages. **VERIFIED.**",
		// One of a list of quoted excerpts.
		"[3] VERIFIED — pretalx, GrafanaCON 2027 CFP: \"Event Dates: April 27-29, 2027\"; \"Event location: New York, NY\". https://pretalx.com/grafanacon-2027/cfp",
		// After a colon, a quote that is not title case is page text.
		"[x] VERIFIED — Vercel — Fluid compute docs: \"As of April 23, 2025, fluid compute is enabled by default for new projects\". https://vercel.com/docs/fluid-compute",
		"[x] VERIFIED — Gartner, `https://www.gartner.com/en/research/methodologies/gartner-hype-cycle`: \"The Hype Cycle research is reserved for paying clients\".",
		// A deprecation notice (citationEntries joins wrapped lines with spaces).
		"[19] VERIFIED — Generative AI module in the Vertex AI SDK, deprecation notice: \"deprecated and will no longer be available after June 24, 2026\". The removed modules include `vertexai.generative_models`. https://cloud.google.com/vertex-ai/x",
		// A sentence quoted right after a leading URL and a dash.
		"https://docs.acme.example/tokens — \"Tokens are scoped to one project. Rotate them often\" (docs page)",
	} {
		if got, ok := citationName(entry); ok {
			t.Errorf("citationName(%q) = %q, want no name (the quote is an excerpt)", entry, got)
		}
	}
	// After an italic container, a quoted excerpt is not the piece: the
	// container is the only name the entry marks.
	const container = "[3] Acme, *Acme API Reference*, \"Tokens expire after one hour. Refresh them before then\", 2025. https://acme.example/api"
	if got, ok := citationName(container); !ok || got != "Acme API Reference" {
		t.Errorf("citationName(%q) = %q ok=%v, want \"Acme API Reference\"", container, got, ok)
	}
}

// Negative control for the excerpt rules: real names that share their
// surface features still come through. A quoted title may hold `vs.`, an
// initialism, a `?`, a lowercase first letter inside a word, or its own
// closing period; italics are never read as excerpts; and a title-case
// quote after a label colon names the work.
func TestConformance_CitationName_Section5_1_ExcerptRulesKeepRealNames(t *testing.T) {
	for entry, want := range map[string]string{
		"[2] Miles, A. & Bechhofer, S. (eds.), *SKOS Simple Knowledge Organization System Reference*, W3C Recommendation, 2009. https://www.w3.org/TR/skos-reference/":                       "SKOS Simple Knowledge Organization System Reference",
		"[2] VERIFIED. W3C, \"Understanding SC 1.2.3: Audio Description or Media Alternative (Prerecorded),\" Level A. https://www.w3.org/WAI/WCAG22/Understanding/x.html":                   "Understanding SC 1.2.3: Audio Description or Media Alternative (Prerecorded)",
		"[10] VERIFIED — AWS What's New, *AWS WAF announces Web Bot Auth support* (Posted 2025-11-21): https://aws.amazon.com/about-aws/whats-new/x":                                         "AWS WAF announces Web Bot Auth support",
		"[1] W3C Internationalization, \"Localization vs. Internationalization\" (Richard Ishida, W3C), https://www.w3.org/International/questions/qa-i18n — the i18n/l10n distinction.":     "Localization vs. Internationalization",
		"[53] VERIFIED [vendor page] SerpApi, \"U.S. Legal Shield\": Production plan and above. https://serpapi.com/us-legal-shield":                                                         "U.S. Legal Shield",
		"[3] Scheibehenne, B., \"Can There Ever Be Too Many Options? A Meta-Analytic Review of Choice Overload,\" *Journal of Consumer Research*, 2010. https://doi.org/10.1086/651235":      "Can There Ever Be Too Many Options? A Meta-Analytic Review of Choice Overload",
		"[3] Source video: \"Why the 60-30-10 Rule is RUINING Your UI Designs,\" YouTube, https://youtu.be/66oOi9OLMCw (transcript fetched 2026-07-02).":                                     "Why the 60-30-10 Rule is RUINING Your UI Designs",
		"[S22b] Liquid AI, \"d1.2: State-of-the-Art Omnimodal Decision Models\", 2026-11-16. https://liquid.ai/blog/d1-2. VERIFIED (vendor claims).":                                         "d1.2: State-of-the-Art Omnimodal Decision Models",
		"[13] VERIFIED [project blog] — Etienne Perot and Jing Chen, \"gVisor is being donated to CNCF\", 2026-10-02: donating the name and trademarks. https://gvisor.dev/blog/x":           "gVisor is being donated to CNCF",
		"[23] VERIFIED — Google Play listing for package `xyz.block.buzz.mobile` (2026-11-18): \"Buzz - Apps on Google Play,\" developer Block, Inc. <https://play.google.com/store/apps/x>": "Buzz - Apps on Google Play",
		"[5] Practitioner consensus against overlays: *Overlay Fact Sheet*, https://overlayfactsheet.com/ (a community statement).":                                                          "Overlay Fact Sheet",
		// A question mark ends a title's question, not a quoted sentence.
		"[14] Barnett, S. M. & Ceci, S. J. (2002), \"When and where do we apply what we learn? A taxonomy for far transfer,\" *Psychological Bulletin* 128(4):612–637. https://doi.org/10.1037/0033-2909.128.4.612": "When and where do we apply what we learn? A taxonomy for far transfer",
		"[16] UNVERIFIED-tier (vendor study). Viibeo, \"How fast do viral Shorts cut? A 54-video study,\" 2026-10-15. https://www.viibeo.com/research/viral-shorts-pacing":                                          "How fast do viral Shorts cut? A 54-video study",
	} {
		if got, ok := citationName(entry); !ok || got != want {
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
