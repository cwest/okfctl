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
	"errors"
	"os"
	"os/exec"

	"github.com/cwest/okfctl/internal/plugin"
)

// dispatch runs the okfctl-<name> plugin resolved on pathenv, passing args
// through with stdio inherited and OKFCTL set to this executable's path so the
// plugin can call back into core. It returns the child's exit code (0 on clean
// exit, the child's code on a non-zero exit) WITHOUT calling os.Exit, so it is
// unit-testable. err is non-nil only when the plugin cannot be found or launched
// (not when the child merely exits non-zero — that fidelity is carried by code).
func dispatch(name string, args []string, pathenv string) (int, error) {
	path, ok := plugin.Lookup(name, pathenv)
	if !ok {
		return 1, errors.New("okfctl-" + name + " not found on PATH")
	}

	cmd := exec.Command(path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	env := os.Environ()
	if self, err := os.Executable(); err == nil {
		env = append(env, "OKFCTL="+self)
	}
	cmd.Env = env

	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		// Child ran and exited non-zero: propagate its code, not an error.
		return exitErr.ExitCode(), nil
	}
	// Failed to launch (permissions, etc.).
	return 1, err
}
