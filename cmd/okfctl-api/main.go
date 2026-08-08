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

// Command okfctl-api is a PATH-dispatch plugin (git/kubectl style) that serves
// an OKF bundle as a plain REST + JSON HTTP API under /api/v1. Invoked as
// `okfctl api serve ...` via core's dispatch, or directly. It is a SEPARATE
// static binary from okfctl core and a sibling of okfctl-search: it links
// internal/okf (and, in later increments, internal/search) as a library rather
// than shelling out, which is the whole point of building it as a plugin — the
// API's view can never disagree with the CLI's because it calls the same
// functions.
//
// This binary is the walking skeleton (Increment 1): `serve` with GET
// /api/v1/stats and GET /api/v1/graph, bound to loopback by default. The node,
// collection, search, and freshness surfaces are later increments.
package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cwest/okfctl/internal/apiserver"
	"github.com/cwest/okfctl/internal/okf"
	"github.com/cwest/okfctl/internal/okfconfig"
	"github.com/cwest/okfctl/internal/search"
	"github.com/spf13/cobra"
)

func main() {
	if err := newAPICmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "okfctl-api:", err)
		os.Exit(1)
	}
}

func newAPICmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "okfctl-api",
		Short:         "Serve an OKF bundle as a REST + JSON HTTP API (okfctl plugin)",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newServeCmd())
	return root
}

func newServeCmd() *cobra.Command {
	var (
		addr         string
		embedderName string
		modelPath    string
	)
	c := &cobra.Command{
		Use:   "serve [bundle-dir]",
		Short: "Serve /api/v1/stats, /api/v1/graph, and /api/v1/search over HTTP for a bundle",
		Long: "serve starts a read-only HTTP API over an OKF bundle. It binds loopback " +
			"by default and REFUSES a non-loopback --addr: on this deployment the tailnet " +
			"+ loopback bind + Caddy boundary is the security model, so a misconfigured " +
			"0.0.0.0 must never accidentally expose the corpus. Every endpoint is a GET; " +
			"the bundle is treated as strictly read-only. /api/v1/search holds the loaded " +
			"index + embedder for the process lifetime and reloads on index change.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// §5: refuse a non-loopback bind BEFORE touching the filesystem or
			// opening a socket — the cheapest place to fail closed.
			if !isLoopback(addr) {
				return fmt.Errorf("refusing to bind non-loopback address %q: okfctl-api binds loopback only (the tailnet + loopback + Caddy boundary is the security model); use e.g. --addr 127.0.0.1:8931", addr)
			}
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			b, err := okf.Load(dir)
			if err != nil {
				return fmt.Errorf("load bundle: %w", err)
			}
			// Resolve the embedder /api/v1/search encodes queries with. It MUST
			// match the embedder the on-disk index was built under; the store's
			// model guard rejects a mismatch at query time rather than returning
			// silently-wrong vectors. Resolving fails fast at startup so a
			// misconfigured --embedder model2vec (no model path) is a clear error,
			// not a per-request 503.
			embedder, err := resolveEmbedder(embedderName, modelPath)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "okfctl-api: http://%s/api/v1 (Ctrl-C to stop)\n", addr)
			// ReadHeaderTimeout bounds slow-header (Slowloris) clients; the
			// bare http.ListenAndServe has none (gosec G114).
			srv := &http.Server{
				Addr:              addr,
				Handler:           apiserver.NewHandler(b, embedder),
				ReadHeaderTimeout: 10 * time.Second,
			}
			return srv.ListenAndServe()
		},
	}
	c.Flags().StringVar(&addr, "addr", "127.0.0.1:8931", "address to bind (loopback only; non-loopback is refused)")
	c.Flags().StringVar(&embedderName, "embedder", "hash", "embedder for /api/v1/search: hash (offline default) | model2vec (local static model); MUST match the index's model")
	c.Flags().StringVar(&modelPath, "model-path", "", "model2vec model directory (overrides the model_path config key)")
	return c
}

// resolveEmbedder returns the embedder named by --embedder, mirroring
// cmd/okfctl-search/main.go's resolver so the API and the search CLI agree on
// which embedder answers a query. "hash" is the zero-config offline default;
// "model2vec" loads a real static model from a local directory resolved
// flag-first, then config (`okfctl config set model_path`). okfctl never
// downloads a model at runtime, so an unset path is a clear, actionable error
// rather than a silent fallback to hash — a query answered by the wrong embedder
// is worse than one that refuses to run.
func resolveEmbedder(name, modelPath string) (search.Embedder, error) {
	switch name {
	case "hash":
		return search.NewHashEmbedder(), nil
	case "model2vec":
		dir := modelPath
		if dir == "" {
			cfg, err := okfconfig.Load()
			if err != nil {
				return nil, fmt.Errorf("reading okfctl config: %w", err)
			}
			dir = cfg["model_path"]
		}
		if dir == "" {
			return nil, fmt.Errorf("--embedder model2vec needs a local model directory: run `okfctl config set model_path <dir>` or pass --model-path <dir>")
		}
		e, err := search.LoadModel2VecEmbedder(dir)
		if err != nil {
			return nil, fmt.Errorf("loading model2vec model from %s: %w", dir, err)
		}
		return e, nil
	default:
		return nil, fmt.Errorf("unknown embedder %q (available: hash, model2vec)", name)
	}
}

// isLoopback reports whether addr (host:port) binds only the loopback
// interface. A bare host that is not an IP (e.g. "localhost") is accepted only
// when it resolves exclusively to loopback addresses; a name that resolves to
// any routable address is treated as non-loopback. An empty host ("" from
// ":8931") means all interfaces and is refused.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "" {
		return false // ":port" listens on all interfaces
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	// A hostname: accept only when every resolved address is loopback, so a
	// name that resolves off-box can't slip past the guard. "localhost" is the
	// common case and resolves to 127.0.0.1 / ::1.
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		// Unresolvable: fall back to the literal-name convention. Treat only
		// the canonical loopback name as loopback.
		return strings.EqualFold(host, "localhost")
	}
	for _, ip := range ips {
		if !ip.IsLoopback() {
			return false
		}
	}
	return true
}
