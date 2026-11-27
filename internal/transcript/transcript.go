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

// Package transcript replays the shell transcripts shown in okfctl's Markdown
// docs against a real binary, so shown output can't drift from what the
// binary prints.
//
// A transcript is a fenced code block with at least one line starting with
// "$ okfctl". Inside it, a line starting with "$ " is a command; every line
// after it, up to the next command or the closing fence, is the output that
// command must print (stdout and stderr interleaved, in write order). A
// command with no lines after it must print nothing. Every command, okfctl or
// not, runs through /bin/sh in one working directory per Markdown file, so
// later blocks in a file see the files earlier blocks created.
//
// Expected output matches line for line, exactly, except for these
// placeholders, which stand for values that change from run to run:
//
//	<DATE>       a calendar date, 2006-01-02
//	<TIMESTAMP>  an RFC 3339 timestamp, 2006-01-02T15:04:05Z (fraction and offset allowed)
//	<SHA>        a hex git object name, 7 to 40 characters
//	<TMPDIR>     the absolute path of the working directory the file replays in
//
// Any other <word> is literal text.
//
// HTML comments placed directly above a block (blank lines and other
// directives may sit between) adjust how it replays. They don't render on
// GitHub or the site:
//
//	<!-- transcript: skip REASON -->
//	    Don't replay this block. REASON is required.
//	<!-- transcript: exit N COMMAND -->
//	    COMMAND (the text after "$ ", exactly) must exit N. Every other
//	    command must exit 0.
//	<!-- transcript: setup
//	$ COMMAND
//	-->
//	    Run each COMMAND, unchecked except that it must exit 0, before the
//	    block. Use it to build the state a block assumes without showing it.
package transcript

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Line is one line of a Markdown file, with its 1-based line number.
type Line struct {
	Num  int
	Text string
}

// Step is one "$ " command in a transcript and the output shown under it.
type Step struct {
	Line     int    // line number of the "$ " line
	Cmd      string // the command, without the "$ " prompt
	Want     []Line // the output lines shown under the command
	WantExit int    // the exit status the command must return
}

// Block is one transcript block.
type Block struct {
	File  string
	Index int    // 1-based position among the file's transcript blocks
	Line  int    // line number of the opening fence
	Skip  string // non-empty: don't replay, for this reason
	Setup []Line // commands to run, unchecked, before the steps
	Steps []Step
}

const (
	directiveOpen  = "<!-- transcript:"
	directiveClose = "-->"
	prompt         = "$ "
)

