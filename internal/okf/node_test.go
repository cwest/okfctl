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
	"reflect"
	"testing"
)

// TestNode_Tags exercises the exported §4.1 tags accessor. Tags are an optional
// YAML list; a scalar tag is a one-element list; non-string scalars coerce to
// their string form (a bare integer is still a real tag); absence yields nil.
func TestNode_Tags(t *testing.T) {
	cases := []struct {
		name string
		fm   map[string]any
		want []string
	}{
		{"absent", map[string]any{}, nil},
		{"list", map[string]any{"tags": []any{"wine", "red"}}, []string{"wine", "red"}},
		{"scalar", map[string]any{"tags": "wine"}, []string{"wine"}},
		{"numeric", map[string]any{"tags": []any{403, "http"}}, []string{"403", "http"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := &Node{Frontmatter: tc.fm}
			if got := n.Tags(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Tags() = %#v, want %#v", got, tc.want)
			}
		})
	}
}
