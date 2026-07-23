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
	"bytes"
	"path/filepath"
	"testing"
)

func runOKF(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestValidateCmd_GoodBundleExitsZero(t *testing.T) {
	dir := filepath.Join("..", "testdata", "good-bundle")
	if _, err := runOKF(t, "validate", dir); err != nil {
		t.Fatalf("validate good bundle returned error (nonzero exit): %v", err)
	}
}

func TestValidateCmd_MissingTypeExitsNonZero(t *testing.T) {
	dir := filepath.Join("..", "testdata", "no-type")
	out, err := runOKF(t, "validate", dir)
	if err == nil {
		t.Fatalf("validate no-type must return error (nonzero exit); out=%q", out)
	}
}
