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

package common

import (
	"context"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"gotest.tools/assert"
	"testing"
)

func nullFunc() func(params *[]v1alpha1.Param) error {
	return func(params *[]v1alpha1.Param) error {
		return nil
	}
}

func TestValidateParamsAndSetDefault(t *testing.T) {

	componentParams := map[string]v1alpha1.ParamValue{
		"valid": v1alpha1.ParamValue{
			Default:  "param",
			Possible: []string{"param", "param1"},
		},
		"newParam": v1alpha1.ParamValue{
			Default:  "newValue",
			Possible: []string{"newValue", "newValue1"},
		},
	}

	t.Run("Invalid Param", func(t *testing.T) {
		params := []v1alpha1.Param{
			{Name: "foo", Value: "bar"},
		}
		_, err := ValidateParamsAndSetDefault(context.TODO(), &params, componentParams, nullFunc())
		assert.Error(t, err, "invalid param : foo")
	})

	t.Run("Invalid Param Value", func(t *testing.T) {
		params := []v1alpha1.Param{
			{Name: "valid", Value: "bar"},
		}
		_, err := ValidateParamsAndSetDefault(context.TODO(), &params, componentParams, nullFunc())
		assert.Error(t, err, "invalid value (bar) for param: valid")
	})

	t.Run("All params are defined so no error", func(t *testing.T) {
		params := []v1alpha1.Param{
			{Name: "valid", Value: "param1"},
			{Name: "newParam", Value: "newValue"},
		}
		updated, err := ValidateParamsAndSetDefault(context.TODO(), &params, componentParams, nullFunc())
		assert.NilError(t, err)
		assert.Equal(t, updated, false)
	})

	t.Run("Some params are missing", func(t *testing.T) {
		params := []v1alpha1.Param{
			{Name: "valid", Value: "param1"},
		}
		updated, err := ValidateParamsAndSetDefault(context.TODO(), &params, componentParams, nullFunc())
		assert.NilError(t, err)
		// since some params are added to spec, updated should be true
		assert.Equal(t, updated, true)
		// missing params will be added with default value
		assert.Equal(t, params[1].Name, "newParam")
		assert.Equal(t, params[1].Value, "newValue")
	})
}
