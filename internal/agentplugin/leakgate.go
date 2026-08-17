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

package agentplugin

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// shareableMarker is the frontmatter `sharing` value a skill must carry to be
// shippable in the packaged plugin. The repo keeps two products separate — the
// generic, shareable skills and the migration-internal ones — and only the
// former go in the artifact. A skill that is not provably shareable is a leak.
const shareableMarker = "shareable"

// skillFrontmatter is the subset of a SKILL.md YAML frontmatter the leak gate
// reads. The `sharing` marker governs whether the skill may ship. In this repo
// it lives at metadata.hermes.sharing (the Hermes skill convention); a
// top-level `sharing` is also honored as a fallback.
type skillFrontmatter struct {
	Sharing  string `yaml:"sharing"`
	Metadata struct {
		Hermes struct {
			Sharing string `yaml:"sharing"`
		} `yaml:"hermes"`
	} `yaml:"metadata"`
}

// sharing returns the effective sharing marker, preferring the top-level field
// and falling back to metadata.hermes.sharing.
func (f skillFrontmatter) sharing() string {
	if f.Sharing != "" {
		return f.Sharing
	}
	return f.Metadata.Hermes.Sharing
}

// AuditSkillPayload scans a directory of skills (each a <name>/SKILL.md) and
// returns the sorted names of any that are NOT marked shippable — i.e. whose
// frontmatter `sharing` is anything other than "shareable", including absent.
// It fails closed: an unmarked skill is a leak, because a plugin that bundles
// skills/ wholesale would otherwise ship a migration-internal skill by accident.
//
// The returned slice is empty (not nil-panicking) when the payload is clean.
func AuditSkillPayload(skillsDir string) ([]string, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, fmt.Errorf("read skills dir: %w", err)
	}

	var leaks []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillMD := filepath.Join(skillsDir, e.Name(), "SKILL.md")
		fm, err := readSkillFrontmatter(skillMD)
		if err != nil {
			// A directory under skills/ with no readable SKILL.md is not a
			// shippable skill; fail closed and flag it rather than skipping.
			leaks = append(leaks, e.Name())
			continue
		}
		if fm.sharing() != shareableMarker {
			leaks = append(leaks, e.Name())
		}
	}
	sort.Strings(leaks)
	return leaks, nil
}

// readSkillFrontmatter parses the leading `---`-delimited YAML block of a
// SKILL.md file into a skillFrontmatter.
func readSkillFrontmatter(path string) (skillFrontmatter, error) {
	var fm skillFrontmatter
	raw, err := os.ReadFile(path) //nolint:gosec // G304: path is a repo skill file under a caller-supplied dir
	if err != nil {
		return fm, err
	}
	body, err := frontmatterBlock(raw)
	if err != nil {
		return fm, err
	}
	if err := yaml.Unmarshal(body, &fm); err != nil {
		return fm, fmt.Errorf("parse frontmatter %s: %w", path, err)
	}
	return fm, nil
}

// frontmatterBlock extracts the bytes between the opening `---` and the next
// `---` line. It returns an error when no frontmatter block is present.
func frontmatterBlock(raw []byte) ([]byte, error) {
	const delim = "---"
	s := string(raw)
	// The file must open with the delimiter (allowing a leading BOM/newline is
	// out of scope; SKILL.md files in this repo start with `---`).
	if len(s) < 3 || s[:3] != delim {
		return nil, fmt.Errorf("no frontmatter delimiter at start of file")
	}
	rest := s[3:]
	// Find the closing delimiter on its own line.
	idx := indexClosingDelim(rest)
	if idx < 0 {
		return nil, fmt.Errorf("unterminated frontmatter block")
	}
	return []byte(rest[:idx]), nil
}

// indexClosingDelim returns the offset within s of the closing `---` line, or
// -1 if none is found.
func indexClosingDelim(s string) int {
	for i := 0; i < len(s); i++ {
		// Look for a newline followed by "---" at a line start.
		if s[i] == '\n' {
			line := s[i+1:]
			if len(line) >= 3 && line[:3] == "---" {
				return i + 1
			}
		}
	}
	return -1
}
