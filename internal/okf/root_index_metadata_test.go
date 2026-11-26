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
	"strings"
	"testing"
)

// OKF v0.3 §8.1 (bundle metadata): a bundle-root index.md MAY carry frontmatter
// with the optional keys okf_version, title, description, and license, plus
// extension keys (§4.1). A producer that regenerates index.md MUST keep the
// existing root frontmatter, and (§4) SHOULD NOT re-serialize values it did not
// change. These tests pin both halves: validate accepts what §8.1 permits, and
// index build writes the root block back byte-for-byte.

// reproRootFrontmatter is the root block from the reported defect: every §8.1
// key plus an x- extension key (§4.1).
const reproRootFrontmatter = "---\n" +
	"okf_version: \"0.3\"\n" +
	"title: Acme retail analytics\n" +
	"description: Tables and metrics for Acme retail.\n" +
	"license: CC-BY-4.0\n" +
	"x-acme-owner: team:data-platform\n" +
	"---\n"

func writeConceptBundle(t *testing.T, dir, rootIndex string) {
	t.Helper()
	writeFile(t, dir, "index.md", rootIndex)
	writeFile(t, dir, "notes/note.md", "---\ntype: Reference\ntitle: Note\ndescription: A note.\n---\n\n# Note\n")
}

// --- validate: negative controls (what §8.1 permits must stay silent) ------

func TestValidate_Section8_1_RootIndexBundleMetadataPasses(t *testing.T) {
	dir := t.TempDir()
	writeConceptBundle(t, dir, reproRootFrontmatter+"\n# Acme\n")
	if f := findingMessagesFor(validateBundleAt(t, dir), "index.md"); len(f) != 0 {
		t.Errorf("§8.1: root index with okf_version/title/description/license/x- key must validate clean; got %v", f)
	}
}

func TestValidate_Section8_1_RootIndexUnprefixedExtensionKeyPasses(t *testing.T) {
	// §4.1: producers SHOULD (not MUST) prefix extension keys with x-; an
	// unprefixed key the spec does not define is still an extension key (§2),
	// and §11.2 forbids rejecting it.
	dir := t.TempDir()
	writeConceptBundle(t, dir, "---\nokf_version: \"0.3\"\nowner: data-platform\nmaintainers: [a, b]\n---\n\n# Acme\n")
	if f := findingMessagesFor(validateBundleAt(t, dir), "index.md"); len(f) != 0 {
		t.Errorf("§4.1/§11.2: an unknown root-index key must not be rejected; got %v", f)
	}
}

func TestValidate_Section8_1_RootIndexWithoutOkfVersionPasses(t *testing.T) {
	// §8.1: every key is optional, okf_version included. A block carrying only
	// a title is conformant.
	dir := t.TempDir()
	writeConceptBundle(t, dir, "---\ntitle: Acme\n---\n\n# Acme\n")
	if f := findingMessagesFor(validateBundleAt(t, dir), "index.md"); len(f) != 0 {
		t.Errorf("§8.1: root index without okf_version must validate clean; got %v", f)
	}
}

func TestValidate_Section8_1_RootIndexEmptyBlockPasses(t *testing.T) {
	// §8.1: all keys optional, so a present block with zero keys is conformant.
	dir := t.TempDir()
	writeConceptBundle(t, dir, "---\n---\n\n# Acme\n")
	if f := findingMessagesFor(validateBundleAt(t, dir), "index.md"); len(f) != 0 {
		t.Errorf("§8.1: root index with an empty block must validate clean; got %v", f)
	}
}

// --- validate: positive controls (the defects the floor exists for) --------

func TestValidate_Section8_1_RootIndexConceptKeyFlagged(t *testing.T) {
	// `type` is a concept key the spec defines (§4.1); it is neither a §8.1
	// bundle-metadata key nor an extension key (§2), so it stays flagged even
	// alongside permitted keys. This is the generator-regression guard.
	for _, key := range []string{"type: Index", "tags: [a]", "status: stable"} {
		dir := t.TempDir()
		writeConceptBundle(t, dir, "---\nokf_version: \"0.3\"\ntitle: Acme\n"+key+"\n---\n\n# Acme\n")
		msgs := findingMessagesFor(validateBundleAt(t, dir), "index.md")
		name, _, _ := strings.Cut(key, ":")
		if len(msgs) != 1 || !strings.Contains(msgs[0], "§8.1") || !strings.HasSuffix(msgs[0], ": "+name) {
			t.Errorf("§8.1: root index carrying concept key %q must yield exactly one §8.1 finding naming it; got %v", name, msgs)
		}
	}
}

