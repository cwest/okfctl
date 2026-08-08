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

// Package okfconfig is the ONE okfctl configuration store: a flat JSON map at
// ~/.config/okfctl/config.json (override with OKFCTL_CONFIG_HOME). It is shared
// by core okfctl (the `config` command) and plugins (e.g. okfctl-search reads
// model_path from here), so there is a single config mechanism, not two. stdlib
// only — JSON, no TOML dependency (keeps okfctl zero-dependency).
package okfconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Path returns the config file location: $OKFCTL_CONFIG_HOME/config.json, else
// <user config dir>/okfctl/config.json, else ./.okfctl/config.json.
func Path() string {
	home := os.Getenv("OKFCTL_CONFIG_HOME")
	if home == "" {
		if h, err := os.UserConfigDir(); err == nil {
			home = filepath.Join(h, "okfctl")
		} else {
			home = ".okfctl"
		}
	}
	return filepath.Join(home, "config.json")
}

// Load reads the config map. A missing file is not an error — it returns an
// empty map so callers can treat "unset" and "empty" identically.
func Load() (map[string]string, error) {
	m := map[string]string{}
	data, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// Save writes the config map as indented JSON, creating the parent dir.
func Save(m map[string]string) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}
