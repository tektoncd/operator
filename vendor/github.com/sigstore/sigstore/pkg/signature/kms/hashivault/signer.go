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
	"bytes"
	"context"
	"crypto"
	"io"

	"github.com/pkg/errors"
	"github.com/sigstore/sigstore/pkg/signature"
	"github.com/sigstore/sigstore/pkg/signature/options"
)

// Taken from https://www.vaultproject.io/api/secret/transit
//nolint:golint
const (
	Algorithm_ECDSA_P256 = "ecdsa-p256"
	Algorithm_ECDSA_P384 = "ecdsa-p384"
	Algorithm_ECDSA_P521 = "ecdsa-p521"
	Algorithm_ED25519    = "ed25519"
	Algorithm_RSA_2048   = "rsa-2048"
	Algorithm_RSA_3072   = "rsa-3072"
	Algorithm_RSA_4096   = "rsa-4096"
)

var hvSupportedAlgorithms []string = []string{
	Algorithm_ECDSA_P256,
	Algorithm_ECDSA_P384,
	Algorithm_ECDSA_P521,
	Algorithm_ED25519,
	Algorithm_RSA_2048,
	Algorithm_RSA_3072,
	Algorithm_RSA_4096,
}

var hvSupportedHashFuncs = []crypto.Hash{
	crypto.SHA224,
	crypto.SHA256,
	crypto.SHA384,
	crypto.SHA512,
	crypto.Hash(0),
}

type SignerVerifier struct {
	hashFunc crypto.Hash
	client   *hashivaultClient
}

// LoadSignerVerifier generates signatures using the specified key object in Vault and hash algorithm.
//
// It also can verify signatures (via a remote vall to the Vault instance). hashFunc should be
// set to crypto.Hash(0) if the key referred to by referenceStr is an ED25519 signing key.
func LoadSignerVerifier(referenceStr string, hashFunc crypto.Hash) (*SignerVerifier, error) {
	h := &SignerVerifier{}

	var err error
	h.client, err = newHashivaultClient(referenceStr)
	if err != nil {
		return nil, err
	}

	switch hashFunc {
	case 0, crypto.SHA224, crypto.SHA256, crypto.SHA384, crypto.SHA512:
		h.hashFunc = hashFunc
	default:
		return nil, errors.New("hash function not supported by Hashivault")
	}

	return h, nil
}

// THIS WILL BE REMOVED ONCE ALL SIGSTORE PROJECTS NO LONGER USE IT
func (h *SignerVerifier) Sign(ctx context.Context, payload []byte) ([]byte, []byte, error) {
	sig, err := h.SignMessage(bytes.NewReader(payload), options.WithContext(ctx))
	return sig, nil, err
}

// SignMessage signs the provided message using Hashivault KMS. If the message is provided,
// this method will compute the digest according to the hash function specified
// when the HashivaultSigner was created.
//
// SignMessage recognizes the following Options listed in order of preference:
//
// - WithDigest()
//
// All other options are ignored if specified.
func (h SignerVerifier) SignMessage(message io.Reader, opts ...signature.SignOption) ([]byte, error) {
	var digest []byte
	var signerOpts crypto.SignerOpts = h.hashFunc

	for _, opt := range opts {
		opt.ApplyDigest(&digest)
		opt.ApplyCryptoSignerOpts(&signerOpts)
	}

	digest, hf, err := signature.ComputeDigestForSigning(message, signerOpts.HashFunc(), hvSupportedHashFuncs, opts...)
	if err != nil {
		return nil, err
	}

	return h.client.sign(digest, hf)

}

// PublicKey returns the public key that can be used to verify signatures created by
// this signer. All options provided in arguments to this method are ignored.
func (h SignerVerifier) PublicKey(_ ...signature.PublicKeyOption) (crypto.PublicKey, error) {
	return h.client.public()
}

// VerifySignature verifies the signature for the given message. Unless provided
// in an option, the digest of the message will be computed using the hash function specified
// when the SignerVerifier was created.
//
// This function returns nil if the verification succeeded, and an error message otherwise.
//
// This function recognizes the following Options listed in order of preference:
//
// - WithDigest()
//
// - WithCryptoSignerOpts()
//
// All other options are ignored if specified.
func (h SignerVerifier) VerifySignature(sig, message io.Reader, opts ...signature.VerifyOption) error {
	var digest []byte
	var signerOpts crypto.SignerOpts = h.hashFunc

	for _, opt := range opts {
		opt.ApplyDigest(&digest)
		opt.ApplyCryptoSignerOpts(&signerOpts)
	}

	digest, hf, err := signature.ComputeDigestForVerifying(message, signerOpts.HashFunc(), hvSupportedHashFuncs, opts...)
	if err != nil {
		return err
	}

	sigBytes, err := io.ReadAll(sig)
	if err != nil {
		return errors.Wrap(err, "reading signature")
	}

	return h.client.verify(sigBytes, digest, hf)
}

// CreateKey attempts to create a new key in Vault with the specified algorithm.
func (h SignerVerifier) CreateKey(_ context.Context, algorithm string) (crypto.PublicKey, error) {
	return h.client.createKey(algorithm)
}

type cryptoSignerWrapper struct {
	ctx      context.Context
	hashFunc crypto.Hash
	sv       *SignerVerifier
	errFunc  func(error)
}

func (c cryptoSignerWrapper) Public() crypto.PublicKey {
	pk, err := c.sv.PublicKey(options.WithContext(c.ctx))
	if err != nil && c.errFunc != nil {
		c.errFunc(err)
	}
	return pk
}

func (c cryptoSignerWrapper) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	hashFunc := c.hashFunc
	if opts != nil {
		hashFunc = opts.HashFunc()
	}
	hvOptions := []signature.SignOption{
		options.WithContext(c.ctx),
		options.WithDigest(digest),
		options.WithCryptoSignerOpts(hashFunc),
	}

	return c.sv.SignMessage(nil, hvOptions...)
}

func (h *SignerVerifier) CryptoSigner(ctx context.Context, errFunc func(error)) (crypto.Signer, crypto.SignerOpts, error) {
	csw := &cryptoSignerWrapper{
		ctx:      ctx,
		sv:       h,
		hashFunc: h.hashFunc,
		errFunc:  errFunc,
	}

	return csw, h.hashFunc, nil
}

func (h *SignerVerifier) SupportedAlgorithms() []string {
	return hvSupportedAlgorithms
}

func (h *SignerVerifier) DefaultAlgorithm() string {
	return Algorithm_ECDSA_P256
}
