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

package main

import (
	"bytes"
	"testing"
)

func runPlugin(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newAPICmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// The loopback bind guard (§5): a non-loopback --addr is refused before any
// net.Listen, so a misconfigured 0.0.0.0 can never accidentally expose the
// corpus on the network. This is the same is_loopback discipline office_server.py
// enforces, made a real check rather than a doc comment.
func TestIsLoopback(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8931", true},
		{"127.0.0.1:0", true},
		{"[::1]:8931", true},
		{"localhost:8931", true},
		{":8931", false},        // all interfaces
		{"0.0.0.0:8931", false}, // all interfaces, explicit
		{"192.168.1.10:8931", false},
		{"example.com:8931", false},
	}
	for _, c := range cases {
		if got := isLoopback(c.addr); got != c.want {
			t.Errorf("isLoopback(%q) = %v, want %v", c.addr, got, c.want)
		}
	}
}

func TestServe_RefusesNonLoopbackAddr(t *testing.T) {
	out, err := runPlugin(t, "serve", "--addr", "0.0.0.0:8931", t.TempDir())
	if err == nil {
		t.Fatalf("serve --addr 0.0.0.0 should be refused, got no error (out=%q)", out)
	}
	if !bytes.Contains([]byte(err.Error()+out), []byte("loopback")) {
		t.Errorf("refusal should mention loopback; got err=%v out=%q", err, out)
	}
}

func TestServe_RefusesMissingBundle(t *testing.T) {
	// A bundle path that does not exist must fail cleanly, not panic.
	_, err := runPlugin(t, "serve", "--addr", "127.0.0.1:0", "/no/such/bundle/here")
	if err == nil {
		t.Fatalf("serve against a missing bundle should error")
	}
}
