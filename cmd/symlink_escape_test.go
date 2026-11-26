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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cwest/okfctl/internal/okf"
)

const outsideSecret = "OUTSIDE-SECRET-BODY"

// escapeFixture writes a bundle with one real node, plus a directory OUTSIDE the
// bundle root holding a concept and a log that must never be read through it.
func escapeFixture(t *testing.T) (root, outside string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "bundle")
	outside = filepath.Join(base, "outside")
	for rel, body := range map[string]string{
		filepath.Join(root, "a.md"):           "---\ntype: Reference\ntitle: A\n---\n# A\n",
		filepath.Join(outside, "secret.md"):   "---\ntype: Secret\ntitle: Secret\n---\n" + outsideSecret + "\n",
		filepath.Join(outside, "log.md"):      "# Log\n" + outsideSecret + "\n",
		filepath.Join(outside, "sub/more.md"): "---\ntype: Secret\n---\n" + outsideSecret + "\n",
	} {
		mustWrite(t, rel, body)
	}
	return root, outside
}

func mustWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}
}

// Positive control (§3.2 rule 5, §11.2): validate warns, naming each escaping
// path, and still exits 0 — the bundle stays conformant (§11.1).
func TestValidateCmd_Section3_2_EscapingSymlinkWarnsNotFails(t *testing.T) {
	root, outside := escapeFixture(t)
	mustSymlink(t, filepath.Join(outside, "secret.md"), filepath.Join(root, "linked.md"))
	mustSymlink(t, filepath.Join(outside, "sub"), filepath.Join(root, "ext"))

	out, err := runOKF(t, "validate", root)
	if err != nil {
		t.Fatalf("validate must not fail on an escaping symlink: %v\n%s", err, out)
	}
	for _, p := range []string{"linked.md", "ext"} {
		if !strings.Contains(out, "warning "+p+": "+okf.EscapedSymlinkMessage) {
			t.Errorf("validate output lacks the warning for %s:\n%s", p, out)
		}
	}
	if !strings.Contains(out, "did not read 2 symlinks that resolve outside the bundle root (ext, linked.md)") {
		t.Errorf("stderr note missing:\n%s", out)
	}
}

// Negative control (§3.2 rule 5): an in-root symlink draws no warning and loads.
func TestValidateCmd_Section3_2_InRootSymlinkSilent(t *testing.T) {
	root, _ := escapeFixture(t)
	mustSymlink(t, "a.md", filepath.Join(root, "alias.md"))

	out, err := runOKF(t, "validate", root)
	if err != nil {
		t.Fatalf("validate: %v\n%s", err, out)
	}
	if strings.Contains(out, "symlink") {
		t.Errorf("in-root symlink drew a symlink warning:\n%s", out)
	}
	gout, err := runOKF(t, "graph", "export", root)
	if err != nil {
		t.Fatalf("graph export: %v", err)
	}
	if !strings.Contains(gout, "alias.md") {
		t.Errorf("in-root symlink missing from the graph:\n%s", gout)
	}
}

// Positive control (§11.2): graph export, serve, and lint never surface content
// read through an escaping symlink.
func TestEntryPoints_Section11_2_EscapingSymlinkContentNeverSurfaces(t *testing.T) {
	root, outside := escapeFixture(t)
	mustSymlink(t, filepath.Join(outside, "secret.md"), filepath.Join(root, "linked.md"))
	mustSymlink(t, filepath.Join(outside, "sub"), filepath.Join(root, "ext"))

	for _, args := range [][]string{
		{"graph", "export", root},
		{"graph", "export", root, "--format", "dot"},
		{"lint", root},
		{"analyze", root},
	} {
		out, _ := runOKF(t, args...)
		if strings.Contains(out, outsideSecret) || strings.Contains(out, "Secret") {
			t.Errorf("%v surfaced out-of-root content:\n%s", args, out)
		}
	}

	b, err := okf.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	newServeHandler(b).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/graph.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /graph.json = %d", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, "linked.md") || strings.Contains(body, "Secret") {
		t.Errorf("serve /graph.json surfaced out-of-root content:\n%s", body)
	}
}

// Positive control (§3.2 rule 5, §11.2): log show refuses a log.md that is a
// symlink out of the root rather than printing the outside file.
func TestLogShow_Section3_2_EscapingLogNotRead(t *testing.T) {
	root, outside := escapeFixture(t)
	mustSymlink(t, filepath.Join(outside, "log.md"), filepath.Join(root, "log.md"))

	out, err := runOKF(t, "log", "show", root)
	if err == nil {
		t.Fatalf("log show must refuse an escaping log.md; out=%q", out)
	}
	if strings.Contains(out, outsideSecret) {
		t.Errorf("log show printed out-of-root content:\n%s", out)
	}
}

// Negative control: an in-root log.md symlink still prints.
func TestLogShow_Section3_2_InRootLogSymlinkReads(t *testing.T) {
	root, _ := escapeFixture(t)
	mustWrite(t, filepath.Join(root, "history", "log-src.md"), "# Log\nIN-ROOT-LOG\n")
	mustSymlink(t, filepath.Join("history", "log-src.md"), filepath.Join(root, "log.md"))

	out, err := runOKF(t, "log", "show", root)
	if err != nil {
		t.Fatalf("log show: %v", err)
	}
	if !strings.Contains(out, "IN-ROOT-LOG") {
		t.Errorf("in-root log symlink not read:\n%s", out)
	}
}

// Positive control (§11.2): a bundle fetched with `connect` from a remote source
// can carry a committed symlink to an absolute local path. The connected bundle
// must not read it; validate warns and graph export stays clean.
func TestConnect_Section11_2_HostileSymlinkInRemoteBundleNotRead(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	mustWrite(t, filepath.Join(outside, "secret.md"), "---\ntype: Secret\n---\n"+outsideSecret+"\n")

	bare := seedBareBundleSource(t, root)
	work := filepath.Join(root, "seed-work")
	mustSymlink(t, filepath.Join(outside, "secret.md"), filepath.Join(work, "leak.md"))
	git(t, work, "add", "-A")
	git(t, work, "commit", "-qm", "add symlink")
	git(t, work, "push", "-q", "origin", "main")

	dst := filepath.Join(root, "local-kb")
	if out, err := runOKF(t, "connect", bare, dst); err != nil {
		t.Fatalf("connect: %v\n%s", err, out)
	}
	if fi, err := os.Lstat(filepath.Join(dst, "leak.md")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("fixture: leak.md should arrive as a symlink (err=%v)", err)
	}
	out, err := runOKF(t, "validate", dst)
	if err != nil {
		t.Fatalf("validate: %v\n%s", err, out)
	}
	if !strings.Contains(out, "warning leak.md: "+okf.EscapedSymlinkMessage) {
		t.Errorf("validate did not warn on the hostile symlink:\n%s", out)
	}
	gout, _ := runOKF(t, "graph", "export", dst)
	// The stderr note names leak.md on purpose; the graph JSON must not.
	if strings.Contains(gout, `"leak.md"`) || strings.Contains(gout, "Secret") || strings.Contains(gout, outsideSecret) {
		t.Errorf("graph export surfaced the hostile symlink:\n%s", gout)
	}
}
