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
	"os"
	"path/filepath"
	"sort"
)

// ResolvedTemplates is the result of resolving a consumer bundle's effective
// type templates (PRD §9.2/§9.3): the winning template per target_type, plus the
// referenced templates that a local template shadowed. It generalizes the old
// one-bundle Templates(b) read so a knowledge bundle can REFERENCE a separate
// type-template bundle instead of vendoring it.
type ResolvedTemplates struct {
	// ByType is the effective template per target_type: a referenced bundle's
	// templates folded in first, then local `Type Template` nodes overlaid on
	// top (local wins per target_type). Each entry carries its Source.
	ByType map[string]Template
	// Shadowed holds the referenced templates that a local template overrode,
	// sorted by target type. It lets `template list` flag a referenced entry a
	// local one supersedes (§9.4), rather than silently dropping it.
	Shadowed []Template
}

// ResolveTemplates computes a consumer bundle's effective type templates (PRD
// §9.2). The referenced type-template bundle is chosen by, in order:
//
//   - templatesFrom, the --templates-from flag value, when non-empty (used as a
//     direct directory path); it OVERRIDES the sidecar.
//   - the bundle's .okf `templates:` key, resolved relative to the bundle root.
//   - neither: no referenced bundle (the silent direction — only local templates
//     resolve, byte-identical to the historical single-bundle read).
//
// The referenced bundle is loaded with okf.Load on its OWN root, so it is never
// part of the consumer's node set and is never written. Its templates are folded
// first (Source = referenced path + in-bundle path); then the consumer's local
// `Type Template` nodes are overlaid, and local wins per target_type. A referenced
// template a local one overrides is recorded in Shadowed, not dropped.
//
// An unloadable reference (a path that is not a readable directory) is an error
// naming the offending path (PRD §9.2 AC5): a referenced convention that cannot
// be read is a hard failure for the template commands, never a silent empty set.
func ResolveTemplates(b *Bundle, templatesFrom string) (ResolvedTemplates, error) {
	out := ResolvedTemplates{ByType: map[string]Template{}}

	// 1) Pick the referenced path: flag overrides sidecar. Remember the display
	//    form (what the user wrote) for Source + errors, and the filesystem
	//    path to actually load.
	var refDisplay, refLoad string
	switch {
	case templatesFrom != "":
		refDisplay = templatesFrom
		refLoad = templatesFrom
	default:
		if key := readTemplatesRef(b.Root); key != "" {
			refDisplay = key
			refLoad = filepath.Join(b.Root, filepath.FromSlash(key))
		}
	}

	// 2) Fold the referenced bundle's templates first (if any), each tagged with
	//    its source (referenced path + the template's in-bundle path).
	if refLoad != "" {
		info, err := os.Stat(refLoad)
		if err != nil || !info.IsDir() {
			return ResolvedTemplates{}, fmt.Errorf("templates reference %q is not a readable bundle directory", refDisplay)
		}
		rb, err := Load(refLoad)
		if err != nil {
			return ResolvedTemplates{}, fmt.Errorf("load templates reference %q: %w", refDisplay, err)
		}
		for target, t := range Templates(rb) {
			t.Source = filepath.ToSlash(filepath.Join(refDisplay, t.Path))
			out.ByType[target] = t
		}
	}

	// 3) Overlay the consumer's local Type Template nodes; local wins per
	//    target_type, and a referenced template it supersedes is recorded as
	//    shadowed rather than discarded.
	for target, local := range Templates(b) {
		if ref, overridden := out.ByType[target]; overridden {
			out.Shadowed = append(out.Shadowed, ref)
		}
		local.Source = local.Path
		out.ByType[target] = local
	}

	sort.Slice(out.Shadowed, func(i, j int) bool {
		return out.Shadowed[i].TargetType < out.Shadowed[j].TargetType
	})
	return out, nil
}
