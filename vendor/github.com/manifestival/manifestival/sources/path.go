package sources

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Parse parses YAML files into Unstructured objects.
//
// It supports 5 cases today:
// 1. pathname = path to a file --> parses that file.
// 2. pathname = path to a directory, recursive = false --> parses all files in
//    that directory.
// 3. pathname = path to a directory, recursive = true --> parses all files in
//    that directory and it's descendants
// 4. pathname = url --> fetches the contents of that URL and parses them as YAML.
// 5. pathname = combination of all previous cases, the string can contain
//    multiple records (file, directory or url) separated by comma
func Parse(pathname string, recursive bool) ([]unstructured.Unstructured, error) {

	pathnames := strings.Split(pathname, ",")
	aggregated := []unstructured.Unstructured{}
	for _, pth := range pathnames {
		els, err := read(pth, recursive)
		if err != nil {
			return nil, err
		}

		aggregated = append(aggregated, els...)
	}
	return aggregated, nil
}

// read cotains a logic to distinguish the type of record in pathname
// (file, directory or url) and calls the appropriate function
func read(pathname string, recursive bool) ([]unstructured.Unstructured, error) {
	if isURL(pathname) {
		return readURL(pathname)
	}

	info, err := os.Stat(pathname)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return readDir(pathname, recursive)
	}
	return readFile(pathname)
}

// readFile parses a single file.
func readFile(pathname string) ([]unstructured.Unstructured, error) {
	file, err := os.Open(pathname)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return Decode(file)
}

// readDir parses all files in a single directory and it's descendant directories
// if the recursive flag is set to true.
func readDir(pathname string, recursive bool) ([]unstructured.Unstructured, error) {
	list, err := ioutil.ReadDir(pathname)
	if err != nil {
		return nil, err
	}

	aggregated := []unstructured.Unstructured{}
	for _, f := range list {
		name := path.Join(pathname, f.Name())
		pathDirOrFile, err := os.Stat(name)
		var els []unstructured.Unstructured

		if os.IsNotExist(err) || os.IsPermission(err) {
			return aggregated, err
		}

		switch {
		case pathDirOrFile.IsDir() && recursive:
			els, err = readDir(name, recursive)
		case !pathDirOrFile.IsDir():
			els, err = readFile(name)
		}

		if err != nil {
			return nil, err
		}
		aggregated = append(aggregated, els...)
	}
	return aggregated, nil
}

// readURL fetches a URL and parses its contents as YAML.
func readURL(url string) ([]unstructured.Unstructured, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return Decode(resp.Body)
}

// isURL checks whether or not the given path parses as a URL.
func isURL(pathname string) bool {
	if _, err := os.Lstat(pathname); err == nil {
		return false
	}
	url, err := url.ParseRequestURI(pathname)
	return err == nil && url.Scheme != ""
}
