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

package azure

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/ReneKroon/ttlcache/v2"
	"github.com/pkg/errors"
	jose "gopkg.in/square/go-jose.v2"

	kvauth "github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.1/keyvault"
	"github.com/Azure/go-autorest/autorest/to"
)

type azureVaultClient struct {
	client    *keyvault.BaseClient
	keyCache  *ttlcache.Cache
	vaultURL  string
	vaultName string
	keyName   string
}

var (
	errAzureReference = errors.New("kms specification should be in the format azurekms://[VAULT_NAME][VAULT_URL]/[KEY_NAME]")

	referenceRegex = regexp.MustCompile(`^azurekms://([^/]+)/([^/]+)?$`)
)

const (
	ReferenceScheme = "azurekms://"
	CacheKey        = "azure_vault_signer"
)

func ValidReference(ref string) error {
	if !referenceRegex.MatchString(ref) {
		return errAzureReference
	}
	return nil
}

func parseReference(resourceID string) (vaultURL, vaultName, keyName string, err error) {
	v := referenceRegex.FindStringSubmatch(resourceID)
	if len(v) != 3 {
		err = errors.Errorf("invalid azurekms format %q", resourceID)
		return
	}

	vaultURL = fmt.Sprintf("https://%s/", v[1])
	vaultName, keyName = strings.Split(v[1], ".")[0], v[2]
	return
}

func newAzureKMS(ctx context.Context, keyResourceID string) (*azureVaultClient, error) {
	vaultURL, vaultName, keyName, err := parseReference(keyResourceID)
	if err != nil {
		return nil, err
	}

	azureTenantID := os.Getenv("AZURE_TENANT_ID")
	if azureTenantID == "" {
		return nil, errors.New("AZURE_TENANT_ID is not set")
	}

	azureClientID := os.Getenv("AZURE_CLIENT_ID")
	if azureClientID == "" {
		return nil, errors.New("AZURE_CLIENT_ID is not set")
	}

	azureClientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	if azureClientSecret == "" {
		return nil, errors.New("AZURE_CLIENT_SECRET is not set")
	}

	client, err := getKeysClient()
	if err != nil {
		return nil, errors.Wrap(err, "new azure kms client")
	}

	azClient := &azureVaultClient{
		client:    &client,
		vaultURL:  vaultURL,
		vaultName: vaultName,
		keyName:   keyName,
		keyCache:  ttlcache.NewCache(),
	}

	azClient.keyCache.SetLoaderFunction(azClient.keyCacheLoaderFunction)
	azClient.keyCache.SkipTTLExtensionOnHit(true)

	return azClient, nil
}

func getKeysClient() (keyvault.BaseClient, error) {
	keyClient := keyvault.New()

	authorizer, err := kvauth.NewAuthorizerFromEnvironment()
	if err != nil {
		return keyvault.BaseClient{}, err
	}

	keyClient.Authorizer = authorizer
	err = keyClient.AddToUserAgent("sigstore")
	if err != nil {
		return keyvault.BaseClient{}, err
	}

	return keyClient, nil
}

func (a *azureVaultClient) keyCacheLoaderFunction(key string) (data interface{}, ttl time.Duration, err error) {
	ttl = time.Second * 300
	var pubKey crypto.PublicKey

	pubKey, err = a.fetchPublicKey(context.Background())
	if err != nil {
		data = nil
		return
	}

	data = pubKey
	return data, ttl, err
}

func (a *azureVaultClient) fetchPublicKey(ctx context.Context) (crypto.PublicKey, error) {
	key, err := a.getKey(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "public key")
	}

	jwkJSON, err := json.Marshal(*key.Key)
	if err != nil {
		return nil, errors.Wrap(err, "encoding the jsonWebKey")
	}

	jwk := jose.JSONWebKey{}
	err = jwk.UnmarshalJSON(jwkJSON)
	if err != nil {
		return nil, errors.Wrap(err, "decoding the jsonWebKey")
	}

	pub, ok := jwk.Key.(*ecdsa.PublicKey)
	if !ok {
		if err != nil {
			return nil, fmt.Errorf("public key was not ECDSA: %#v", pub)
		}
	}

	return pub, nil
}

func (a *azureVaultClient) getKey(ctx context.Context) (keyvault.KeyBundle, error) {
	key, err := a.client.GetKey(ctx, a.vaultURL, a.keyName, "")
	if err != nil {
		return keyvault.KeyBundle{}, errors.Wrap(err, "public key")
	}

	return key, err
}

func (a *azureVaultClient) public() (crypto.PublicKey, error) {
	return a.keyCache.Get(CacheKey)
}

func (a *azureVaultClient) createKey(ctx context.Context) (crypto.PublicKey, error) {
	_, err := a.getKey(ctx)
	if err == nil {
		return a.public()
	}

	_, err = a.client.CreateKey(
		ctx,
		a.vaultURL,
		a.keyName,
		keyvault.KeyCreateParameters{
			KeyAttributes: &keyvault.KeyAttributes{
				Enabled: to.BoolPtr(true),
			},
			KeySize: to.Int32Ptr(2048),
			KeyOps: &[]keyvault.JSONWebKeyOperation{
				keyvault.Sign,
				keyvault.Verify,
			},
			Kty: keyvault.EC,
			Tags: map[string]*string{
				"use": to.StringPtr("sigstore"),
			},
		})
	if err != nil {
		return nil, err
	}

	return a.public()
}

func (a *azureVaultClient) sign(ctx context.Context, rawPayload []byte) ([]byte, error) {
	hash := sha256.Sum256(rawPayload)
	signed := hash[:]

	params := keyvault.KeySignParameters{
		Algorithm: keyvault.ES256,
		Value:     to.StringPtr(base64.RawURLEncoding.EncodeToString(signed)),
	}

	result, err := a.client.Sign(ctx, a.vaultURL, a.keyName, "", params)
	if err != nil {
		return nil, errors.Wrap(err, "signing the payload")
	}

	decResult, err := base64.RawURLEncoding.DecodeString(*result.Result)
	if err != nil {
		return nil, errors.Wrap(err, "decoding the result")
	}

	return decResult, nil
}

func (a *azureVaultClient) verify(ctx context.Context, signature, payload []byte) error {
	hash := sha256.Sum256(payload)
	signed := hash[:]

	params := keyvault.KeyVerifyParameters{
		Algorithm: keyvault.ES256,
		Digest:    to.StringPtr(base64.RawURLEncoding.EncodeToString(signed)),
		Signature: to.StringPtr(base64.RawURLEncoding.EncodeToString(signature)),
	}

	result, err := a.client.Verify(ctx, a.vaultURL, a.keyName, "", params)
	if err != nil {
		return errors.Wrap(err, "verify")
	}

	if !*result.Value {
		return errors.New("Failed vault verification")
	}

	return nil
}
