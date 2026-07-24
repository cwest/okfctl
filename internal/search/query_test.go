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

import "testing"

func TestQuery_RanksClosestTop(t *testing.T) {
	b, _ := fixtureBundle(t)
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)
	// A query sharing tokens with the Tannin node should rank it first.
	res, err := Query(s, e, "tannin structure astringency", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) == 0 {
		t.Fatal("no results")
	}
	if res[0].Path != "wine/tannin.md" {
		t.Errorf("top result = %q, want wine/tannin.md; results=%+v", res[0].Path, res)
	}
	// sorted descending
	for i := 1; i < len(res); i++ {
		if res[i-1].Score < res[i].Score {
			t.Errorf("results not sorted desc at %d: %+v", i, res)
		}
	}
}

func TestQuery_KHonored(t *testing.T) {
	b, _ := fixtureBundle(t)
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)
	res, err := Query(s, e, "wine", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) > 2 {
		t.Errorf("k=2 honored: got %d results", len(res))
	}
}

func TestRelated_ExcludesSelf(t *testing.T) {
	b, _ := fixtureBundle(t)
	e := NewHashEmbedder()
	s := BuildIndex(b, e, nil)
	res, err := Related(s, "wine/tannin.md", 5)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.Path == "wine/tannin.md" {
			t.Error("Related should exclude the node itself")
		}
	}
	if _, err := Related(s, "wine/nonexistent.md", 5); err == nil {
		t.Error("Related on unknown path should error")
	}
}
