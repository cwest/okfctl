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

	templateCmd.AddCommand(&cobra.Command{
		Use:   "list [bundle-dir]",
		Short: "List the type templates a bundle declares (target type, required fields, body sections)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tmpls, err := loadTemplates(args)
			if err != nil {
				return err
			}
			if len(tmpls) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no type templates in bundle")
				return nil
			}
			targets := make([]string, 0, len(tmpls))
			for target := range tmpls {
				targets = append(targets, target)
			}
			sort.Strings(targets)
			for _, target := range targets {
				t := tmpls[target]
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d required field(s), %d body section(s)\n",
					target, len(t.RequiredFields), len(t.BodySections))
			}
			return nil
		},
	})

	templateCmd.AddCommand(&cobra.Command{
		Use:   "show <target-type> [bundle-dir]",
		Short: "Show a single type template's required/recommended fields and body sections",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			tmpls, err := loadTemplates(args[1:])
			if err != nil {
				return err
			}
			t, ok := tmpls[target]
			if !ok {
				return fmt.Errorf("no template governs type %q", target)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "target_type: %s\n", t.TargetType)
			fmt.Fprintf(out, "source: %s\n", t.Path)
			fmt.Fprintf(out, "required_fields: %s\n", strings.Join(t.RequiredFields, ", "))
			fmt.Fprintf(out, "recommended_fields: %s\n", strings.Join(t.RecommendedFields, ", "))
			fmt.Fprintf(out, "body_sections: %s\n", strings.Join(t.BodySections, ", "))
			return nil
		},
	})

	return templateCmd
}

// loadTemplates loads the bundle from an optional trailing [bundle-dir] arg
// (defaulting to ".") and folds its type templates.
func loadTemplates(dirArgs []string) (map[string]okf.Template, error) {
	dir := "."
	if len(dirArgs) == 1 {
		dir = dirArgs[0]
	}
	b, err := okf.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("load bundle: %w", err)
	}
	return okf.Templates(b), nil
}
