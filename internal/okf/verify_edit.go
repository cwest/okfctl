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
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ValidActor reports whether s is one of the three §7 actor forms:
//
//   - `<producer>/<version>` for agents and tools (e.g. reference_agent/gemini-2.5-pro)
//   - `human:<id>` for a person (e.g. human:ahormati)
//   - `process:<id>` for an automated process (e.g. process:finance-nightly)
//
// It is deliberately strict about the FORM (a bare id, an empty id, or the
// "--by me" anti-pattern are rejected) but opaque about the id's contents: §7
// leaves the id free-form, so ValidActor never validates beyond non-emptiness of
// the prefix's operand or the producer/version halves. A tool that guesses at
// who is a tool that manufactures trust — this gate refuses to invent an actor.
func ValidActor(s string) bool {
	switch {
	case strings.HasPrefix(s, "human:"):
		return len(strings.TrimPrefix(s, "human:")) > 0
	case strings.HasPrefix(s, "process:"):
		return len(strings.TrimPrefix(s, "process:")) > 0
	default:
		// `<producer>/<version>`: exactly one slash, both halves non-empty.
		producer, version, ok := strings.Cut(s, "/")
		return ok && producer != "" && version != "" && !strings.Contains(version, "/")
	}
}

// AppendVerifiedFile APPENDS a §5.2 verification event { by, at } to the node at
// abs, writing the file back in place. It is the write-side companion of
// (*Node).Verified() and is built on the same order- and body-preserving
// machinery as TouchModifiedFile: the frontmatter block is round-tripped through
// a yaml.Node so existing keys keep their order and the Markdown body is
// preserved verbatim.
//
// The four card-mandated safety properties:
//
//   - APPEND, never replace (§5.2 models verified as a list because verification
//     history IS history): an existing verified list is extended, a prior entry
//     is never modified, and created/modified are never touched.
//   - A BARE verified MAPPING is normalized to a list per §5.2's one-element-list
//     rule (the prior verifier becomes element 0, the new event element 1) rather
//     than corrupted.
//   - A node with no verified key gains a one-element verified list.
//   - A node with no frontmatter at all gains a minimal block carrying just the
//     verified event (Validate still flags a missing type — that is the floor's
//     job, not this write's).
//
// `by` is written verbatim; callers validate it against ValidActor first. `at`
// is stamped RFC3339 UTC.
func AppendVerifiedFile(abs, by string, at time.Time) error {
	// abs is the user's node file being stamped; reading it is intended.
	raw, err := os.ReadFile(abs) //nolint:gosec // G304: reading the user's own bundle node
	if err != nil {
		return fmt.Errorf("read %s: %w", abs, err)
	}
	stamp := at.UTC().Format(timestampLayout)

	yamlBlock, rawAfter, ok := splitFrontmatterRaw(raw)
	if !ok {
		// No frontmatter block: prepend a minimal one carrying just verified.
		// (Validate will still flag a missing type — that is the floor's job.)
		var out bytes.Buffer
		out.WriteString("---\nverified:\n  - by: " + by + "\n    at: " + stamp + "\n---\n")
		out.Write(raw)
		// A bundle node is a shareable knowledge document; 0o644 is intended.
		return os.WriteFile(abs, out.Bytes(), 0o644) //nolint:gosec // G306: shareable bundle content file
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(yamlBlock, &doc); err != nil {
		return fmt.Errorf("parse frontmatter of %s: %w", abs, err)
	}
	root := frontmatterMapping(&doc)
	if root == nil {
		return fmt.Errorf("frontmatter of %s is not a mapping", abs)
	}
	appendVerified(root, by, stamp)

	var fmBuf bytes.Buffer
	enc := yaml.NewEncoder(&fmBuf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return fmt.Errorf("encode frontmatter of %s: %w", abs, err)
	}
	_ = enc.Close()

	var out bytes.Buffer
	out.WriteString("---\n")
	out.Write(fmBuf.Bytes())
	out.WriteString("---\n")
	// rawAfter is the body region verbatim (including any blank separator line),
	// so the only bytes that change are inside the frontmatter block.
	out.Write(rawAfter)
	// A bundle node is a shareable knowledge document; 0o644 is intended.
	return os.WriteFile(abs, out.Bytes(), 0o644) //nolint:gosec // G306: shareable bundle content file
}

// appendVerified appends a { by, at } event to the mapping's `verified` value,
// preserving key order. It handles the three §5.2 shapes:
//
//   - absent: a new sequence with the one event is appended to the mapping.
//   - a sequence: the event is appended to it (prior entries untouched).
//   - a bare mapping (§5.2 one-element-list rule): the value is promoted to a
//     two-element sequence with the prior mapping first, then the new event —
//     the prior verifier is never dropped.
//
// Any other scalar value (a malformed verified) is treated like absent-but-keep:
// it is replaced by a one-element sequence, which is the recoverable choice (the
// scalar could never have parsed as an event anyway).
func appendVerified(mapping *yaml.Node, by, stamp string) {
	event := verifiedEventNode(by, stamp)
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != "verified" {
			continue
		}
		cur := mapping.Content[i+1]
		switch cur.Kind {
		case yaml.SequenceNode:
			cur.Content = append(cur.Content, event)
		case yaml.MappingNode:
			// §5.2: a bare mapping is a one-element list. Promote it, keeping
			// the prior verifier as the first element.
			prior := *cur
			seq := &yaml.Node{Kind: yaml.SequenceNode}
			seq.Content = append(seq.Content, &prior, event)
			mapping.Content[i+1] = seq
		default:
			// Unusable scalar/null: replace with a fresh one-element sequence.
			seq := &yaml.Node{Kind: yaml.SequenceNode}
			seq.Content = append(seq.Content, event)
			mapping.Content[i+1] = seq
		}
		return
	}
	// verified is absent: append it as a new one-element sequence.
	seq := &yaml.Node{Kind: yaml.SequenceNode}
	seq.Content = append(seq.Content, event)
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: "verified"},
		seq,
	)
}

// verifiedEventNode builds a { by, at } mapping node for one verification event.
func verifiedEventNode(by, stamp string) *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "by"},
			{Kind: yaml.ScalarNode, Value: by},
			{Kind: yaml.ScalarNode, Value: "at"},
			{Kind: yaml.ScalarNode, Value: stamp},
		},
	}
}