func TestValidate_Section8_1_NonRootIndexMetadataStillFlagged(t *testing.T) {
	// §8.1 is bundle-root-only: an index in a subdirectory has no frontmatter
	// (§8), so title/description there remain a violation.
	dir := t.TempDir()
	writeConceptBundle(t, dir, reproRootFrontmatter+"\n# Acme\n")
	writeFile(t, dir, "notes/index.md", "---\ntitle: Notes\ndescription: Notes.\n---\n\n# Notes\n")
	if f := findingMessagesFor(validateBundleAt(t, dir), "notes/index.md"); len(f) == 0 {
		t.Errorf("§8: a non-root index carrying §8.1 keys must still be flagged; got none")
	}
}

// --- index build: §8.1 MUST keep the root frontmatter ----------------------

// awkwardRootFrontmatter exercises everything a re-serializer would disturb:
// a comment, non-canonical key order, single-quoted and unquoted scalars, a
// flow sequence, a block scalar, and trailing spaces inside a quoted value.
const awkwardRootFrontmatter = "---\n" +
	"# bundle metadata, curated by hand\n" +
	"title:   'Acme  retail'\n" +
	"x-acme-owner: team:data-platform\n" +
	"okf_version: 0.3\n" +
	"x-acme-tags: [ retail,  margin ]\n" +
	"description: >-\n" +
	"  Tables and metrics\n" +
	"  for Acme retail.\n" +
	"license: \"LICENSE.md \"\n" +
	"---\n"

func TestWriteIndex_Section8_1_PreservesRootFrontmatterByteForByte(t *testing.T) {
	for name, fm := range map[string]string{"repro": reproRootFrontmatter, "awkward": awkwardRootFrontmatter} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeConceptBundle(t, dir, fm+"\n# Acme\n")
			// A .okf sidecar declaring a DIFFERENT version must not leak into an
			// existing root block (the block is kept, not merged).
			writeFile(t, dir, ".okf", "okf_version: 0.2\n")

			b, err := Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := WriteIndex(b); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(dir, "index.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(string(got), fm) {
				t.Fatalf("§8.1/§4: index build must keep the root frontmatter byte-for-byte;\n--- want prefix ---\n%s--- got ---\n%s", fm, got)
			}
			if !strings.Contains(string(got), "# Acme\n") || !strings.Contains(string(got), "* [Notes](notes/)") {
				t.Errorf("rebuilt root index lost its H1 or listing; got:\n%s", got)
			}

			b2, err := Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			if ok, why := IndexInSync(b2); !ok {
				t.Errorf("index check must be clean right after build: %s", why)
			}
			if f := Validate(b2); len(f) != 0 {
				t.Errorf("rebuilt bundle must validate clean; got %v", f)
			}
			// Idempotent: a second build is a byte-level no-op.
			if err := WriteIndex(b2); err != nil {
				t.Fatal(err)
			}
			again, _ := os.ReadFile(filepath.Join(dir, "index.md"))
			if string(again) != string(got) {
				t.Errorf("second index build changed the root index:\n--- first ---\n%s--- second ---\n%s", got, again)
			}
		})
	}
}

func TestWriteIndex_Section8_1_PreservesCRLFRootFrontmatter(t *testing.T) {
	dir := t.TempDir()
	crlf := strings.ReplaceAll(reproRootFrontmatter+"\n# Acme\n", "\n", "\r\n")
	writeConceptBundle(t, dir, crlf)
	b, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteIndex(b); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "index.md"))
	want := strings.ReplaceAll(reproRootFrontmatter, "\n", "\r\n")
	if !strings.HasPrefix(string(got), want) {
		t.Fatalf("§8.1: CRLF root frontmatter must survive index build verbatim; got %q", got)
	}
	if strings.Contains(string(got), "\r\r\n") {
		t.Errorf("CRLF round-trip doubled carriage returns: %q", got)
	}
	b2, _ := Load(dir)
	if ok, why := IndexInSync(b2); !ok {
		t.Errorf("index check must be clean after a CRLF build: %s", why)
	}
}

func TestWriteIndex_Section8_1_KeepsBlockWithoutOkfVersionUnmerged(t *testing.T) {
	// A root block without okf_version is kept as is: the sidecar value is NOT
	// injected, because that would modify frontmatter the producer did not own.
	dir := t.TempDir()
	fm := "---\ntitle: Acme\n---\n"
	writeConceptBundle(t, dir, fm+"\n# Acme\n")
	writeFile(t, dir, ".okf", "okf_version: 0.2\n")
	b, _ := Load(dir)
	if got := RenderIndex(b); !strings.HasPrefix(got, fm+"\n# Acme\n") {
		t.Errorf("§8.1: block without okf_version must be kept verbatim, not merged with the sidecar; got:\n%s", got)
	}
}

