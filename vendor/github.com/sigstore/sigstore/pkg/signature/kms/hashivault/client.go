//
// Copyright 2021 The Sigstore Authors.
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

package hashivault

import (
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ReneKroon/ttlcache/v2"
	vault "github.com/hashicorp/vault/api"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
)

type hashivaultClient struct {
	client                  *vault.Client
	keyPath                 string
	transitSecretEnginePath string
	keyCache                *ttlcache.Cache
}

var (
	errReference   = errors.New("kms specification should be in the format hashivault://<key>")
	referenceRegex = regexp.MustCompile(`^hashivault://(?P<path>\w(([\w-.]+)?\w)?)$`)
)

const (
	vaultV1DataPrefix = "vault:v1:"

	// use a consistent key for cache lookups
	CacheKey = "signer"

	ReferenceScheme = "hashivault://"
)

func ValidReference(ref string) error {
	if !referenceRegex.MatchString(ref) {
		return errReference
	}
	return nil
}

func parseReference(resourceID string) (keyPath string, err error) {
	i := referenceRegex.SubexpIndex("path")
	v := referenceRegex.FindStringSubmatch(resourceID)
	if len(v) < i+1 {
		err = errors.Errorf("invalid vault format %q", resourceID)
		return
	}
	keyPath = v[i]
	return
}

func newHashivaultClient(keyResourceID string) (*hashivaultClient, error) {
	keyPath, err := parseReference(keyResourceID)
	if err != nil {
		return nil, err
	}

	address := os.Getenv("VAULT_ADDR")
	if address == "" {
		return nil, errors.New("VAULT_ADDR is not set")
	}

	token := os.Getenv("VAULT_TOKEN")
	if token == "" {
		log.Printf("VAULT_TOKEN is not set, trying to read token from file at path ~/.vault-token")
		homeDir, err := homedir.Dir()
		if err != nil {
			return nil, errors.Wrap(err, "get home directory")
		}

		tokenFromFile, err := os.ReadFile(filepath.Join(homeDir, ".vault-token"))
		if err != nil {
			return nil, errors.Wrap(err, "read .vault-token file")
		}

		token = string(tokenFromFile)
	}

	client, err := vault.NewClient(&vault.Config{
		Address: address,
	})
	if err != nil {
		return nil, errors.Wrap(err, "new vault client")
	}

	client.SetToken(token)

	transitSecretEnginePath := os.Getenv("TRANSIT_SECRET_ENGINE_PATH")
	if transitSecretEnginePath == "" {
		transitSecretEnginePath = "transit"
	}

	hvClient := &hashivaultClient{
		client:                  client,
		keyPath:                 keyPath,
		transitSecretEnginePath: transitSecretEnginePath,
		keyCache:                ttlcache.NewCache(),
	}
	hvClient.keyCache.SetLoaderFunction(hvClient.keyCacheLoaderFunction)
	hvClient.keyCache.SkipTTLExtensionOnHit(true)

	return hvClient, nil
}

func (h *hashivaultClient) keyCacheLoaderFunction(key string) (data interface{}, ttl time.Duration, err error) {
	ttl = time.Second * 300
	var pubKey crypto.PublicKey
	pubKey, err = h.fetchPublicKey(context.Background())
	if err != nil {
		data = nil
		return
	}
	data = pubKey
	return data, ttl, err
}

func (h *hashivaultClient) fetchPublicKey(_ context.Context) (crypto.PublicKey, error) {
	client := h.client.Logical()

	keyResult, err := client.Read(fmt.Sprintf("/%s/keys/%s", h.transitSecretEnginePath, h.keyPath))
	if err != nil {
		return nil, errors.Wrap(err, "public key")
	}

	keysData, hasKeys := keyResult.Data["keys"]
	latestVersion, hasVersion := keyResult.Data["latest_version"]
	if !hasKeys || !hasVersion {
		return nil, errors.New("Failed to read transit key keys: corrupted response")
	}

	keys, ok := keysData.(map[string]interface{})
	if !ok {
		return nil, errors.New("Failed to read transit key keys: Invalid keys map")
	}

	keyVersion := latestVersion.(json.Number)
	keyData, ok := keys[string(keyVersion)]
	if !ok {
		return nil, errors.New("Failed to read transit key keys: corrupted response")
	}

	publicKeyPem, ok := keyData.(map[string]interface{})["public_key"]
	if !ok {
		return nil, errors.New("Failed to read transit key keys: corrupted response")
	}

	return cryptoutils.UnmarshalPEMToPublicKey([]byte(publicKeyPem.(string)))
}

func (h *hashivaultClient) public() (crypto.PublicKey, error) {
	return h.keyCache.Get(CacheKey)
}

func (h hashivaultClient) sign(digest []byte, alg crypto.Hash) ([]byte, error) {
	client := h.client.Logical()

	signResult, err := client.Write(fmt.Sprintf("/%s/sign/%s%s", h.transitSecretEnginePath, h.keyPath, hashString(alg)), map[string]interface{}{
		"input":     base64.StdEncoding.Strict().EncodeToString(digest),
		"prehashed": alg != crypto.Hash(0),
	})
	if err != nil {
		return nil, errors.Wrap(err, "Transit: failed to sign payload")
	}

	encodedSignature, ok := signResult.Data["signature"]
	if !ok {
		return nil, errors.New("Transit: response corrupted in-transit")
	}

	return vaultDecode(encodedSignature)

}

func (h hashivaultClient) verify(sig, digest []byte, alg crypto.Hash) error {
	client := h.client.Logical()
	encodedSig := base64.StdEncoding.EncodeToString(sig)

	result, err := client.Write(fmt.Sprintf("/%s/verify/%s/%s", h.transitSecretEnginePath, h.keyPath, hashString(alg)), map[string]interface{}{
		"input":     base64.StdEncoding.EncodeToString(digest),
		"signature": fmt.Sprintf("%s%s", vaultV1DataPrefix, encodedSig),
	})

	if err != nil {
		return errors.Wrap(err, "verify")
	}

	valid, ok := result.Data["valid"]
	if !ok {
		return errors.New("corrupted response")
	}

	if isValid, ok := valid.(bool); ok && isValid {
		return errors.New("Failed vault verification")
	}
	return nil
}

// Vault likes to prefix base64 data with a version prefix
func vaultDecode(data interface{}) ([]byte, error) {
	encoded, ok := data.(string)
	if !ok {
		return nil, errors.New("Received non-string data")
	}
	return base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, vaultV1DataPrefix))
}

func hashString(h crypto.Hash) string {
	var hashStr string
	switch h {
	case crypto.SHA224:
		hashStr = "/sha2-224"
	case crypto.SHA256:
		hashStr = "/sha2-256"
	case crypto.SHA384:
		hashStr = "/sha2-384"
	case crypto.SHA512:
		hashStr = "/sha2-512"
	default:
		hashStr = ""
	}
	return hashStr
}

func (h hashivaultClient) createKey(typeStr string) (crypto.PublicKey, error) {
	client := h.client.Logical()

	if _, err := client.Write(fmt.Sprintf("/%s/keys/%s", h.transitSecretEnginePath, h.keyPath), map[string]interface{}{
		"type": typeStr,
	}); err != nil {
		return nil, errors.Wrap(err, "Failed to create transit key")
	}
	return h.public()
}
