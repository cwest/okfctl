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

// Package okf is the stable, read-only Go facade for consuming an Open Knowledge
// Format (OKF) bundle. It is the supported front door for a Go program that wants
// to load, validate, lint, search, and analyze a bundle without shelling out to
// the okfctl binary and parsing its text.
//
// # A pure delegation layer
//
// Every function here forwards to the tool's internal implementation with no
// logic of its own, and every domain type is a Go type ALIAS
// (type Bundle = internal.Bundle), never a copy. That is a deliberate contract:
// behavior is defined exactly once, so this facade and the okfctl CLI can never
// disagree, and a value obtained here is the very same type the tool uses
// internally — there is one Bundle in a program, not two that can drift.
//
// The OKF specification (upstream: GoogleCloudPlatform/knowledge-catalog
// okf/SPEC.md, v0.2) is the authority for what these functions do; this package
// adds no spec-defined behavior. Where a function enforces a spec-mandated rule,
// the internal implementation cites the section (e.g. Validate enforces the §6.2
// / §7.1 floor); consult that source for the normative definition.
//
// # Read-only by design — no write path
//
// This facade re-exports NO mutating operation. There is deliberately no New,
// Move, Touch, index build, or log append here: those live in the internal
// package and stay internal in this release. The omission is a scope boundary,
// not an oversight or a "coming soon" — a stable READ contract is what a
// consumer can safely depend on today, while the write path's contract is not
// yet frozen. Exercising the entire facade over a bundle never mutates it.
//
// # Stability commitment
//
// The exported names and their signatures are the stable surface. Because the
// types are aliases, their fields are governed by the underlying domain types;
// additive changes (new fields, new functions) may occur, but the read-only,
// delegation-only character of this package will not change without a major
// version. New capabilities are added by delegating to a new internal function,
// never by growing branching logic here.
package okf