// Parse finds every transcript block in a Markdown file. It returns an error,
// naming the file and line, for a malformed directive or transcript.
func Parse(file string, src []byte) ([]Block, error) {
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	errf := func(n int, format string, a ...any) error {
		return fmt.Errorf("%s:%d: %s", file, n, fmt.Sprintf(format, a...))
	}

	type pendingDirective struct {
		line                  int
		skip                  string
		exits                 map[string]int
		setup                 []Line
		hasSkip, hasDirective bool
	}
	var pend pendingDirective
	var blocks []Block

	for i := 0; i < len(lines); i++ {
		text := lines[i]
		trimmed := strings.TrimSpace(text)

		if strings.HasPrefix(trimmed, directiveOpen) {
			if !pend.hasDirective {
				pend = pendingDirective{line: i + 1, exits: map[string]int{}, hasDirective: true}
			}
			body := strings.TrimSpace(strings.TrimPrefix(trimmed, directiveOpen))
			if rest, ok := strings.CutSuffix(body, directiveClose); ok {
				verb, arg, _ := strings.Cut(strings.TrimSpace(rest), " ")
				arg = strings.TrimSpace(arg)
				switch verb {
				case "skip":
					if arg == "" {
						return nil, errf(i+1, "transcript skip needs a reason")
					}
					pend.skip, pend.hasSkip = arg, true
				case "exit":
					code, cmd, _ := strings.Cut(arg, " ")
					n, err := strconv.Atoi(code)
					if err != nil || strings.TrimSpace(cmd) == "" {
						return nil, errf(i+1, "transcript exit wants `exit N COMMAND`, got %q", arg)
					}
					pend.exits[strings.TrimSpace(cmd)] = n
				default:
					return nil, errf(i+1, "unknown transcript directive %q (want skip, exit, or setup)", verb)
				}
				continue
			}
			if body != "setup" {
				return nil, errf(i+1, "unknown transcript directive %q (multi-line form is only `setup`)", body)
			}
			start := i + 1
			for i++; ; i++ {
				if i >= len(lines) {
					return nil, errf(start, "transcript setup is never closed with -->")
				}
				t := strings.TrimSpace(lines[i])
				if t == directiveClose {
					break
				}
				if t == "" {
					continue
				}
				cmd, ok := strings.CutPrefix(t, prompt)
				if !ok {
					return nil, errf(i+1, "transcript setup line must start with %q, got %q", prompt, t)
				}
				pend.setup = append(pend.setup, Line{Num: i + 1, Text: cmd})
			}
			continue
		}

		fenceLen := backtickRun(trimmed)
		if fenceLen < 3 {
			if pend.hasDirective && trimmed != "" {
				return nil, errf(pend.line, "transcript directive is not directly above a code block")
			}
			continue
		}

		// A fenced block: collect its body up to the closing fence.
		open := i + 1
		var body []Line
		closed := false
		for i++; i < len(lines); i++ {
			t := strings.TrimSpace(lines[i])
			if n := backtickRun(t); n >= fenceLen && n == len(t) {
				closed = true
				break
			}
			body = append(body, Line{Num: i + 1, Text: lines[i]})
		}
		if !closed {
			return nil, errf(open, "code block is never closed")
		}
		if !isTranscript(body) {
			if pend.hasDirective {
				return nil, errf(pend.line, "transcript directive sits above a block with no `$ okfctl` line")
			}
			continue
		}

		b := Block{File: file, Index: len(blocks) + 1, Line: open, Skip: pend.skip, Setup: pend.setup}
		for _, l := range body {
			if cmd, ok := strings.CutPrefix(l.Text, prompt); ok {
				b.Steps = append(b.Steps, Step{Line: l.Num, Cmd: cmd})
				continue
			}
			if len(b.Steps) == 0 {
				return nil, errf(l.Num, "transcript output appears before its first `$ ` command")
			}
			s := &b.Steps[len(b.Steps)-1]
			s.Want = append(s.Want, l)
		}
		for cmd, code := range pend.exits {
			found := false
			for j := range b.Steps {
				if b.Steps[j].Cmd == cmd {
					b.Steps[j].WantExit, found = code, true
				}
			}
			if !found {
				return nil, errf(pend.line, "transcript exit names %q, which isn't a command in the block", cmd)
			}
		}
		blocks = append(blocks, b)
		pend = pendingDirective{}
	}
	if pend.hasDirective {
		return nil, errf(pend.line, "transcript directive is not directly above a code block")
	}
	return blocks, nil
}

func backtickRun(s string) int {
	n := 0
	for n < len(s) && s[n] == '`' {
		n++
	}
	return n
}

func isTranscript(body []Line) bool {
	for _, l := range body {
		if strings.HasPrefix(l.Text, prompt+"okfctl") {
			return true
		}
	}
	return false
}

// Failure is one place a replayed transcript disagrees with the binary.
type Failure struct {
	Block    Block
	Line     int    // the Markdown line that disagrees
	Cmd      string // the command whose output or status drifted
	Expected string
	Actual   string
	Msg      string // set instead of Expected/Actual for status and setup failures
}

func (f Failure) Error() string {
	head := fmt.Sprintf("%s:%d: block %d (line %d): $ %s", f.Block.File, f.Line, f.Block.Index, f.Block.Line, f.Cmd)
	if f.Msg != "" {
		return head + ": " + f.Msg
	}
	return fmt.Sprintf("%s\n  expected: %s\n  actual:   %s", head, f.Expected, f.Actual)
}

