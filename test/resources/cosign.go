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

package resources

import (
	"context"
	"fmt"
	"os"

	"github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/verify"
	"github.com/sigstore/cosign/v2/pkg/cosign/kubernetes"
)

func CosignGenerateKeyPair(namespace, secretName string) error {
	// we don't want to provide any password
	passFunc := func(bool) ([]byte, error) {
		return nil, nil
	}

	return kubernetes.KeyPairSecret(context.TODO(), fmt.Sprintf("%v/%v", namespace, secretName), passFunc)
}

// cosign verify-blob-attestation --insecure-ignore-tlog --key k8s://tekton-chains/signing-secrets --signature sig --type slsaprovenance --check-claims=false /dev/null
func CosignVerifyBlobAttestation(key, signature, payload string) error {
	signatureFile, err := os.CreateTemp("", "signature")
	if err != nil {
		return err
	}
	defer os.Remove(signatureFile.Name())

	if _, err := signatureFile.WriteString(signature); err != nil {
		return err
	}

	payloadFile, err := os.CreateTemp("", "payload")
	if err != nil {
		return err
	}
	defer os.Remove(payloadFile.Name())

	if _, err := payloadFile.WriteString(signature); err != nil {
		return err
	}

	command := verify.VerifyBlobAttestationCommand{
		KeyOpts: options.KeyOpts{
			KeyRef: key,
		},
		CheckClaims:   false,
		IgnoreTlog:    true,
		PredicateType: "slsaprovenance",
		SignaturePath: signatureFile.Name(),
	}

	return command.Exec(context.TODO(), payloadFile.Name())
}
