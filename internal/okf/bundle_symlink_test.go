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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// symlinkBundle builds a bundle root and a sibling directory OUTSIDE it holding
// a concept that must never be read through the bundle. It returns the bundle
// root and the outside directory.
func symlinkBundle(t *testing.T) (root, outside string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "bundle")
	outside = filepath.Join(base, "outside")
	writeFile(t, root, "index.md", "---\ntype: Index\n---\n# KB\n")
	writeFile(t, root, "real/a.md", "---\ntype: Reference\ntitle: A\n---\n# A\n")
	writeFile(t, outside, "secret.md", "---\ntype: Secret\ntitle: Secret\n---\nOUTSIDE-SECRET-BODY\n")
	return root, outside
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}
}

func assertNoSecret(t *testing.T, b *Bundle) {
	t.Helper()
	for p, n := range b.Nodes {
		if strings.Contains(n.Body, "OUTSIDE-SECRET-BODY") || n.Type() == "Secret" {
			t.Errorf("node %s carries content read from outside the bundle root", p)
		}
	}
	for p, n := range b.Reserved {
		if strings.Contains(n.Body, "OUTSIDE-SECRET-BODY") {
			t.Errorf("reserved %s carries content read from outside the bundle root", p)
		}
	}
}

// Positive control (§3.2 rule 5, §11.2): a symlinked .md whose real path is
// outside the bundle root is not read, and the skip is recorded by path.
func TestConformance_Section3_2_SymlinkedFileOutsideRootNotRead(t *testing.T) {
	root, outside := symlinkBundle(t)
	symlink(t, filepath.Join(outside, "secret.md"), filepath.Join(root, "linked.md"))
	// A relative target that climbs out of the root is the same escape.
	symlink(t, "../../outside/secret.md", filepath.Join(root, "sub/rel.md"))

	b, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := b.Nodes["linked.md"]; ok {
		t.Error("linked.md (escaping symlink) was loaded as a node")
	}
	if _, ok := b.Nodes["sub/rel.md"]; ok {
		t.Error("sub/rel.md (escaping relative symlink) was loaded as a node")
	}
	assertNoSecret(t, b)
	if want := []string{"linked.md", "sub/rel.md"}; !reflect.DeepEqual(b.EscapedSymlinks, want) {
		t.Errorf("EscapedSymlinks = %v, want %v", b.EscapedSymlinks, want)
	}
	if _, ok := b.Nodes["real/a.md"]; !ok {
		t.Error("a regular in-root node went missing")
	}
}

// Negative control (§3.2 rule 5): a symlinked .md that resolves INSIDE the root
// still loads and is not reported.
func TestConformance_Section3_2_SymlinkedFileInsideRootStillLoads(t *testing.T) {
	root, _ := symlinkBundle(t)
	symlink(t, "real/a.md", filepath.Join(root, "alias.md"))
	symlink(t, filepath.Join(root, "real", "a.md"), filepath.Join(root, "abs-alias.md"))

	b, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, p := range []string{"alias.md", "abs-alias.md"} {
		n, ok := b.Nodes[p]
		if !ok {
			t.Errorf("in-root symlink %s did not load", p)
			continue
		}
		if n.Type() != "Reference" {
			t.Errorf("%s type = %q, want Reference", p, n.Type())
		}
	}
	if len(b.EscapedSymlinks) != 0 {
		t.Errorf("EscapedSymlinks = %v, want none", b.EscapedSymlinks)
	}
}

// Positive control (§3.2 rule 5): a symlinked directory that escapes the root
// is not walked and is recorded, including one whose name ends in .md (which
// previously reached os.ReadFile and failed the whole load).
func TestConformance_Section3_2_SymlinkedDirOutsideRootNotWalked(t *testing.T) {
	root, outside := symlinkBundle(t)
	symlink(t, outside, filepath.Join(root, "ext"))
	symlink(t, outside, filepath.Join(root, "notes.md"))

	b, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for p := range b.Nodes {
		if strings.HasPrefix(p, "ext/") || strings.HasPrefix(p, "notes.md") {
			t.Errorf("walked into escaping symlinked dir: %s", p)
		}
	}
	assertNoSecret(t, b)
	if want := []string{"ext", "notes.md"}; !reflect.DeepEqual(b.EscapedSymlinks, want) {
		t.Errorf("EscapedSymlinks = %v, want %v", b.EscapedSymlinks, want)
	}
}

