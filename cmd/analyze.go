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
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

func newAnalyzeCmd() *cobra.Command {
	var jsonOut bool
	var staleDays int
	var timeSensitiveFraction float64
	var thinLines int
	var clusterMin int
	var coverageThreshold int
	var noIgnore *bool
	c := &cobra.Command{
		Use:   "analyze [bundle-dir]",
		Short: "Report where a bundle is WEAK: freshness, clusters, gaps, connectivity, structure",
		Long: "analyze is a proactive curation REPORT, not a gate. Where lint answers " +
			"\"is this corpus broken?\" (a CI gate: --strict exits non-zero), analyze answers " +
			"\"where is this corpus weak?\" across five dimensions — coverage gaps, freshness/" +
			"staleness, connectivity, tag-cluster synthesis candidates, and structure. It never " +
			"mutates the bundle.\n\n" +
			"Report semantics: analyze exits 0 whenever the analysis runs successfully, regardless " +
			"of how many findings it produces. The exit code reflects whether the analysis " +
			"succeeded, not whether the corpus is perfect. There is deliberately no --strict flag; " +
			"use lint --strict for a gate.\n\n" +
			"Pass --json for the machine path (the curation sweep files research cards from it).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) == 1 {
				dir = args[0]
			}
			b, err := loadBundleForCmd(cmd, dir, *noIgnore)
			if err != nil {
				return err
			}
			rep := okf.Analyze(b, okf.AnalyzeOptions{
				StaleDays:             staleDays,
				TimeSensitiveFraction: timeSensitiveFraction,
				ThinLines:             thinLines,
				ClusterMin:            clusterMin,
				CoverageThreshold:     coverageThreshold,
			})
			if jsonOut {
				enc, err := json.MarshalIndent(rep, "", "  ")
				if err != nil {
					return fmt.Errorf("marshal report: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(enc))
				return nil
			}
			writeAnalyzeHuman(cmd.OutOrStdout(), rep)
			return nil
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, "emit machine-readable JSON instead of the human report")
	c.Flags().IntVar(&staleDays, "stale-days", 0, "age (days) past which a node's modified/created is flagged stale (default 180)")
	c.Flags().Float64Var(&timeSensitiveFraction, "time-sensitive-fraction", 0, "surface a time-sensitive node once its age >= fraction*stale-days (default 0.5); undated marked nodes always surface")
	c.Flags().IntVar(&thinLines, "thin-lines", 0, "body line count below which a node is thin (default 15)")
	c.Flags().IntVar(&clusterMin, "cluster-min", 0, "min nodes sharing a tag to flag a synthesis cluster (default 3)")
	c.Flags().IntVar(&coverageThreshold, "coverage-threshold", 0, "min distinct nodes mentioning a term to report a coverage gap (default 3)")
	noIgnore = addNoIgnoreFlag(c)
	return c
}

// writeAnalyzeHuman renders the report as a human-readable, terminal-friendly
// text report. Empty dimensions show "✓ none" so the reader sees the full
// curation surface, not just what tripped.
func writeAnalyzeHuman(w io.Writer, rep okf.AnalyzeReport) {
	total := 0

	fmt.Fprintln(w, "# OKF Corpus Analysis")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%d node(s), %d internal link(s). Stale threshold: %dd.\n",
		rep.Summary.Nodes, rep.Summary.TotalInternalLinks, rep.Summary.StaleThresholdDays)

	section := func(title string, n int, body func()) {
		total += n
		fmt.Fprintf(w, "\n## %s\n", title)
		if n == 0 {
			fmt.Fprintln(w, "  ✓ none")
			return
		}
		body()
	}

	// Coverage.
	cg := rep.Coverage
	section("Coverage gaps — dangling forward-links (research opportunities)", len(cg.DanglingLinks), func() {
		for _, d := range cg.DanglingLinks {
			fmt.Fprintf(w, "  - %s → missing: %s\n", d.From, d.Target)
		}
	})
	section("Coverage gaps — thin nodes (need expansion)", len(cg.ThinNodes), func() {
		for _, t := range cg.ThinNodes {
			fmt.Fprintf(w, "  - %s (%d lines)\n", t.Path, t.BodyLines)
		}
	})
	section("Coverage gaps — uncited nodes (need sourcing)", len(cg.Uncited), func() {
		for _, u := range cg.Uncited {
			fmt.Fprintf(w, "  - %s\n", u.Path)
		}
	})
	section("Coverage gaps — single-citation nodes (corroborate)", len(cg.SingleCitation), func() {
		for _, s := range cg.SingleCitation {
			fmt.Fprintf(w, "  - %s\n", s.Path)
		}
	})
	section("Coverage gaps — known terms with no node (delegated to lint)", len(cg.KnownGaps), func() {
		for _, m := range cg.KnownGaps {
			fmt.Fprintf(w, "  - %s\n", m)
		}
	})

	// Freshness.
	fr := rep.Freshness
	section("Freshness — stale / undated nodes (revalidate)", len(fr.Stale), func() {
		for _, s := range fr.Stale {
			fmt.Fprintf(w, "  - %s (age: %s, basis: %s)\n", s.Path, ageStr(s.AgeDays), s.Basis)
		}
	})
	section("Freshness — time-sensitive bodies aged past the gate (re-verify claims)", len(fr.TimeSensitive), func() {
		for _, t := range fr.TimeSensitive {
			fmt.Fprintf(w, "  - %s (age: %s, markers: %s)\n", t.Path, ageStr(t.AgeDays), strings.Join(t.Markers, ", "))
		}
	})

	// Connectivity.
	co := rep.Connectivity
	section("Connectivity — orphan nodes (wire into the graph)", len(co.Orphans), func() {
		for _, o := range co.Orphans {
			fmt.Fprintf(w, "  - %s\n", o.Path)
		}
	})
	section("Connectivity — weakly-linked nodes", len(co.WeaklyLinked), func() {
		for _, wl := range co.WeaklyLinked {
			fmt.Fprintf(w, "  - %s (out:%d in:%d)\n", wl.Path, wl.Out, wl.In)
		}
	})

	// Clusters.
	section("Clusters — synthesis-overview candidates", len(rep.Clusters), func() {
		for _, cl := range rep.Clusters {
			fmt.Fprintf(w, "  - tag '%s': %d nodes %v\n", cl.Tag, len(cl.Nodes), cl.Nodes)
		}
	})

	// Structure.
	st := rep.Structure
	section("Structure — duplicate/near-duplicate titles (merge candidates)", len(st.DuplicateTitles), func() {
		for _, d := range st.DuplicateTitles {
			fmt.Fprintf(w, "  - %s\n", strings.Join(d.Members, " ≈ "))
		}
	})
	section("Structure — near-duplicate slugs (rename/merge candidates)", len(st.NearDuplicateSlugs), func() {
		for _, s := range st.NearDuplicateSlugs {
			fmt.Fprintf(w, "  - %s ≈ %s\n", s.A, s.B)
		}
	})

	fmt.Fprintf(w, "\n---\n%d actionable signal(s) across all dimensions.\n", total)
}

// ageStr renders an optional age-in-days as "Nd" or "n/a" (undated).
func ageStr(age *int) string {
	if age == nil {
		return "n/a"
	}
	return fmt.Sprintf("%dd", *age)
}
