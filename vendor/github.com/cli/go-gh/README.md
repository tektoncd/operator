# go-gh beta

**This project is in beta!** Feedback is welcome.

A Go module for CLI Go applications and [gh extensions][extensions] that want a convenient way to interact with [gh][], and the GitHub API using [gh][] environment configuration.

`go-gh` supports multiple ways of getting access to `gh` functionality:

* Helpers that automatically read a `gh` config to authenticate themselves
* `gh.Exec` shells out to a `gh` install on your machine

If you'd like to use `go-gh` on systems without `gh` installed and configured, you can provide custom authentication details to the `go-gh` API helpers.


## Installation
```bash
go get github.com/cli/go-gh
```

## Usage
```golang
package main
import (
	"fmt"
	"github.com/cli/go-gh"
)

func main() {
	// These examples assume `gh` is installed and has been authenticated

	// Execute `gh issue list -R cli/cli`, and print the output.
	args := []string{"issue", "list", "-R", "cli/cli"}
	stdOut, _, err := gh.Exec(args...)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(stdOut.String())
	
	// Use an API helper to grab repository tags
	client, err := gh.RESTClient(nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	response := []struct{ Name string }{}
	err = client.Get("repos/cli/cli/tags", &response)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(response)
}
```

See [examples][examples] for more use cases.

## Reference Documentation

Full reference docs can be found on [pkg.go.dev](https://pkg.go.dev/github.com/cli/go-gh).

## Contributing

If anything feels off, or if you feel that some functionality is missing, please check out the [contributing page][contributing]. There you will find instructions for sharing your feedback, and submitting pull requests to the project.

[extensions]: https://github.com/topics/gh-extension
[gh]: https://github.com/cli/cli
[examples]: ./example_gh_test.go
[contributing]: ./.github/CONTRIBUTING.md