// Negative control: a symlinked directory that stays inside the root is not
// reported (the walk does not descend symlinked directories either way, so the
// in-root target is still loaded once through its real path).
func TestConformance_Section3_2_SymlinkedDirInsideRootNotReported(t *testing.T) {
	root, _ := symlinkBundle(t)
	symlink(t, "real", filepath.Join(root, "mirror"))

	b, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(b.EscapedSymlinks) != 0 {
		t.Errorf("EscapedSymlinks = %v, want none", b.EscapedSymlinks)
	}
	if _, ok := b.Nodes["real/a.md"]; !ok {
		t.Error("real/a.md went missing")
	}
}

// Positive control (§3.2 rule 5, §11.2): a reserved file and the .okf sidecar
// are read by Load too, so an escaping symlink there is refused as well.
func TestConformance_Section3_2_EscapingReservedAndSidecarNotRead(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "bundle")
	outside := filepath.Join(base, "outside")
	writeFile(t, root, "a.md", "---\ntype: Reference\n---\n# A\n")
	writeFile(t, outside, "log.md", "# Log\nOUTSIDE-SECRET-BODY\n")
	writeFile(t, outside, "okf", "okf_version: 9.9\n")
	symlink(t, filepath.Join(outside, "log.md"), filepath.Join(root, "log.md"))
	symlink(t, filepath.Join(outside, "okf"), filepath.Join(root, ".okf"))

	b, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := b.Reserved["log.md"]; ok {
		t.Error("escaping log.md symlink was read")
	}
	if b.OkfVersion != SpecVersion {
		t.Errorf("OkfVersion = %q, want %q (escaping .okf must not be read)", b.OkfVersion, SpecVersion)
	}
	if want := []string{".okf", "log.md"}; !reflect.DeepEqual(b.EscapedSymlinks, want) {
		t.Errorf("EscapedSymlinks = %v, want %v", b.EscapedSymlinks, want)
	}
}

// Negative control: an in-root .okf symlink is still honored.
func TestConformance_Section3_2_InRootSidecarSymlinkHonored(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.md", "---\ntype: Reference\n---\n# A\n")
	writeFile(t, root, "meta/okf", "okf_version: 0.1\n")
	symlink(t, "meta/okf", filepath.Join(root, ".okf"))

	b, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if b.OkfVersion != "0.1" {
		t.Errorf("OkfVersion = %q, want 0.1", b.OkfVersion)
	}
	if len(b.EscapedSymlinks) != 0 {
		t.Errorf("EscapedSymlinks = %v, want none", b.EscapedSymlinks)
	}
}

// Regression (negative control): with a RELATIVE bundle root, an in-root
// symlink must still be recognized as inside. EvalSymlinks of a relative path
// returns a relative path, which cannot be compared to the absolute real root.
func TestConformance_Section3_2_RelativeRootInRootSymlinkStillLoads(t *testing.T) {
	root, outside := symlinkBundle(t)
	symlink(t, "real/a.md", filepath.Join(root, "alias.md"))
	symlink(t, filepath.Join(outside, "secret.md"), filepath.Join(root, "linked.md"))
	t.Chdir(filepath.Dir(root))

	b, err := Load("bundle")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := b.Nodes["alias.md"]; !ok {
		t.Error("in-root symlink alias.md did not load under a relative root")
	}
	if want := []string{"linked.md"}; !reflect.DeepEqual(b.EscapedSymlinks, want) {
		t.Errorf("EscapedSymlinks = %v, want %v", b.EscapedSymlinks, want)
	}
}

// escapeTarget writes an outside file and links bundle-relative rel to it.
func escapeTarget(t *testing.T, root, rel, content string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "outside-"+filepath.Base(rel))
	if err := os.WriteFile(out, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	symlink(t, out, filepath.Join(root, filepath.FromSlash(rel)))
	return out
}

