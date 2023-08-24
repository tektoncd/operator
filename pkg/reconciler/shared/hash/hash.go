/*
Copyright 2021 The Tekton Authors

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

package hash

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"golang.org/x/mod/sumdb/dirhash"
)

// Compute generates an unique hash/string for the object pass to it.
// with sha256
func Compute(obj interface{}) (string, error) {
	d, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	hashSha256 := sha256.New()
	hashSha256.Write(d)
	return fmt.Sprintf("%x", hashSha256.Sum(nil)), nil
}

// Compute generates an unique hash/string for the object pass to it.
// with md5
func ComputeMd5(obj interface{}) (string, error) {
	d, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	hashMd5 := md5.New()
	hashMd5.Write(d)
	return fmt.Sprintf("%x", hashMd5.Sum(nil)), nil
}

// computes has for the given directory, tasks the directory and files recursively
// "prefix" used internally to produce constant base path,
// actual location will be replaced with this prefix on hash calculation
func ComputeHashDir(dirLocation, prefix string) (string, error) {
	return dirhash.HashDir(dirLocation, prefix, dirhash.DefaultHash)
}