func TestWriteIndex_Section8_1_KeepsUnparseableRootBlock(t *testing.T) {
	// A root block that fails to parse is kept, not silently dropped: §8.1
	// says keep the existing frontmatter, and validate still reports it.
	dir := t.TempDir()
	fm := "---\ntitle: [unterminated\n---\n"
	writeConceptBundle(t, dir, fm+"\n# Acme\n")
	b, _ := Load(dir)
	if err := WriteIndex(b); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "index.md"))
	if !strings.HasPrefix(string(got), fm) {
		t.Fatalf("§8.1: an unparseable root block must be kept, not dropped; got:\n%s", got)
	}
	b2, _ := Load(dir)
	if !hasFindingMessage(Validate(b2), "index.md", "unparseable frontmatter") {
		t.Errorf("validate must still report the unparseable root block; got %v", Validate(b2))
	}
}

func TestWriteIndex_Section8_1_NoRootBlockStillEmitsSidecarMarker(t *testing.T) {
	// Control: with no root block on disk, the existing marker-compatibility
	// rule still applies — the .okf sidecar's okf_version is emitted.
	dir := t.TempDir()
	writeConceptBundle(t, dir, "# Acme\n")
	writeFile(t, dir, ".okf", "okf_version: 0.2\n")
	b, _ := Load(dir)
	if got := RenderIndex(b); !strings.HasPrefix(got, "---\nokf_version: \"0.2\"\n---\n\n# Acme\n") {
		t.Errorf("no root block + sidecar must still emit the okf_version-only block; got:\n%s", got)
	}
}

func TestWriteIndex_Section8_1_NestedIndexStaysFrontmatterFree(t *testing.T) {
	// Control: preservation is bundle-root-only. A nested index carrying
	// frontmatter is regenerated without it (§8).
	dir := t.TempDir()
	writeConceptBundle(t, dir, reproRootFrontmatter+"\n# Acme\n")
	writeFile(t, dir, "notes/index.md", "---\ntitle: Notes\n---\n\n# Notes\n")
	b, _ := Load(dir)
	if got := RenderDirIndex(b, "notes"); strings.HasPrefix(got, "---") {
		t.Errorf("§8: nested index must not keep frontmatter; got:\n%s", got)
	}
}

// --- migrate: the version bump touches okf_version only -------------------

func TestBumpRootIndexMarker_Section8_1_ChangesOnlyOkfVersion(t *testing.T) {
	// §8.1 keep + §4 no re-serialization: bumping the version must rewrite the
	// okf_version value and leave every other byte of the root index alone.
	dir := t.TempDir()
	before := strings.Replace(awkwardRootFrontmatter, "okf_version: 0.3\n", "okf_version: \"0.1\"  # declared\n", 1) + "\n# Acme\n"
	writeFile(t, dir, "index.md", before)
	if err := bumpRootIndexMarker(dir, "0.2"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "index.md"))
	want := strings.Replace(before, "okf_version: \"0.1\"  # declared\n", "okf_version: \"0.2\"  # declared\n", 1)
	if string(got) != want {
		t.Fatalf("§8.1/§4: version bump must change only the okf_version value;\n--- want ---\n%s--- got ---\n%s", want, got)
	}
}

func TestBumpRootIndexMarker_Section8_1_QuotesBareVersion(t *testing.T) {
	// §4: producers SHOULD quote number-like values, so a bare 0.1 becomes "0.2".
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "---\ntitle: Acme\nokf_version: 0.1\n---\n\n# Acme\n")
	if err := bumpRootIndexMarker(dir, "0.2"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "index.md"))
	if want := "---\ntitle: Acme\nokf_version: \"0.2\"\n---\n\n# Acme\n"; string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBumpRootIndexMarker_Section8_1_CRLFAndNoOps(t *testing.T) {
	dir := t.TempDir()
	crlf := "---\r\ntitle:  Acme\r\nokf_version: '0.1'\r\n---\r\n\r\n# Acme\r\n"
	writeFile(t, dir, "index.md", crlf)
	if err := bumpRootIndexMarker(dir, "0.2"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "index.md"))
	if want := strings.Replace(crlf, "'0.1'", "\"0.2\"", 1); string(got) != want {
		t.Fatalf("CRLF bump: got %q, want %q", got, want)
	}
	// Already at target, and no marker at all: both are byte-level no-ops.
	for _, src := range []string{"---\ntitle:  Acme\nokf_version: '0.2'\n---\n", "---\ntitle:  Acme\n---\n"} {
		writeFile(t, dir, "index.md", src)
		if err := bumpRootIndexMarker(dir, "0.2"); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(filepath.Join(dir, "index.md")); string(got) != src {
			t.Errorf("no-op bump changed the file: got %q, want %q", got, src)
		}
	}
}
