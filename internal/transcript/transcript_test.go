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

package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fence = "```"

func TestParse_FindsOnlyOkfctlTranscriptBlocks(t *testing.T) {
	src := strings.Join([]string{
		"# Title",                      // 1
		"",                             // 2
		fence + "sh",                   // 3: a command block with no `$ okfctl` prompt
		"okfctl validate x",            // 4
		fence,                          // 5
		"",                             // 6
		fence + "text",                 // 7: the transcript
		"$ okfctl bundle init kb",      // 8
		"Initialized OKF bundle in kb", // 9
		"$ echo hi >> kb/x",            // 10
		"$ okfctl validate kb",         // 11
		"OK",                           // 12
		fence,                          // 13
	}, "\n")
	blocks, err := Parse("doc.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	b := blocks[0]
	if b.File != "doc.md" || b.Index != 1 || b.Line != 7 {
		t.Fatalf("block identity = %s #%d line %d", b.File, b.Index, b.Line)
	}
	if len(b.Steps) != 3 {
		t.Fatalf("got %d steps, want 3", len(b.Steps))
	}
	if s := b.Steps[0]; s.Cmd != "okfctl bundle init kb" || s.Line != 8 || len(s.Want) != 1 || s.Want[0].Text != "Initialized OKF bundle in kb" || s.Want[0].Num != 9 {
		t.Fatalf("step 0 = %+v", s)
	}
	if s := b.Steps[1]; s.Cmd != "echo hi >> kb/x" || len(s.Want) != 0 {
		t.Fatalf("step 1 = %+v", s)
	}
	if s := b.Steps[2]; s.WantExit != 0 || s.Want[0].Num != 12 {
		t.Fatalf("step 2 = %+v", s)
	}
}

func TestParse_Directives(t *testing.T) {
	src := strings.Join([]string{
		"<!-- transcript: exit 1 okfctl lint --strict kb -->",
		"<!-- transcript: setup",
		"$ cp -R \"$TRANSCRIPT_FIXTURES/kb\" .",
		"-->",
		"",
		fence + "text",
		"$ okfctl lint --strict kb",
		"okfctl: 1 lint finding(s)",
		fence,
		"",
		"<!-- transcript: skip needs network access to a live registry -->",
		fence,
		"$ okfctl connect https://example.com",
		"connected",
		fence,
	}, "\n")
	blocks, err := Parse("doc.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2", len(blocks))
	}
	if got := blocks[0].Steps[0].WantExit; got != 1 {
		t.Errorf("exit directive: WantExit = %d, want 1", got)
	}
	if len(blocks[0].Setup) != 1 || blocks[0].Setup[0].Text != `cp -R "$TRANSCRIPT_FIXTURES/kb" .` || blocks[0].Setup[0].Num != 3 {
		t.Errorf("setup = %+v", blocks[0].Setup)
	}
	if blocks[1].Skip != "needs network access to a live registry" {
		t.Errorf("skip reason = %q", blocks[1].Skip)
	}
}

func TestParse_RejectsBadDirectives(t *testing.T) {
	transcriptBlock := fence + "\n$ okfctl validate kb\nOK\n" + fence + "\n"
	cases := map[string]string{
		"skip without a reason":         "<!-- transcript: skip -->\n" + transcriptBlock,
		"directive with no block":       "<!-- transcript: skip offline -->\n\nSome prose.\n\n" + transcriptBlock,
		"directive at end of file":      transcriptBlock + "\n<!-- transcript: skip offline -->\n",
		"directive on a non-transcript": "<!-- transcript: skip offline -->\n" + fence + "\nplain text\n" + fence + "\n",
		"unknown verb":                  "<!-- transcript: frobnicate -->\n" + transcriptBlock,
		"exit names no command":         "<!-- transcript: exit 1 okfctl lint kb -->\n" + transcriptBlock,
		"exit code not a number":        "<!-- transcript: exit one okfctl validate kb -->\n" + transcriptBlock,
		"setup line without prompt":     "<!-- transcript: setup\ncp a b\n-->\n" + transcriptBlock,
		"unterminated directive":        "<!-- transcript: setup\n$ cp a b\n" + transcriptBlock,
		"output before first command":   fence + "\nstray\n$ okfctl validate kb\n" + fence + "\n",
		"unterminated fence":            fence + "\n$ okfctl validate kb\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse("doc.md", []byte(src))
			if err == nil {
				t.Fatal("Parse accepted a malformed transcript; want an error")
			}
			if !strings.Contains(err.Error(), "doc.md:") {
				t.Errorf("error %q does not name the file and line", err)
			}
		})
	}
}

