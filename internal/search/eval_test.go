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

package search

import (
	"fmt"
	"os"
	"testing"

	"github.com/cwest/okfctl/internal/okf"
)

// evalPair is one gold retrieval judgment: a natural-language query and the
// bundle-relative path of the node whose content answers it. The gold node was
// hand-vetted against the real corpus; several targets live in a BURIED section
// of a long, multi-heading node — exactly the case whole-node mean pooling loses.
type evalPair struct {
	query    string
	wantNode string
}

// goldSet is the retrieval benchmark. It is deliberately mixed:
//   - "buried-passage" queries whose answer is one small section of a long node
//     (Colima fork, Cloudflare-tunnel fork, backup fork, deskilling section,
//     traffic-splitting section, rule-of-three tell) — the cases the passage
//     layer is meant to fix.
//   - "already-served" queries a short/on-topic node already answered under
//     whole-node pooling (family-link, em-dash, keyless-SA, screen-time) — the
//     negative control: passage ranking must not regress these.
var goldSet = []evalPair{
	{"should I use colima or docker desktop for the container runtime", "casey/dawarich-deployment-hermes-integration-design.md"},
	{"how to expose the ingestion endpoint publicly with cloudflare tunnel", "casey/dawarich-deployment-hermes-integration-design.md"},
	{"backup strategy for the self-hosted deployment on this host", "casey/dawarich-deployment-hermes-integration-design.md"},
	{"deskilling trap for junior engineers in an agent team", "casey/agentic-team-operating-model-self-audit.md"},
	{"traffic splitting revisions rollback for agent deployments", "research/platform-scale-agent-harness-fleet.md"},
	{"the rule of three reflex in AI-written prose", "content/anti-ai-writing-voice.md"},
	{"is the em dash still a reliable sign of AI written text", "content/anti-ai-writing-voice.md"},
	{"give a child their own consumer google account with family link", "casey/child-first-account-and-ipad-parental-controls.md"},
	{"screen time downtime app limits passcode on ipad", "casey/child-first-account-and-ipad-parental-controls.md"},
	{"grant an agent GCP access with keyless service account impersonation", "research/granting-an-agent-google-access.md"},
	{"per-user oauth to read workspace gmail and calendar for an agent", "research/granting-an-agent-google-access.md"},
}

// TestEval_RetrievalQuality measures MRR and recall@5 of the passage-based Query
// against the real corpus with a real model2vec embedder, and — as a control —
// the whole-node ranking (rank over Entries) on the same store. It is skipped
// unless OKFCTL_EVAL_CORPUS (a bundle dir) and OKFCTL_TEST_MODEL_DIR (a
// potion-base-8M dir) are both set, because neither is vendored. Run:
//
//	OKFCTL_EVAL_CORPUS=~/src/knowledge-base/bundles/knowledge \
//	OKFCTL_TEST_MODEL_DIR=<potion-base-8M dir> \
//	go test ./internal/search/ -run TestEval_RetrievalQuality -v
func TestEval_RetrievalQuality(t *testing.T) {
	corpus := os.Getenv("OKFCTL_EVAL_CORPUS")
	modelDir := os.Getenv("OKFCTL_TEST_MODEL_DIR")
	if corpus == "" || modelDir == "" {
		t.Skip("set OKFCTL_EVAL_CORPUS and OKFCTL_TEST_MODEL_DIR to run the retrieval-quality eval")
	}
	b, err := okf.Load(corpus)
	if err != nil {
		t.Fatalf("load corpus: %v", err)
	}
	e, err := LoadModel2VecEmbedder(modelDir)
	if err != nil {
		t.Fatalf("load model: %v", err)
	}
	s := BuildIndex(b, e, nil)

	passageMRR, passageRecall := evalRanker(t, s, e, func(qv []float64) []Result {
		return rankPassages(s.Passages, qv, 5, Filter{}, nil)
	})
	wholeMRR, wholeRecall := evalRanker(t, s, e, func(qv []float64) []Result {
		return rank(s.Entries, qv, 5, "", Filter{}, nil)
	})

	t.Logf("corpus=%s nodes=%d passages=%d queries=%d", corpus, len(s.Entries), len(s.Passages), len(goldSet))
	t.Logf("BEFORE (whole-node) : MRR=%.3f recall@5=%.3f", wholeMRR, wholeRecall)
	t.Logf("AFTER  (passages)   : MRR=%.3f recall@5=%.3f", passageMRR, passageRecall)

	// The passage layer must not regress the aggregate — the whole point is to
	// close the gap upward, never to trade quality away.
	if passageMRR < wholeMRR {
		t.Errorf("passage MRR %.3f regressed below whole-node MRR %.3f", passageMRR, wholeMRR)
	}
	if passageRecall < wholeRecall {
		t.Errorf("passage recall@5 %.3f regressed below whole-node recall@5 %.3f", passageRecall, wholeRecall)
	}
}

// evalRanker runs the gold set through a ranking function and returns MRR and
// recall@5. rankFn receives the query vector and returns up to 5 ranked results.
func evalRanker(t *testing.T, s *Store, e Embedder, rankFn func(qv []float64) []Result) (mrr, recall float64) {
	t.Helper()
	var rrSum float64
	var hits int
	for _, g := range goldSet {
		qv := e.Encode([]string{g.query})[0]
		res := rankFn(qv)
		rank := goldRank(res, g.wantNode)
		if rank > 0 {
			rrSum += 1.0 / float64(rank)
			if rank <= 5 {
				hits++
			}
		}
		if testing.Verbose() {
			t.Logf("  rank=%s q=%q -> %s", rankStr(rank), g.query, g.wantNode)
		}
	}
	n := float64(len(goldSet))
	return rrSum / n, float64(hits) / n
}

// goldRank returns the 1-based rank of wantNode in res, or 0 if absent.
func goldRank(res []Result, wantNode string) int {
	for i, r := range res {
		if r.Path == wantNode {
			return i + 1
		}
	}
	return 0
}

func rankStr(rank int) string {
	if rank == 0 {
		return ">5"
	}
	return fmt.Sprintf("%d", rank)
}
