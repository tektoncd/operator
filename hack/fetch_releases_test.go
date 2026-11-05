/*
Copyright 2026 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hack_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const releaseConfig = `shipwright-build:
  github: shipwright-io/build
  version: v0.21.10
  openshift:
    - github: redhat-openshift-builds/strategies
      version: v0.21.0
      files: [strategies.yaml]
shared-resource:
  github: openshift/csi-driver-shared-resource
  version: v0.4.9
`

// Only curl is mocked: the real script and yq parse configuration and validate
// downloaded manifests, without network access or writes to the checkout.
const mockCurl = `#!/usr/bin/env bash
set -eu
while (($#)); do
  case "$1" in
    -o|--output) output=$2; shift 2 ;;
    -w|--write-out|--retry) shift 2 ;;
    -*) shift ;;
    *) url=$1; shift ;;
  esac
done
printf '%s\n' "$url" >> "$FETCH_TEST_LOG"
mode=valid
if [[ "${url##*/}" == "$FETCH_TEST_FILE" ]]; then
  mode=$FETCH_TEST_MODE
fi
case "$mode" in
  transport) exit 7 ;;
  http) printf 'unavailable' > "$output"; printf '503'; exit 0 ;;
  malformed) printf 'metadata: [\n' > "$output" ;;
  empty) : > "$output" ;;
  html) printf '<html>unavailable</html>\n' > "$output" ;;
  null) printf 'null\n---\nnull\n' > "$output" ;;
  invalid-first)
    printf 'not: a-resource\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: valid\n' > "$output" ;;
  missing-name) printf 'apiVersion: v1\nkind: ConfigMap\nmetadata: {}\n' > "$output" ;;
  numeric-name) printf 'apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: 123\n' > "$output" ;;
  *)
    printf 'apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: fetched\ndata:\n  source: "%s"\n---\n' "$url" > "$output" ;;
