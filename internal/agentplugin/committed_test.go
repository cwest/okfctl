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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// repoPluginJSON is the path to the committed root manifest, relative to this
// package.
func repoPluginJSON() string {
	return filepath.Join("..", "..", "plugin.json")
}

// TestCommittedManifest_IsSchemaValid ties the offline validator to the real
// artifact: the root plugin.json this repo ships MUST validate against the
// Agent Plugins 1.0.0 rules. This is the fast, always-on companion to the CI
// job that validates the same file against the pinned schema URL.
func TestCommittedManifest_IsSchemaValid(t *testing.T) {
	raw, err := os.ReadFile(repoPluginJSON())
	if err != nil {
		t.Fatalf("root plugin.json must exist and be readable: %v", err)
	}
	if err := ValidateManifest(raw); err != nil {
		t.Fatalf("committed plugin.json must be Agent Plugins 1.0.0 conformant: %v", err)
	}
}

// TestCommittedManifest_HasNoHandMaintainedVersion enforces the single-source
// version rule: the committed manifest must NOT carry a hand-typed "version"
// string. The authoritative version is the release tag (surfaced by
// `okfctl version`); a second hand-maintained copy here would drift. `version`
// is optional in the schema, so omitting it stays fully conformant.
func TestCommittedManifest_HasNoHandMaintainedVersion(t *testing.T) {
	raw, err := os.ReadFile(repoPluginJSON())
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if _, present := m["version"]; present {
		t.Error("committed plugin.json must not hand-maintain a version string; " +
			"the release tag is the single source of truth")
	}
}

// TestCommittedManifest_NestsConfigUnderReverseDomainExtension asserts the
// okfctl-specific payload lives under extensions."dev.okfctl", never as a
// bespoke top-level key (the spec's extension mechanism).
func TestCommittedManifest_NestsConfigUnderReverseDomainExtension(t *testing.T) {
	raw, err := os.ReadFile(repoPluginJSON())
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Extensions map[string]json.RawMessage `json:"extensions"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Extensions["dev.okfctl"]; !ok {
		t.Error(`committed plugin.json must nest okfctl config under extensions."dev.okfctl"`)
	}
}

// TestInvalidFixture_FailsOfflineValidator is the offline negative control:
// the same malformed manifest the CI schema gate rejects must also fail the
// offline validator. A validator that passes everything is decoration.
func TestInvalidFixture_FailsOfflineValidator(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "invalid-plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateManifest(raw); err == nil {
		t.Fatal("the malformed negative-control fixture MUST fail validation")
	}
}

// TestPackagedArtifact_IsLeakFree runs the leak gate over the exact file set a
// plugin client receives — the committed plugin.json's declared skills, resolved
// against skills/. Every declared skill must exist and be shareable, and no
// shippable-but-undeclared migration skill may sit in skills/. This is the
// "gate on the PACKAGED artifact, not the source tree" the repo contract
// requires.
func TestPackagedArtifact_IsLeakFree(t *testing.T) {
	// Resolve the declared skill list from the committed manifest.
	raw, err := os.ReadFile(repoPluginJSON())
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Extensions struct {
			OKFctl struct {
				Skills []string `json:"skills"`
			} `json:"dev.okfctl"`
		} `json:"extensions"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	declared := m.Extensions.OKFctl.Skills
	if len(declared) == 0 {
		t.Fatal("manifest declares no skills under extensions.dev.okfctl.skills")
	}

	skillsDir := filepath.Join("..", "..", "skills")

	// Every declared skill must exist on disk and pass the leak gate.
	leaks, err := AuditSkillPayload(skillsDir)
	if err != nil {
		t.Fatalf("audit of skills/: %v", err)
	}
	if len(leaks) != 0 {
		t.Fatalf("packaged skills contain leaks (non-shareable/unmarked): %v", leaks)
	}

	// The manifest must declare exactly the shareable set on disk — no declared
	// skill missing from disk, and no shareable skill on disk left undeclared.
	onDisk := map[string]bool{}
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			onDisk[e.Name()] = true
		}
	}
	declaredSet := map[string]bool{}
	for _, s := range declared {
		declaredSet[s] = true
		if !onDisk[s] {
			t.Errorf("manifest declares skill %q that does not exist under skills/", s)
		}
	}
	for name := range onDisk {
		if !declaredSet[name] {
			t.Errorf("skills/%s exists but is not declared in the manifest; "+
				"a shippable skill left undeclared could hide a leak", name)
		}
	}
}