// Replay runs blocks in order in dir, one shared working directory, and
// returns every disagreement. env entries are added to (and override) a small
// fixed environment: PATH from the caller's process, HOME and
// OKFCTL_CONFIG_HOME inside dir, TZ=UTC, LC_ALL=C, and a git ceiling at dir's
// parent so an enclosing repository can't leak into the run.
func Replay(blocks []Block, dir string, env []string) []Failure {
	home := filepath.Join(dir, ".home")
	_ = os.MkdirAll(home, 0o750)
	base := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"OKFCTL_CONFIG_HOME=" + filepath.Join(home, "okfctl"),
		"TZ=UTC",
		"LC_ALL=C",
		"PWD=" + dir,
		"GIT_CEILING_DIRECTORIES=" + filepath.Dir(dir),
	}
	fullEnv := append(base, env...)
	vars := map[string]string{"<TMPDIR>": dir}

	var fails []Failure
	for _, b := range blocks {
		if b.Skip != "" {
			continue
		}
		setupOK := true
		for _, s := range b.Setup {
			out, code := run(s.Text, dir, fullEnv)
			if code != 0 {
				fails = append(fails, Failure{Block: b, Line: s.Num, Cmd: s.Text,
					Msg: fmt.Sprintf("setup command exited %d:\n%s", code, out)})
				setupOK = false
				break
			}
		}
		if !setupOK {
			continue
		}
		for _, s := range b.Steps {
			out, code := run(s.Cmd, dir, fullEnv)
			if f, bad := compare(b, s, out, vars); bad {
				fails = append(fails, f)
			}
			if code != s.WantExit {
				fails = append(fails, Failure{Block: b, Line: s.Line, Cmd: s.Cmd,
					Msg: fmt.Sprintf("exit status %d, want %d", code, s.WantExit)})
			}
		}
	}
	return fails
}

// run executes one transcript command through /bin/sh. The shell is the
// point: transcripts show shell lines (quotes, redirects, pipes) and must run
// as a reader would type them. Commands come only from Markdown committed to
// this repo, and only under `go test`.
func run(cmd, dir string, env []string) (string, int) {
	c := exec.Command("/bin/sh", "-c", cmd) //nolint:gosec // G204: running the doc's shell line is the point; see above
	c.Dir = dir
	c.Env = env
	var buf bytes.Buffer
	c.Stdout = &buf
	c.Stderr = &buf
	err := c.Run()
	code := 0
	if err != nil {
		code = -1
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			buf.WriteString(err.Error())
		}
	}
	return buf.String(), code
}

// compare reports the first line where out disagrees with the step's shown
// output, or a missing or extra line.
func compare(b Block, s Step, out string, vars map[string]string) (Failure, bool) {
	got := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if out == "" {
		got = nil
	}
	for i := 0; i < len(s.Want) || i < len(got); i++ {
		switch {
		case i >= len(got):
			return Failure{Block: b, Line: s.Want[i].Num, Cmd: s.Cmd, Expected: s.Want[i].Text, Actual: "(no more output)"}, true
		case i >= len(s.Want):
			line := s.Line
			if len(s.Want) > 0 {
				line = s.Want[len(s.Want)-1].Num
			}
			return Failure{Block: b, Line: line, Cmd: s.Cmd, Expected: "(no more output)", Actual: got[i]}, true
		case !matchLine(s.Want[i].Text, got[i], vars):
			return Failure{Block: b, Line: s.Want[i].Num, Cmd: s.Cmd, Expected: s.Want[i].Text, Actual: got[i]}, true
		}
	}
	return Failure{}, false
}

// placeholders maps each run-to-run variable token to the pattern it matches.
var placeholders = map[string]string{
	"<DATE>":      `\d{4}-\d{2}-\d{2}`,
	"<TIMESTAMP>": `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})`,
	"<SHA>":       `[0-9a-f]{7,40}`,
}

var tokenRE = regexp.MustCompile(`<[A-Z]+>`)

// matchLine reports whether got matches want, a literal line that may hold
// placeholders. Trailing whitespace on got is ignored: the repo's Markdown
// hooks strip it from the doc, so a shown line can never carry it.
func matchLine(want, got string, vars map[string]string) bool {
	got = strings.TrimRight(got, " \t")
	if !strings.Contains(want, "<") {
		return want == got
	}
	var pat strings.Builder
	pat.WriteString("^")
	last := 0
	for _, loc := range tokenRE.FindAllStringIndex(want, -1) {
		tok := want[loc[0]:loc[1]]
		var sub string
		if p, ok := placeholders[tok]; ok {
			sub = p
		} else if v, ok := vars[tok]; ok {
			sub = regexp.QuoteMeta(v)
		} else {
			continue // not a placeholder: stays in the literal run
		}
		pat.WriteString(regexp.QuoteMeta(want[last:loc[0]]))
		pat.WriteString(sub)
		last = loc[1]
	}
	pat.WriteString(regexp.QuoteMeta(want[last:]))
	pat.WriteString("$")
	return regexp.MustCompile(pat.String()).MatchString(got)
}
