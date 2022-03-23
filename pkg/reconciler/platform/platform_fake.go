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

package platform

type FakePlatform struct {
	pParams           func() PlatformConfig
	allSupportedCtrls func() ControllerMap
}

func (fp *FakePlatform) PlatformParams() PlatformConfig {
	return fp.pParams()
}

func (fp *FakePlatform) AllSupportedControllers() ControllerMap {
	return fp.allSupportedCtrls()
}

func SeededFakePlatform(cn []ControllerName, ctrls ControllerMap) *FakePlatform {
	f := FakePlatform{}
	f.pParams = func() PlatformConfig {
		return PlatformConfig{
			ControllerNames: cn,
		}
	}
	f.allSupportedCtrls = func() ControllerMap {
		return ctrls
	}
	return &f
}