func readString(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Positive control (§3.2 rule 5, §11.2): the readers and writers that open a
// single bundle file directly, outside Load's walk, refuse one that is a
// symlink out of the root, and leave the outside file untouched.
func TestConformance_Section11_2_DirectReadersRefuseEscapingSymlinks(t *testing.T) {
	t.Run("AppendLog", func(t *testing.T) {
		root := t.TempDir()
		out := escapeTarget(t, root, "log.md", "# Log\nOUTSIDE\n")
		if err := AppendLog(root, "hello"); err == nil {
			t.Error("AppendLog wrote through an escaping log.md")
		}
		if got := readString(t, out); got != "# Log\nOUTSIDE\n" {
			t.Errorf("outside file changed: %q", got)
		}
	})
	t.Run("ReadLog", func(t *testing.T) {
		root := t.TempDir()
		escapeTarget(t, root, "log.md", "# Log\nOUTSIDE\n")
		if body, err := ReadLog(root); err == nil || strings.Contains(body, "OUTSIDE") {
			t.Errorf("ReadLog read an escaping log.md: body=%q err=%v", body, err)
		}
	})
	t.Run("WriteIndexAndIndexInSync", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "a.md", "---\ntype: Reference\n---\n# A\n")
		out := escapeTarget(t, root, "index.md", "OUTSIDE\n")
		b, err := Load(root)
		if err != nil {
			t.Fatal(err)
		}
		if ok, why := IndexInSync(b); ok || !strings.Contains(why, "outside the bundle root") {
			t.Errorf("IndexInSync = %v, %q; want an out-of-root refusal", ok, why)
		}
		if err := WriteIndex(b); err == nil {
			t.Error("WriteIndex wrote through an escaping index.md")
		}
		if got := readString(t, out); got != "OUTSIDE\n" {
			t.Errorf("outside file changed: %q", got)
		}
	})
	t.Run("bumpOkfSidecar", func(t *testing.T) {
		root := t.TempDir()
		out := escapeTarget(t, root, ".okf", "okf_version: 0.1\n")
		if err := bumpOkfSidecar(root, "0.2"); err == nil {
			t.Error("bumpOkfSidecar wrote through an escaping .okf")
		}
		if got := readString(t, out); got != "okf_version: 0.1\n" {
			t.Errorf("outside file changed: %q", got)
		}
	})
	t.Run("bumpRootIndexMarker", func(t *testing.T) {
		root := t.TempDir()
		out := escapeTarget(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n# KB\n")
		if err := bumpRootIndexMarker(root, "0.2"); err == nil {
			t.Error("bumpRootIndexMarker wrote through an escaping index.md")
		}
		if got := readString(t, out); !strings.Contains(got, `"0.1"`) {
			t.Errorf("outside file changed: %q", got)
		}
	})
	t.Run("LoadDriftIgnoreRevs", func(t *testing.T) {
		root := t.TempDir()
		escapeTarget(t, root, DriftIgnoreRevsFile, "0123456789abcdef0123456789abcdef01234567\n")
		got, err := LoadDriftIgnoreRevs(root)
		if err != nil || len(got) != 0 {
			t.Errorf("LoadDriftIgnoreRevs = %v, %v; want empty (escaping file not read)", got, err)
		}
	})
}

// Negative control: the same direct readers still follow an in-root symlink.
func TestConformance_Section11_2_DirectReadersFollowInRootSymlinks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "meta/log-src.md", "# Log\n")
	symlink(t, "meta/log-src.md", filepath.Join(root, "log.md"))
	writeFile(t, root, "meta/revs", "0123456789abcdef0123456789abcdef01234567\n")
	symlink(t, "meta/revs", filepath.Join(root, DriftIgnoreRevsFile))
	writeFile(t, root, "meta/okf", "okf_version: 0.1\n")
	symlink(t, "meta/okf", filepath.Join(root, ".okf"))

	if err := AppendLog(root, "hello"); err != nil {
		t.Fatalf("AppendLog: %v", err)
	}
	if got := readString(t, filepath.Join(root, "meta", "log-src.md")); !strings.Contains(got, "hello") {
		t.Errorf("in-root log target not updated: %q", got)
	}
	if body, err := ReadLog(root); err != nil || !strings.Contains(body, "hello") {
		t.Errorf("ReadLog = %q, %v", body, err)
	}
	if got, err := LoadDriftIgnoreRevs(root); err != nil || len(got) != 1 {
		t.Errorf("LoadDriftIgnoreRevs = %v, %v; want one rev", got, err)
	}
	if err := bumpOkfSidecar(root, "0.2"); err != nil {
		t.Errorf("bumpOkfSidecar: %v", err)
	}
}
