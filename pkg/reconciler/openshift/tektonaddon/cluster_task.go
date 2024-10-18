/*
Copyright 2022 The Tekton Authors

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

package tektonaddon

import (
	"fmt"
	"strings"
)

func formattedVersionMajorMinorX(version, x string) string {
	ver := getPatchVersionTrimmed(version)
	ver = fmt.Sprintf("%s.%s", ver, x)
	return formattedVersionSnake(ver)
}

func formattedVersionSnake(version string) string {
	ver := strings.TrimPrefix(version, "v")
	return strings.Replace(ver, ".", "-", -1)
}

// To get the minor major version for label i.e. v1.6
func getPatchVersionTrimmed(version string) string {
	endIndex := strings.LastIndex(version, ".")
	if endIndex != -1 {
		version = version[:endIndex]
	}
	return version
}
