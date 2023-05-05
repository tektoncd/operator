/*
Copyright 2023 The Tekton Authors

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

package tektonconfig

import (
	"testing"

	securityv1 "github.com/openshift/api/security/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_sccAEqualORPriorityOverB(t *testing.T) {
	type args struct {
		prioritizedSCCList []*securityv1.SecurityContextConstraints
		sccA               string
		sccB               string
	}
	tests := []struct {
		name     string
		args     args
		wantPass bool
		wantErr  bool
	}{
		{
			name: "sccA not found",
			args: args{
				prioritizedSCCList: []*securityv1.SecurityContextConstraints{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "sccB",
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "sccC",
						},
					},
				},
				sccA: "sccA",
				sccB: "sccB",
			},
			wantPass: false,
			wantErr:  true,
		},
		{
			name: "sccB not found",
			args: args{
				prioritizedSCCList: []*securityv1.SecurityContextConstraints{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "sccA",
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "sccC",
						},
					},
				},
				sccA: "sccA",
				sccB: "sccB",
			},
			wantPass: false,
			wantErr:  true,
		},
		{
			name: "sccA has lower priority than sccB",
			args: args{
				prioritizedSCCList: []*securityv1.SecurityContextConstraints{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "sccB",
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "sccA",
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "sccC",
						},
					},
				},
				sccA: "sccA",
				sccB: "sccB",
			},
			wantPass: false,
			wantErr:  false,
		},
		{
			// passing test
			name: "sccA has higher priority than sccB",
			args: args{
				prioritizedSCCList: []*securityv1.SecurityContextConstraints{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "sccA",
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "sccB",
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "sccC",
						},
					},
				},
				sccA: "sccA",
				sccB: "sccB",
			},
			wantPass: true,
			wantErr:  false,
		},
		{
			// passing test
			name: "sccA == sccB",
			args: args{
				prioritizedSCCList: []*securityv1.SecurityContextConstraints{
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "sccA",
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "sccB",
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name: "sccC",
						},
					},
				},
				sccA: "sccA",
				sccB: "sccA",
			},
			wantPass: true,
			wantErr:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := sccAEqualORPriorityOverB(test.args.prioritizedSCCList, test.args.sccA, test.args.sccB)
			if (err != nil) != test.wantErr {
				t.Errorf("sccAEqualORPriorityOverB() error = %v, expected %v", err, test.wantErr)
				return
			}
			if got != test.wantPass {
				t.Errorf("sccAEqualORPriorityOverB() got = %v, exptected %v", got, test.wantPass)
			}
		})
	}
}
