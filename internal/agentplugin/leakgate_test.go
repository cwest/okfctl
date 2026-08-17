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
	"os"
	"path/filepath"
	"testing"
)

// writeSkill writes a SKILL.md with the given metadata.hermes.sharing marker
// (the repo convention) under dir/<name>/SKILL.md. A sharing of "" writes no
// marker at all, exercising the fail-closed path.
func writeSkill(t *testing.T, dir, name, sharing string) {
	t.Helper()
	sd := filepath.Join(dir, name)
	if err := os.MkdirAll(sd, 0o755); err != nil {
		t.Fatal(err)
	}
	fm := "name: " + name + "\nmetadata:\n  hermes:\n"
	if sharing != "" {
		fm += "    sharing: " + sharing + "\n"
	}
	content := "---\n" + fm + "---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(sd, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The leak gate keeps the two products separate: the packaged plugin ships ONLY
// the generic, shareable skills. A migration-internal skill (sharing other than
// "shareable", or no sharing marker at all) must be caught before it ships.

func TestAuditSkillPayload_AllShareablePasses(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "okf-authoring", "shareable")
	writeSkill(t, dir, "okf-curation-health", "shareable")

	leaks, err := AuditSkillPayload(dir)
	if err != nil {
		t.Fatalf("audit of an all-shareable payload must not error: %v", err)
	}
	if len(leaks) != 0 {
		t.Fatalf("an all-shareable payload must be leak-free; got leaks: %v", leaks)
	}
}

func TestAuditSkillPayload_NonShareableIsLeak(t *testing.T) {
	// POSITIVE CONTROL: a skill marked private/internal MUST be flagged.
	dir := t.TempDir()
	writeSkill(t, dir, "okf-authoring", "shareable")
	writeSkill(t, dir, "okf-migration-internal", "private")

	leaks, err := AuditSkillPayload(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(leaks) != 1 || leaks[0] != "okf-migration-internal" {
		t.Fatalf("a non-shareable skill MUST be flagged as a leak; got: %v", leaks)
	}
}

func TestAuditSkillPayload_MissingSharingIsLeak(t *testing.T) {
	// A skill with no sharing marker is not provably shippable — fail closed.
	dir := t.TempDir()
	writeSkill(t, dir, "okf-unmarked", "")

	leaks, err := AuditSkillPayload(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(leaks) != 1 || leaks[0] != "okf-unmarked" {
		t.Fatalf("a skill with no sharing marker MUST be flagged; got: %v", leaks)
	}
}

// TestAuditSkillPayload_ShippedSkillsAreClean is the real-payload assertion:
// the actual skills/ directory this repo ships must pass the leak gate. This is
// the negative control at real scale — the shipped set is all-shareable, so it
// must stay silent.
func TestAuditSkillPayload_ShippedSkillsAreClean(t *testing.T) {
	skillsDir := filepath.Join("..", "..", "skills")
	if _, err := os.Stat(skillsDir); err != nil {
		t.Skipf("skills/ not found at %s: %v", skillsDir, err)
	}
	leaks, err := AuditSkillPayload(skillsDir)
	if err != nil {
		t.Fatalf("audit of the shipped skills/ must not error: %v", err)
	}
	if len(leaks) != 0 {
		t.Fatalf("the shipped skills/ payload must be leak-free; got leaks: %v", leaks)
	}
}
