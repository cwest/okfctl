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
	_ "embed"
	"fmt"
	"net/http"
	"time"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

//go:embed assets/index.html
var indexHTML []byte

// newServeHandler builds the read-only HTTP handler for a loaded bundle:
//
//	GET /            -> the embedded single-page visualizer
//	GET /graph.json  -> BuildGraph(b) as JSON (same serializer as graph export)
func newServeHandler(b *okf.Bundle) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/graph.json", func(w http.ResponseWriter, r *http.Request) {
		out, err := graphJSON(okf.BuildGraph(b))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprintln(w, out)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// ServeMux's "/" is a catch-all; only the exact root serves the page.
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Write errors here mean the client hung up mid-response; nothing to
		// recover, and headers are already sent so http.Error is unavailable.
		_, _ = w.Write(indexHTML)
	})

	return mux
}

func newServeCmd() *cobra.Command {
	var addr string
	c := &cobra.Command{
		Use:   "serve [bundle-dir]",
		Short: "Draw the bundle's link graph in your browser",
		Long: "serve runs a web server on 127.0.0.1:8080 that draws the bundle's link graph in " +
			"your browser, where you can click a node to see its type and links and filter the " +
			"graph by type or by top-level directory. Dashed nodes are orphans. The page is part " +
			"of the binary.\n\n" +
			"serve loads the bundle at startup and never edits it. Restart it to pick up a " +
			"change. It listens only on your own machine unless you pass --addr.",
		Example: "  # Serve the bundle in the current directory on http://127.0.0.1:8080\n" +
			"  okfctl serve\n\n" +
			"  # Serve a bundle elsewhere on another port\n" +
			"  okfctl serve --addr 127.0.0.1:9000 ./mykb",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			b, err := okf.Load(dir)
			if err != nil {
				return fmt.Errorf("load bundle: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "okfctl serve: http://%s (Ctrl-C to stop)\n", addr)
			// ReadHeaderTimeout bounds slow-header (Slowloris) clients; the
			// bare http.ListenAndServe has none (gosec G114).
			srv := &http.Server{
				Addr:              addr,
				Handler:           newServeHandler(b),
				ReadHeaderTimeout: 10 * time.Second,
			}
			return srv.ListenAndServe()
		},
	}
	c.Flags().StringVar(&addr, "addr", "127.0.0.1:8080", "address to bind (loopback by default)")
	return c
}
