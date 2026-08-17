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

// Package agentplugin validates okfctl's Agent Plugins 1.0.0 packaging manifest
// (the root plugin.json) and the skill payload it ships.
//
// This is the "packaging" spec — Agent Plugins 1.0.0, published at
// https://agent-plugins.org — and is distinct from internal/plugin, which
// discovers git/kubectl-style okfctl-<name> executable plugins on PATH. The two
// share the word "plugin" and nothing else.
//
// The authoritative validation is the pinned JSON-Schema, run in CI against the
// live schema URL (see .github/workflows/ci.yml). This package re-encodes the
// same 1.0.0 rules as an offline, dependency-free Go check so `go test ./...`
// exercises the manifest without a network round-trip. When the two disagree,
// the pinned schema wins; keep this file in lockstep with it.
package agentplugin

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// SchemaURL is the canonical, pinned Agent Plugins 1.0.0 manifest schema
// identifier. The schema declares this as a `const`, so any other value is a
// hard failure.
const SchemaURL = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"

// namePattern mirrors the schema's `name` pattern verbatim:
//
//	^(?!.*(?:--|\.\.))[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$
//
// Go's regexp (RE2) has no lookahead, so the "no -- and no .." rule is enforced
// separately in validateName; this pattern covers the character-class and
// anchor requirements.
var namePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)

var forbiddenNameRun = regexp.MustCompile(`--|\.\.`)

// knownTopLevelKeys is the closed set the schema allows at the root
// (additionalProperties: false). Any key outside this set is rejected.
var knownTopLevelKeys = map[string]bool{
	"$schema":     true,
	"name":        true,
	"version":     true,
	"description": true,
	"author":      true,
	"homepage":    true,
	"repository":  true,
	"license":     true,
	"keywords":    true,
	"extensions":  true,
}

// ValidateManifest checks raw plugin.json bytes against the Agent Plugins 1.0.0
// schema rules that can be enforced offline: required keys, the $schema const,
// the name pattern, the closed top-level key set, and the extensions shape.
func ValidateManifest(raw []byte) error {
	// Decode into a generic map so we can enforce additionalProperties: false —
	// a struct with json tags would silently drop unknown keys.
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("plugin.json is not valid JSON: %w", err)
	}

	// required: ["$schema", "name"]
	if _, ok := m["$schema"]; !ok {
		return fmt.Errorf("plugin.json is missing the required $schema key")
	}
	if _, ok := m["name"]; !ok {
		return fmt.Errorf("plugin.json is missing the required name key")
	}

	// additionalProperties: false
	for k := range m {
		if !knownTopLevelKeys[k] {
			return fmt.Errorf("plugin.json has unknown top-level key %q; "+
				"okfctl-specific config must live under extensions", k)
		}
	}

	// $schema const
	var schema string
	if err := json.Unmarshal(m["$schema"], &schema); err != nil {
		return fmt.Errorf("plugin.json $schema is not a string: %w", err)
	}
	if schema != SchemaURL {
		return fmt.Errorf("plugin.json $schema is %q; it must be the pinned 1.0.0 URL %q", schema, SchemaURL)
	}

	// name pattern
	var name string
	if err := json.Unmarshal(m["name"], &name); err != nil {
		return fmt.Errorf("plugin.json name is not a string: %w", err)
	}
	if err := validateName(name); err != nil {
		return err
	}

	// extensions: object of objects
	if rawExt, ok := m["extensions"]; ok {
		if err := validateExtensions(rawExt); err != nil {
			return err
		}
	}

	return nil
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("plugin.json name must be non-empty")
	}
	if len(name) > 64 {
		return fmt.Errorf("plugin.json name %q exceeds the 64-char maximum", name)
	}
	if forbiddenNameRun.MatchString(name) {
		return fmt.Errorf("plugin.json name %q must not contain a %q or %q run", name, "--", "..")
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("plugin.json name %q violates the schema pattern "+
			"(lowercase alphanumerics, dots and dashes, no leading/trailing separator)", name)
	}
	return nil
}

func validateExtensions(raw json.RawMessage) error {
	var ext map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ext); err != nil {
		return fmt.Errorf("plugin.json extensions is not an object: %w", err)
	}
	for ns, val := range ext {
		// additionalProperties: { type: object } — each namespace value is an object.
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(val, &obj); err != nil {
			return fmt.Errorf("plugin.json extensions[%q] must be an object: %w", ns, err)
		}
	}
	return nil
}

// jsonString quotes s as a JSON string literal. Test helper kept in the
// non-test file so both test files can use it without duplication.
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