func TestMatchLine_Placeholders(t *testing.T) {
	vars := map[string]string{"<TMPDIR>": "/tmp/x"}
	cases := []struct {
		want, got string
		ok        bool
	}{
		{"exact", "exact", true},
		{"exact", "exacT", false},
		{"## <DATE>", "## 2026-11-27", true},
		{"## <DATE>", "## 2026-11-2", false},
		{"at <TIMESTAMP>", "at 2026-11-27T10:15:53Z", true},
		{"at <TIMESTAMP>", "at 2026-11-27T10:15:53.123-05:00", true},
		{"at <TIMESTAMP>", "at 2026-11-27 10:15:53Z", false},
		{"commit <SHA>", "commit 7b2a213", true},
		{"commit <SHA>", "commit 7b2a213e1f0c9d8b7a6f5e4d3c2b1a0f9e8d7c6b", true},
		{"commit <SHA>", "commit xyz", false},
		{"wrote <TMPDIR>/kb", "wrote /tmp/x/kb", true},
		{"wrote <TMPDIR>/kb", "wrote /tmp/y/kb", false},
		{"regex chars stay literal: a.b*", "regex chars stay literal: aXbbb", false},
		{"<unknown> stays literal", "<unknown> stays literal", true},
	}
	for _, c := range cases {
		if got := matchLine(c.want, c.got, vars); got != c.ok {
			t.Errorf("matchLine(%q, %q) = %v, want %v", c.want, c.got, got, c.ok)
		}
	}
}

// fakeOkfctl writes an `okfctl` shell script into a fresh bin dir so the
// runner can be exercised without building the real binary.
func fakeOkfctl(t *testing.T, script string) []string {
	t.Helper()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "okfctl"), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH")}
}

func replaySrc(t *testing.T, src string, env []string) []Failure {
	t.Helper()
	blocks, err := Parse("doc.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return Replay(blocks, t.TempDir(), env)
}

func TestReplay_MatchingTranscriptPasses(t *testing.T) {
	env := fakeOkfctl(t, `echo "out: $*"; echo "err: $1" >&2; [ "$1" = fail ] && exit 3; exit 0`)
	src := "<!-- transcript: exit 3 okfctl fail now -->\n" + fence + "\n" +
		"$ okfctl hello world\nout: hello world\nerr: hello\n" +
		"$ echo state > f\n$ cat f\nstate\n" +
		"$ okfctl fail now\nout: fail now\nerr: fail\n" + fence + "\n"
	if fs := replaySrc(t, src, env); len(fs) != 0 {
		t.Fatalf("want no failures, got:\n%v", fs)
	}
}

func TestReplay_OutputDriftNamesFileBlockLineExpectedActual(t *testing.T) {
	env := fakeOkfctl(t, `echo "Initialized OKF bundle in $3"`)
	src := "intro\n\n" + fence + "text\n$ okfctl bundle init kb\nInitialized OKF bundle in kc\n" + fence + "\n"
	fs := replaySrc(t, src, env)
	if len(fs) != 1 {
		t.Fatalf("want 1 failure, got %d: %v", len(fs), fs)
	}
	msg := fs[0].Error()
	for _, want := range []string{
		"doc.md:5:",               // the drifted line
		"block 1 (line 3)",        // which block
		"$ okfctl bundle init kb", // which command
		"expected: Initialized OKF bundle in kc",
		"actual:   Initialized OKF bundle in kb",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("failure message missing %q:\n%s", want, msg)
		}
	}
}

func TestReplay_ExtraAndMissingLinesFail(t *testing.T) {
	env := fakeOkfctl(t, `echo one; echo two`)
	short := fence + "\n$ okfctl x\none\n" + fence + "\n"
	if fs := replaySrc(t, short, env); len(fs) != 1 || !strings.Contains(fs[0].Error(), "actual:   two") {
		t.Errorf("unshown extra output line must fail, got %v", fs)
	}
	long := fence + "\n$ okfctl x\none\ntwo\nthree\n" + fence + "\n"
	if fs := replaySrc(t, long, env); len(fs) != 1 || !strings.Contains(fs[0].Error(), "expected: three") {
		t.Errorf("shown line the binary no longer prints must fail, got %v", fs)
	}
}

func TestReplay_ExitStatusDriftFails(t *testing.T) {
	env := fakeOkfctl(t, `echo ok; exit 1`)
	src := fence + "\n$ okfctl validate kb\nok\n" + fence + "\n"
	fs := replaySrc(t, src, env)
	if len(fs) != 1 || !strings.Contains(fs[0].Error(), "exit status 1, want 0") {
		t.Fatalf("unexpected non-zero exit must fail, got %v", fs)
	}
	env = fakeOkfctl(t, `echo ok`)
	src = "<!-- transcript: exit 1 okfctl validate kb -->\n" + src
	fs = replaySrc(t, src, env)
	if len(fs) != 1 || !strings.Contains(fs[0].Error(), "exit status 0, want 1") {
		t.Fatalf("missing expected non-zero exit must fail, got %v", fs)
	}
}

func TestReplay_SkipAndSetup(t *testing.T) {
	env := fakeOkfctl(t, `cat "$1"`)
	src := "<!-- transcript: skip shows a remote registry this test can't reach -->\n" +
		fence + "\n$ okfctl nope\nnever checked\n" + fence + "\n\n" +
		"<!-- transcript: setup\n$ echo seeded > kb.txt\n-->\n" +
		fence + "\n$ okfctl kb.txt\nseeded\n" + fence + "\n"
	if fs := replaySrc(t, src, env); len(fs) != 0 {
		t.Fatalf("want no failures, got %v", fs)
	}
	bad := "<!-- transcript: setup\n$ false\n-->\n" + fence + "\n$ okfctl kb.txt\n" + fence + "\n"
	if fs := replaySrc(t, bad, env); len(fs) == 0 || !strings.Contains(fs[0].Error(), "setup") {
		t.Fatalf("a failing setup command must fail the block, got %v", fs)
	}
}

