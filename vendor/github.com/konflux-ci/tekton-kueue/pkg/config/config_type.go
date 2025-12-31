// +k8s:deepcopy-gen=package

package config

/*
Copyright 2025.

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

type Config struct {
	MultiKueueConfig  `json:",inline"`
	TektonKueueConfig `json:",inline"`
}

type MultiKueueConfig struct {
	MultiKueueOverride bool `json:"multiKueueOverride,omitempty"`
}
type TektonKueueConfig struct {
	QueueName string `json:"queueName,omitempty"`
	CEL       CEL    `json:"cel,omitempty"`
}

type CEL struct {
	Expressions []string `json:"expressions,omitempty"`
}
