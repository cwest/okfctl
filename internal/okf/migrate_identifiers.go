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
	"regexp"
	"strings"
)

// A v0.1 citation entry with no URL can still name its document exactly: a
// DOI, an arXiv id, a PubMed id or an ISBN. Each has one canonical resolver,
// so the §5.1 `resource` is built from the author's own identifier, offline
// and deterministically. Nothing is fetched and nothing is guessed: the URL is
// a fixed template over the identifier, and an ISBN must pass its check digit
// before it is linked. Whether the link resolves, or the record's title
// matches the entry, is a network question this code never asks.

var (
	// identDOIRe matches a DOI (prefix 10.<registrant>/<suffix>), bare or
	// after `doi:` / `DOI `. The leading group keeps it from matching inside a
	// path or a longer number (`foo/10.1234/x`, `v2.10.1234/x`).
	identDOIRe = regexp.MustCompile("(?:^|[^\\w/.])(10\\.\\d{4,9}/[^\\s<>\"`\\]]+)")
	// identArXivRe matches an arXiv id after an `arXiv:` or `arXiv ` label:
	// the 2007+ form (2312.06648, optional version) or the older
	// archive/number form (hep-th/9901001).
	identArXivRe = regexp.MustCompile(`(?i)\barXiv[:\s]\s*(\d{4}\.\d{4,5}(?:v\d+)?|[a-z-]+(?:\.[a-z]{2})?/\d{7}(?:v\d+)?)\b`)
	// identPMIDRe matches a PubMed id after a `PMID` label.
	identPMIDRe = regexp.MustCompile(`(?i)\bPMID:?\s*(\d{1,8})\b`)
	// identISBNRe matches an ISBN after an `ISBN`, `ISBN-10` or `ISBN-13`
	// label, digits optionally grouped with hyphens.
	identISBNRe = regexp.MustCompile(`(?i)\bISBN(?:-1[03])?:?\s*([0-9][0-9-]{7,16}[0-9X])\b`)
)

// identifierResource returns the one §5.1 resource that text's identifiers
// name. ok is false when the text carries no identifier, carries two that
// name different documents, or carries an ISBN that fails its checksum: an
// unlinkable identifier is still a document the author cited, so the entry
// stays for judgment rather than silently losing it.
func identifierResource(text string) (string, bool) {
	built, bad := identifierResources(text)
	if bad || len(built) != 1 {
		return "", false
	}
	return built[0], true
}

// identifierResources builds the resolver URL for every distinct identifier
// in text, in kind order (DOI, arXiv, PubMed, ISBN). bad reports an ISBN that
// failed its checksum and so was not built.
func identifierResources(text string) (built []string, bad bool) {
	seen := map[string]bool{}
	add := func(u string) {
		if k := canonicalResource(u); !seen[k] {
			seen[k] = true
			built = append(built, u)
		}
	}
	for _, m := range identDOIRe.FindAllStringSubmatch(text, -1) {
		add("https://doi.org/" + trimURL(m[1]))
	}
	for _, m := range identArXivRe.FindAllStringSubmatch(text, -1) {
		add("https://arxiv.org/abs/" + m[1])
	}
	for _, m := range identPMIDRe.FindAllStringSubmatch(text, -1) {
		add("https://pubmed.ncbi.nlm.nih.gov/" + m[1] + "/")
	}
	for _, m := range identISBNRe.FindAllStringSubmatch(text, -1) {
		isbn, ok := normalizeISBN(m[1])
		if !ok {
			bad = true
			continue
		}
		add("https://openlibrary.org/isbn/" + isbn)
	}
	return built, bad
}

// citationNames is every resource a citation entry names, as comparison keys
// (canonicalResource): its links and URLs plus the resolver URL of each of
// its identifiers. It answers "does another entry cite this document too?".
func citationNames(entry string) map[string]bool {
	out := map[string]bool{}
	rest := citationLinkSpanRe.ReplaceAllStringFunc(entry, func(span string) string {
		m := citationLinkSpanRe.FindStringSubmatch(span)
		if !strings.HasPrefix(span, "!") && !strings.HasPrefix(m[2], "#") {
			out[canonicalResource(m[2])] = true
		}
		return " "
	})
	for _, u := range citationURLRe.FindAllString(rest, -1) {
		out[canonicalResource(trimURL(u))] = true
	}
	built, _ := identifierResources(rest)
	for _, u := range built {
		out[canonicalResource(u)] = true
	}
	return out
}

// normalizeISBN strips hyphens and reports whether the result is a valid
// ISBN-10 (mod-11 check, X = 10 in the last place only) or ISBN-13 (978/979
// prefix, mod-10 check with weights 1,3).
func normalizeISBN(raw string) (string, bool) {
	s := strings.ToUpper(strings.ReplaceAll(raw, "-", ""))
	switch len(s) {
	case 10:
		sum := 0
		for i, c := range s {
			var d int
			switch {
			case c >= '0' && c <= '9':
				d = int(c - '0')
			case c == 'X' && i == 9:
				d = 10
			default:
				return "", false
			}
			sum += (10 - i) * d
		}
		return s, sum%11 == 0
	case 13:
		if !strings.HasPrefix(s, "978") && !strings.HasPrefix(s, "979") {
			return "", false
		}
		sum := 0
		for i, c := range s {
			if c < '0' || c > '9' {
				return "", false
			}
			w := 1
			if i%2 == 1 {
				w = 3
			}
			sum += w * int(c-'0')
		}
		return s, sum%10 == 0
	}
	return "", false
}

// canonicalResource maps a DOI URL to one comparison key: DOIs are
// case-insensitive, and doi.org and the legacy dx.doi.org (http or https)
// resolve the same name. Any other resource compares as written.
func canonicalResource(res string) string {
	for _, p := range []string{"https://doi.org/", "http://doi.org/", "https://dx.doi.org/", "http://dx.doi.org/"} {
		if len(res) > len(p) && strings.EqualFold(res[:len(p)], p) {
			return "https://doi.org/" + strings.ToLower(res[len(p):])
		}
	}
	return res
}