func TestReplay_SessionStatePersistsAcrossBlocks(t *testing.T) {
	env := fakeOkfctl(t, `cat "$1"`)
	src := fence + "\n$ echo kept > f\n$ okfctl f\nkept\n" + fence + "\n\n" +
		fence + "\n$ okfctl f\nkept\n" + fence + "\n"
	if fs := replaySrc(t, src, env); len(fs) != 0 {
		t.Fatalf("blocks in one file share a working dir; got %v", fs)
	}
}

func TestParseHTML_ReadsTranscriptPanels(t *testing.T) {
	src := strings.Join([]string{
		`<div class="term">`, // 1
		`<pre>`,              // 2: no data-transcript, ignored
		`$ okfctl not-a-transcript`,
		`</pre>`, // 4
		`<!-- transcript: exit 1 okfctl lint --strict kb -->`,                                            // 5
		`<div role="tabpanel"><pre data-transcript>`,                                                     // 6
		`<span class="p">$</span> <span class="c">okfctl lint</span> <span class="f">--strict</span> kb`, // 7
		`<span class="o">missing-xref: a.md mentions &quot;B&quot; &amp; more</span>`,                    // 8
		`<span class="p">$</span> echo hi &gt; f</pre></div>`,                                            // 9
		`</div>`, // 10
	}, "\n")
	blocks, err := ParseHTML("index.astro", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1 (a <pre> without data-transcript is not a transcript)", len(blocks))
	}
	b := blocks[0]
	if b.File != "index.astro" || b.Line != 6 {
		t.Fatalf("block identity = %s line %d, want index.astro line 6", b.File, b.Line)
	}
	if len(b.Steps) != 2 {
		t.Fatalf("got %d steps, want 2: %+v", len(b.Steps), b.Steps)
	}
	s := b.Steps[0]
	if s.Cmd != "okfctl lint --strict kb" || s.Line != 7 || s.WantExit != 1 {
		t.Fatalf("step 0 = %+v, want the tag-stripped command on line 7 exiting 1", s)
	}
	if len(s.Want) != 1 || s.Want[0].Text != `missing-xref: a.md mentions "B" & more` || s.Want[0].Num != 8 {
		t.Fatalf("step 0 output = %+v, want entities unescaped on line 8", s.Want)
	}
	if s := b.Steps[1]; s.Cmd != "echo hi > f" || s.Line != 9 || len(s.Want) != 0 {
		t.Fatalf("step 1 = %+v, want the command before </pre> on line 9", s)
	}
}

func TestParseHTML_ErrorsNameTheSourceLine(t *testing.T) {
	src := strings.Join([]string{
		`<p>intro</p>`,                    // 1
		`<p>more</p>`,                     // 2
		`<!-- transcript: frobnicate -->`, // 3
		`<pre data-transcript>`,           // 4
		`$ okfctl x</pre>`,                // 5
	}, "\n")
	_, err := ParseHTML("index.astro", []byte(src))
	if err == nil || !strings.HasPrefix(err.Error(), "index.astro:3:") {
		t.Fatalf("want an error at index.astro:3, got %v", err)
	}
}

func TestParseHTML_JSXCommentDirectives(t *testing.T) {
	src := strings.Join([]string{
		`{/* transcript: exit 1 okfctl lint kb */}`, // 1
		`{/* transcript: setup`,                     // 2
		`$ okfctl bundle init kb`,                   // 3
		`*/}`,                                       // 4
		`<pre data-transcript>`,                     // 5
		`$ okfctl lint kb</pre>`,                    // 6
	}, "\n")
	blocks, err := ParseHTML("index.astro", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 || blocks[0].Steps[0].WantExit != 1 || len(blocks[0].Setup) != 1 || blocks[0].Setup[0].Num != 3 {
		t.Fatalf("JSX-comment directives must apply like HTML ones, got %+v", blocks)
	}
	detached := "{/* transcript: skip a reason */}\n<div>\n<pre data-transcript>\n$ okfctl x</pre>\n"
	if _, err := ParseHTML("index.astro", []byte(detached)); err == nil || !strings.HasPrefix(err.Error(), "index.astro:1:") {
		t.Fatalf("markup between a directive and its panel must detach it, got %v", err)
	}
}

func TestReplay_TmpdirPlaceholder(t *testing.T) {
	env := fakeOkfctl(t, `echo "wrote $PWD/kb"`)
	src := fence + "\n$ okfctl x\nwrote <TMPDIR>/kb\n" + fence + "\n"
	if fs := replaySrc(t, src, env); len(fs) != 0 {
		t.Fatalf("<TMPDIR> must match the session dir, got %v", fs)
	}
}