esac
printf '200'
`

func TestFetchCustomRelease(t *testing.T) {
	if _, err := exec.LookPath("yq"); err != nil {
		t.Skip("fetch-releases.sh tests require mikefarah/yq v4")
	}
	script, err := os.ReadFile("fetch-releases.sh")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		target    string
		component string
		version   string
		files     []string
		mode      string
		failFile  string
		config    string
		fresh     bool
		wantFiles []string
	}{
		{name: "OpenShift independent strategies pin", wantFiles: []string{"release.yaml", "strategies.yaml"}},
		{name: "Kubernetes sample strategies", target: "kubernetes", files: []string{"release.yaml", "sample-strategies.yaml"}, wantFiles: []string{"release.yaml", "sample-strategies.yaml"}},
		{name: "SharedResource without platform extras", component: "shared-resource", version: "0.4.9", wantFiles: []string{"release.yaml"}},
		{name: "fresh OpenShift payload", fresh: true, wantFiles: []string{"release.yaml", "strategies.yaml"}},
		{name: "fresh payload extra download failure", fresh: true, mode: "transport", failFile: "strategies.yaml"},
		{name: "primary transport failure", mode: "transport"},
		{name: "primary HTTP failure", mode: "http"},
		{name: "primary malformed YAML", mode: "malformed"},
		{name: "primary empty response", mode: "empty"},
		{name: "primary HTML response", mode: "html"},
		{name: "primary null documents", mode: "null"},
		{name: "primary invalid first document", mode: "invalid-first"},
		{name: "primary resource missing name", mode: "missing-name"},
		{name: "primary numeric resource name", mode: "numeric-name"},
		{name: "extra transport failure", mode: "transport", failFile: "strategies.yaml"},
		{name: "extra HTTP failure", mode: "http", failFile: "strategies.yaml"},
		{name: "extra malformed YAML", mode: "malformed", failFile: "strategies.yaml"},
		{name: "extra empty response", mode: "empty", failFile: "strategies.yaml"},
		{name: "extra HTML response", mode: "html", failFile: "strategies.yaml"},
		{name: "extra null documents", mode: "null", failFile: "strategies.yaml"},
		{name: "extra invalid first document", mode: "invalid-first", failFile: "strategies.yaml"},
		{name: "publication failure restores previous payload", mode: "publish"},
		{name: "invalid configuration", config: "shipwright-build: [\n"},
		{name: "missing primary repository", config: strings.Replace(releaseConfig, "github: shipwright-io/build", "unused: shipwright-io/build", 1)},
		{name: "missing primary version", config: strings.Replace(releaseConfig, "version: v0.21.10", "unused: v0.21.10", 1)},
		{name: "missing extra repository", config: strings.Replace(releaseConfig, "github: redhat-openshift-builds/strategies", "unused: redhat-openshift-builds/strategies", 1)},
		{name: "missing extra version", config: strings.Replace(releaseConfig, "version: v0.21.0", "unused: v0.21.0", 1)},
		{name: "missing extra files", config: strings.Replace(releaseConfig, "files: [strategies.yaml]", "unused: [strategies.yaml]", 1)},
		{name: "invalid extra configuration type", config: strings.Replace(releaseConfig, "openshift:", "openshift: unexpected\n  unused:", 1)},
		{name: "invalid primary version directory", config: strings.Replace(releaseConfig, "version: v0.21.10", "version: v..", 1)},
		{name: "invalid extra filename", config: strings.Replace(releaseConfig, "files: [strategies.yaml]", "files: [../strategies.yaml]", 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.target == "" {
				tt.target = "openshift"
			}
			if tt.component == "" {
				tt.component = "shipwright-build"
			}
			if tt.version == "" {
				tt.version = "0.21.10"
			}
			if tt.files == nil {
				tt.files = []string{"release.yaml"}
			}
			if tt.failFile == "" {
				tt.failFile = "release.yaml"
			}
			if tt.config == "" {
				tt.config = releaseConfig
			}
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "hack", "fetch-releases.sh"), script, 0o644)
			configPath := filepath.Join(root, "components.yaml")
			writeTestFile(t, configPath, []byte(tt.config), 0o644)
			writeTestFile(t, filepath.Join(root, "bin", "curl"), []byte(mockCurl), 0o755)
			if tt.mode == "publish" {
				mv, err := exec.LookPath("mv")
				if err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, filepath.Join(root, "bin", "mv"), []byte("#!/usr/bin/env bash\nif [[ $1 == */payload ]]; then exit 1; fi\nexec "+mv+" \"$@\"\n"), 0o755)
			}
			componentDir := filepath.Join(root, "cmd", tt.target, "operator", "kodata", tt.component)
			destination := filepath.Join(componentDir, tt.version)
			oldPayload := filepath.Join(destination, "old.yaml")
			if !tt.fresh {
				writeTestFile(t, oldPayload, []byte("previous payload\n"), 0o644)
			}
			otherPayload := filepath.Join(componentDir, "0.20.0", "other.yaml")
			writeTestFile(t, otherPayload, []byte("other version\n"), 0o644)
			logPath := filepath.Join(root, "requests.log")

			args := []string{"-c", `source "$1"; TARGET="$2"; CONFIG="$3"; release_yaml_custom "$4" "${@:5}"`, "fetch-test", filepath.Join(root, "hack", "fetch-releases.sh"), tt.target, configPath, tt.component}
			args = append(args, tt.files...)
			cmd := exec.Command("bash", args...)
			cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"), "FETCH_TEST_MODE="+tt.mode, "FETCH_TEST_FILE="+tt.failFile, "FETCH_TEST_LOG="+logPath)
			output, err := cmd.CombinedOutput()
			if tt.wantFiles == nil {
				if err == nil {
					t.Fatalf("expected fetch to fail, output:\n%s", output)
				}
				entries, readErr := os.ReadDir(destination)
				if tt.fresh {
					if !os.IsNotExist(readErr) {
						t.Fatalf("failed fresh fetch published a payload: entries=%v err=%v", entries, readErr)
					}
				} else if readErr != nil || len(entries) != 1 {
					t.Fatalf("failed fetch changed previous payload: entries=%v err=%v", entries, readErr)
				} else {
					assertTestFile(t, oldPayload, "previous payload\n")
				}
			} else {
				if err != nil {
					t.Fatalf("fetch failed: %v\n%s", err, output)
				}
				entries, err := os.ReadDir(destination)
				if err != nil {
					t.Fatal(err)
				}
				var names []string
				for _, entry := range entries {
					names = append(names, entry.Name())
				}
				if !slices.Equal(names, tt.wantFiles) {
					t.Fatalf("payload files=%v, want %v", names, tt.wantFiles)
				}
				requests, err := os.ReadFile(logPath)
				if err != nil {
					t.Fatal(err)
				}
				if tt.target == "openshift" && tt.component == "shipwright-build" && !strings.Contains(string(requests), "/redhat-openshift-builds/strategies/releases/download/v0.21.0/strategies.yaml") {
					t.Fatalf("OpenShift strategies did not use independently pinned version: %s", requests)
				}
				if tt.target == "kubernetes" && strings.Contains(string(requests), "redhat-openshift-builds") {
					t.Fatalf("Kubernetes fetched OpenShift extras: %s", requests)
				}
			}
			if tt.wantFiles == nil {
				assertTestFile(t, otherPayload, "other version\n")
			} else if _, err := os.Stat(otherPayload); !os.IsNotExist(err) {
				t.Fatalf("successful fetch retained stale payload %s: %v", otherPayload, err)
			}
			entries, err := os.ReadDir(filepath.Dir(componentDir))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), tt.component+".fetch-") {
					t.Errorf("temporary payload directory leaked: %s", entry.Name())
				}
			}
		})
	}
}

func writeTestFile(t *testing.T, path string, contents []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, mode); err != nil {
		t.Fatal(err)
	}
}

func assertTestFile(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Errorf("file %s=%q, want %q", path, data, want)
	}
}
