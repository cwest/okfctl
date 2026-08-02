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
	"os"
	"path/filepath"
	"strings"
)

// DriftIgnoreRevsFile is the bundle-root file naming the mechanical commit SHAs
// that opt out of git drift. It mirrors the file `git blame --ignore-revs-file`
// consumes: one SHA per line, blank lines and #-comments ignored. A checked-in
// list lets a human declare commit INTENT that git itself cannot read — the
// day-one bulk migration commit that would otherwise collapse a corpus's real
// authoring history into a single date.
const DriftIgnoreRevsFile = ".okf-drift-ignore-revs"

// LoadDriftIgnoreRevs reads DriftIgnoreRevsFile from the bundle root and returns
// the set of commit SHAs to opt out of drift comparison. The file is OPTIONAL:
// its absence yields an empty (non-nil) set and no error — the common case.
//
// Format (identical to git's ignore-revs file so users already understand it):
//   - one SHA per line
//   - blank lines ignored
//   - a line beginning with '#' is a comment, ignored
//   - an inline '#' comment after a SHA is stripped
//   - surrounding whitespace trimmed
//
// SHAs are lower-cased so matching is case-insensitive against git's %H output.
func LoadDriftIgnoreRevs(root string) (map[string]bool, error) {
	out := map[string]bool{}
	data, err := os.ReadFile(filepath.Join(root, DriftIgnoreRevsFile))
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil // optional file: absence is not an error
		}
		return nil, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip an inline comment: "abc123  # the key sweep".
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		// Take the first whitespace-delimited token as the SHA, tolerating any
		// trailing annotation.
		if f := strings.Fields(line); len(f) > 0 {
			out[strings.ToLower(f[0])] = true
		}
	}
	return out, nil
}
