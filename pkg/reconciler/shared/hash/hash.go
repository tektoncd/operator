package hash

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// ComputeHashOf generates an unique hash/string for the
// object pass to it.
func Compute(obj interface{}) (string, error) {
	h := sha256.New()
	d, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	h.Write(d)
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
