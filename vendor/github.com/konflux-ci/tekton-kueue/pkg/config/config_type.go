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
	CEL             CEL             `yaml:"cel,omitempty"`
	SchedulerConfig SchedulerConfig `yaml:",inline"`
}

type CEL struct {
	Expressions []string `yaml:"expressions,omitempty"`
}

type SchedulerConfig struct {
	QueueName           string `json:"queueName,omitempty" yaml:"queueName,omitempty"`
	MultiClusterEnabled bool   `json:"multi-cluster-enabled,omitempty" yaml:"multi-cluster-enabled,omitempty"`
	MultiClusterRole    string `json:"multi-cluster-role,omitempty" yaml:"multi-cluster-role,omitempty"`
}
