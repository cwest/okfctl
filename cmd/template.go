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
	"fmt"
	"sort"
	"strings"

	"github.com/cwest/okfctl/internal/okf"
	"github.com/spf13/cobra"
)

func newTemplateCmd() *cobra.Command {
	templateCmd := &cobra.Command{
		Use:   "template",
		Short: "Read the type-template bundle (templates are authored as ordinary OKF nodes)",
	}

	var listFrom string
	listC := &cobra.Command{
		Use:   "list [bundle-dir]",
		Short: "List the type templates a bundle declares (target type, required fields, body sections)",
		Long: "template list shows the type templates a bundle declares. Templates are okfctl's " +
			"opt-in team overlay (PRD §9): they're authored as ordinary OKF nodes whose type is " +
			"`Type Template`, NOT a spec concept, and they never affect the spec floor. A bundle " +
			"may REFERENCE a separate type-template bundle via its .okf `templates: <path>` key " +
			"(or --templates-from, which overrides it); referenced templates are folded in and " +
			"local `Type Template` nodes overlay them (local wins per target type). Each row names " +
			"a target type, how many required fields and body sections its template defines, and " +
			"the source node it came from; a local entry that shadows a referenced one is flagged. " +
			"Read-only. A bundle with no templates prints a notice and exits zero.",
		Example: "  # List templates declared by the current bundle\n" +
			"  okfctl template list\n\n" +
			"  # List templates in a bundle elsewhere\n" +
			"  okfctl template list ./bundles/knowledge\n\n" +
			"  # Resolve templates from a specific bundle, overriding the .okf reference\n" +
			"  okfctl template list --templates-from ../okf-type-templates ./bundles/knowledge",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := resolveTemplatesForCmd(args, listFrom)
			if err != nil {
				return err
			}
			if len(rt.ByType) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no type templates in bundle")
				return nil
			}
			shadowed := map[string]bool{}
			for _, s := range rt.Shadowed {
				shadowed[s.TargetType] = true
			}
			targets := make([]string, 0, len(rt.ByType))
			for target := range rt.ByType {
				targets = append(targets, target)
			}
			sort.Strings(targets)
			for _, target := range targets {
				t := rt.ByType[target]
				shadowNote := ""
				if shadowed[target] {
					shadowNote = " [shadows referenced template]"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s	%d required field(s), %d body section(s)	(source: %s)%s\n",
					target, len(t.RequiredFields), len(t.BodySections), t.Source, shadowNote)
			}
			return nil
		},
	}
	listC.Flags().StringVar(&listFrom, "templates-from", "", templatesFromFlagUsage)
	templateCmd.AddCommand(listC)

	var showFrom string
	showC := &cobra.Command{
		Use:   "show <target-type> [bundle-dir]",
		Short: "Show a single type template's required/recommended fields and body sections",
		Long: "template show prints one type template in full: its target type, source node, and " +
			"its required fields, recommended fields, and body sections. Templates are okfctl's " +
			"opt-in team overlay (PRD §9), authored as ordinary OKF nodes — this command only " +
			"reads them and never mutates the bundle. The governing template may come from a " +
			"referenced bundle (the .okf `templates:` key, or --templates-from which overrides it) " +
			"or from a local `Type Template` node, with local winning per target type; `source` " +
			"names where it resolved from. It errors if no template governs the given type. See " +
			"what `okfctl validate --templates` will enforce and what `node new` will scaffold.",
		Example: "  # Show the template that governs the \"Runbook\" type\n" +
			"  okfctl template show Runbook\n\n" +
			"  # Show a template in a bundle elsewhere\n" +
			"  okfctl template show Runbook ./bundles/knowledge\n\n" +
			"  # Resolve from a specific template bundle, overriding the .okf reference\n" +
			"  okfctl template show Runbook --templates-from ../okf-type-templates ./bundles/knowledge",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			rt, err := resolveTemplatesForCmd(args[1:], showFrom)
			if err != nil {
				return err
			}
			t, ok := rt.ByType[target]
			if !ok {
				return fmt.Errorf("no template governs type %q", target)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "target_type: %s\n", t.TargetType)
			fmt.Fprintf(out, "source: %s\n", t.Source)
			fmt.Fprintf(out, "required_fields: %s\n", strings.Join(t.RequiredFields, ", "))
			fmt.Fprintf(out, "recommended_fields: %s\n", strings.Join(t.RecommendedFields, ", "))
			fmt.Fprintf(out, "body_sections: %s\n", strings.Join(t.BodySections, ", "))
			return nil
		},
	}
	showC.Flags().StringVar(&showFrom, "templates-from", "", templatesFromFlagUsage)
	templateCmd.AddCommand(showC)

	return templateCmd
}

// templatesFromFlagUsage is the shared help for the --templates-from flag so
// every command that resolves templates documents it identically.
const templatesFromFlagUsage = "resolve type templates from this bundle directory, overriding the .okf templates key (PRD §9.2)"

// resolveTemplatesForCmd loads the bundle from an optional trailing [bundle-dir]
// arg (defaulting to ".") and resolves its effective type templates: a referenced
// bundle (--templates-from, or the .okf `templates:` key) folded in, with local
// `Type Template` nodes overlaid (local wins per target type).
func resolveTemplatesForCmd(dirArgs []string, templatesFrom string) (okf.ResolvedTemplates, error) {
	dir := "."
	if len(dirArgs) == 1 {
		dir = dirArgs[0]
	}
	b, err := okf.Load(dir)
	if err != nil {
		return okf.ResolvedTemplates{}, fmt.Errorf("load bundle: %w", err)
	}
	return okf.ResolveTemplates(b, templatesFrom)
}
