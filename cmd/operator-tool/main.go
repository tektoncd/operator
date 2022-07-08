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
package main

import (
	"flag"
	"fmt"
	"os"
)

var (
	config string
)

func main() {
	flag.StringVar(&config, "config", "", "components configuration to load")

	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		os.Exit(1)
	}
	var err error
	switch args[0] {
	case "component-version":
		err = componentVersion(config, args[1:])
	case "check":
		err = check(config, false)
	case "check-bugfix":
		err = check(config, true)
	default:
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
