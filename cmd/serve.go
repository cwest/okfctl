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
		w.Write(indexHTML)
	})

	return mux
}

func newServeCmd() *cobra.Command {
	var addr string
	c := &cobra.Command{
		Use:   "serve [bundle-dir]",
		Short: "Serve an interactive web visualization of the bundle graph",
		Long: "serve starts a local web server that renders the bundle as an interactive, " +
			"navigable knowledge graph. Assets are embedded in the binary — no separate " +
			"install. Binds loopback by default; override with --addr.",
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
			return http.ListenAndServe(addr, newServeHandler(b))
		},
	}
	c.Flags().StringVar(&addr, "addr", "127.0.0.1:8080", "address to bind (loopback by default)")
	return c
}
