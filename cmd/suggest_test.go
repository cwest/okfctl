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

package cmd

import (
	"strings"
	"testing"
)

// Spec (docs/specs/2026-07-24-plugin-dispatch.md, "not found" branch): the
// did-you-mean suggestion set must be drawn from built-ins AND discovered
// plugins. A typo of a PLUGIN name must surface that plugin as a suggestion —
// cobra's native suggester only knows built-ins, so this is the gap this test
// pins.
func TestSuggest_TypoOfPluginNameSuggestsPlugin(t *testing.T) {
	dir := t.TempDir()
	mkPluginStub(t, dir, "hello", "echo hi")

	err := unknownCommandError(NewRootCmd(), []string{"helo", "there"}, dir)
	if err == nil {
		t.Fatalf("typo of a plugin name should produce an unknown-command error")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "helo") {
		t.Errorf("error should name the unknown command %q; got %q", "helo", err.Error())
	}
	if !strings.Contains(msg, "hello") {
		t.Errorf("error should suggest the near plugin %q (did-you-mean from plugins); got %q", "hello", err.Error())
	}
}

// A typo of a BUILT-IN must still suggest that built-in (regression guard on the
// existing behavior once suggestions are computed ourselves).
func TestSuggest_TypoOfBuiltinSuggestsBuiltin(t *testing.T) {
	dir := t.TempDir() // no plugins
	err := unknownCommandError(NewRootCmd(), []string{"valdate", "x"}, dir)
	if err == nil {
		t.Fatalf("typo of a built-in should produce an unknown-command error")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "valdate") {
		t.Errorf("error should name the unknown command; got %q", err.Error())
	}
	if !strings.Contains(msg, "validate") {
		t.Errorf("error should suggest 'validate'; got %q", err.Error())
	}
}

// An exact plugin name is NOT an error — it dispatches, so no suggestion error.
func TestSuggest_ExactPluginIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	mkPluginStub(t, dir, "hello", "echo hi")
	if err := unknownCommandError(NewRootCmd(), []string{"hello"}, dir); err != nil {
		t.Errorf("exact plugin name should not be an unknown-command error; got %v", err)
	}
}

// An exact built-in is NOT an error either.
func TestSuggest_ExactBuiltinIsNotAnError(t *testing.T) {
	dir := t.TempDir()
	if err := unknownCommandError(NewRootCmd(), []string{"validate"}, dir); err != nil {
		t.Errorf("exact built-in should not be an unknown-command error; got %v", err)
	}
}
