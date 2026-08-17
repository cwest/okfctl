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
	"strings"
	"testing"
)

// A minimal, spec-valid manifest: the two required keys and nothing else.
// Agent Plugins 1.0.0 requires only "$schema" and "name".
const minimalManifest = `{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "okfctl"
}`

func TestValidateManifest_MinimalConformantPasses(t *testing.T) {
	if err := ValidateManifest([]byte(minimalManifest)); err != nil {
		t.Fatalf("a minimal spec-conformant manifest must validate; got: %v", err)
	}
}

func TestValidateManifest_MissingSchemaFails(t *testing.T) {
	// $schema is required (schema `required: ["$schema", "name"]`).
	m := `{"name": "okfctl"}`
	err := ValidateManifest([]byte(m))
	if err == nil {
		t.Fatal("a manifest with no $schema MUST fail — $schema is required")
	}
	if !strings.Contains(err.Error(), "$schema") {
		t.Errorf("error should name the missing $schema key; got: %v", err)
	}
}

func TestValidateManifest_WrongSchemaConstFails(t *testing.T) {
	// $schema is a const: only the pinned 1.0.0 URL is valid.
	m := `{
	  "$schema": "https://agent-plugins.org/schemas/9.9.9/plugin.schema.json",
	  "name": "okfctl"
	}`
	if err := ValidateManifest([]byte(m)); err == nil {
		t.Fatal("a manifest pinning a non-1.0.0 schema URL MUST fail — $schema is a const")
	}
}

func TestValidateManifest_MissingNameFails(t *testing.T) {
	m := `{"$schema": "` + SchemaURL + `"}`
	err := ValidateManifest([]byte(m))
	if err == nil {
		t.Fatal("a manifest with no name MUST fail — name is required")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should name the missing name key; got: %v", err)
	}
}

func TestValidateManifest_BadNamePatternFails(t *testing.T) {
	// name pattern: ^(?!.*(?:--|\.\.))[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$
	// Uppercase, a leading dash, and a "--" run are all rejected by the schema.
	for _, bad := range []string{"Okfctl", "-okfctl", "okf--ctl", "okfctl-", "okf..ctl", ""} {
		m := `{"$schema": "` + SchemaURL + `", "name": ` + jsonString(bad) + `}`
		if err := ValidateManifest([]byte(m)); err == nil {
			t.Errorf("name %q violates the schema pattern and MUST fail", bad)
		}
	}
}

func TestValidateManifest_GoodNamePatternPasses(t *testing.T) {
	for _, good := range []string{"okfctl", "okf-ctl", "a", "okf.ctl", "okfctl2"} {
		m := `{"$schema": "` + SchemaURL + `", "name": ` + jsonString(good) + `}`
		if err := ValidateManifest([]byte(m)); err != nil {
			t.Errorf("name %q satisfies the schema pattern and must pass; got: %v", good, err)
		}
	}
}

func TestValidateManifest_UnknownTopLevelKeyFails(t *testing.T) {
	// The schema is `additionalProperties: false` at the root: any okfctl-specific
	// config MUST live under `extensions`, never as a bespoke top-level key.
	m := `{
	  "$schema": "` + SchemaURL + `",
	  "name": "okfctl",
	  "skills": ["okf-authoring"]
	}`
	err := ValidateManifest([]byte(m))
	if err == nil {
		t.Fatal("an unknown top-level key MUST fail — root additionalProperties is false")
	}
	if !strings.Contains(err.Error(), "skills") {
		t.Errorf("error should name the offending key; got: %v", err)
	}
}

func TestValidateManifest_ExtensionsReverseDomainPasses(t *testing.T) {
	// okfctl-specific config nested under a reverse-domain extension key is the
	// spec's sanctioned escape hatch and MUST validate.
	m := `{
	  "$schema": "` + SchemaURL + `",
	  "name": "okfctl",
	  "extensions": {
	    "dev.okfctl": { "skills": ["okf-authoring"] }
	  }
	}`
	if err := ValidateManifest([]byte(m)); err != nil {
		t.Fatalf("extensions with a reverse-domain key must validate; got: %v", err)
	}
}

func TestValidateManifest_ExtensionsNonObjectValueFails(t *testing.T) {
	// extensions.additionalProperties requires each namespace value to be an object.
	m := `{
	  "$schema": "` + SchemaURL + `",
	  "name": "okfctl",
	  "extensions": { "dev.okfctl": "not-an-object" }
	}`
	if err := ValidateManifest([]byte(m)); err == nil {
		t.Fatal("a non-object extension namespace value MUST fail")
	}
}

func TestValidateManifest_MalformedJSONFails(t *testing.T) {
	if err := ValidateManifest([]byte(`{ not json`)); err == nil {
		t.Fatal("malformed JSON MUST fail to validate")
	}
}
