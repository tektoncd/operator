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

package dsse

import (
	"bytes"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/ioutil"

	"github.com/in-toto/in-toto-golang/pkg/ssl"
	"github.com/sigstore/sigstore/pkg/signature"
)

func WrapSigner(s signature.Signer, payloadType string) signature.Signer {
	return &wrappedSigner{
		s:           s,
		payloadType: payloadType,
	}
}

type wrappedSigner struct {
	s           signature.Signer
	payloadType string
}

func (w *wrappedSigner) PublicKey(opts ...signature.PublicKeyOption) (crypto.PublicKey, error) {
	return w.s.PublicKey(opts...)
}

func (w *wrappedSigner) SignMessage(r io.Reader, opts ...signature.SignOption) ([]byte, error) {
	p, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	pae := ssl.PAE(w.payloadType, string(p))
	sig, err := w.s.SignMessage(bytes.NewReader(pae), opts...)
	if err != nil {
		return nil, err
	}

	env := ssl.Envelope{
		PayloadType: w.payloadType,
		Payload:     base64.StdEncoding.EncodeToString(p),
		Signatures: []ssl.Signature{
			{
				Sig: base64.StdEncoding.EncodeToString(sig),
			},
		},
	}
	return json.Marshal(env)
}

func WrapVerifier(v signature.Verifier) signature.Verifier {
	return &wrappedVerifier{
		v: v,
	}
}

type wrappedVerifier struct {
	v signature.Verifier
}

func (w *wrappedVerifier) PublicKey(opts ...signature.PublicKeyOption) (crypto.PublicKey, error) {
	return w.v.PublicKey(opts...)
}

func (w *wrappedVerifier) VerifySignature(s io.Reader, _ io.Reader, opts ...signature.VerifyOption) error {
	sig, err := ioutil.ReadAll(s)
	if err != nil {
		return err
	}

	env := ssl.Envelope{}
	if err := json.Unmarshal(sig, &env); err != nil {
		return nil
	}

	verifier := ssl.NewEnvelopeVerifier(&innerWrapper{v: w.v})
	return verifier.Verify(&env)
}

type innerWrapper struct {
	v signature.Verifier
}

func (w *innerWrapper) Verify(_ string, data []byte, sig []byte) error {
	return w.v.VerifySignature(bytes.NewReader(sig), bytes.NewReader(data))
}

func WrapSignerVerifier(sv signature.SignerVerifier, payloadType string) signature.SignerVerifier {
	signer := &wrappedSigner{
		payloadType: payloadType,
		s:           sv,
	}
	verifier := &wrappedVerifier{
		v: sv,
	}

	return &wrappedSignerVerifier{
		signer:   signer,
		verifier: verifier,
	}
}

type wrappedSignerVerifier struct {
	signer   *wrappedSigner
	verifier *wrappedVerifier
}

func (w *wrappedSignerVerifier) PublicKey(opts ...signature.PublicKeyOption) (crypto.PublicKey, error) {
	return w.signer.PublicKey(opts...)
}

func (w *wrappedSignerVerifier) VerifySignature(s io.Reader, r io.Reader, opts ...signature.VerifyOption) error {
	return w.verifier.VerifySignature(s, r, opts...)
}

func (w *wrappedSignerVerifier) SignMessage(r io.Reader, opts ...signature.SignOption) ([]byte, error) {
	return w.signer.SignMessage(r, opts...)
}
