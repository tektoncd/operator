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
	"github.com/sigstore/cosign/cmd/cosign/cli"
	"github.com/sigstore/cosign/pkg/cosign/kubernetes"
	"os"
)

func CosignGenerateKeyPair(namespace, secretName string) error {
	// we don't want to provide any password
	passFunc := func(bool) ([]byte, error) {
		return nil, nil
	}

	return kubernetes.KeyPairSecret(context.TODO(), fmt.Sprintf("%v/%v", namespace, secretName), passFunc)
}

// cosign verify-blob --key k8s://tekton-chains/signing-secrets --signature ./signature ./payload
func CosignVerifyBlob(key, signature, payload string) error {
	signatureFile, err := os.CreateTemp("", "signature")
	if err != nil {
		return err
	}
	defer os.Remove(signatureFile.Name())

	if _, err := signatureFile.WriteString(signature); err != nil {
		return err
	}

	payloadFile, err := os.CreateTemp("", "signature")
	if err != nil {
		return err
	}
	defer os.Remove(payloadFile.Name())

	if _, err := payloadFile.WriteString(payload); err != nil {
		return err
	}

	return cli.VerifyBlobCmd(context.TODO(), cli.KeyOpts{KeyRef: key}, "", signatureFile.Name(), payloadFile.Name())
}
