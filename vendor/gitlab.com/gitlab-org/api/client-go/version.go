//
// Copyright 2021, Andrea Funto'
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
//

package gitlab

import "net/http"

type (
	VersionServiceInterface interface {
		GetVersion(options ...RequestOptionFunc) (*Version, *Response, error)
	}

	// VersionService handles communication with the GitLab server instance to
	// retrieve its version information via the GitLab API.
	//
	// GitLab API docs: https://docs.gitlab.com/ee/api/version.html
	VersionService struct {
		client *Client
	}
)

var _ VersionServiceInterface = (*VersionService)(nil)

// Version represents a GitLab instance version.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/version.html
type Version struct {
	Version  string `json:"version"`
	Revision string `json:"revision"`
}

func (s Version) String() string {
	return Stringify(s)
}

// GetVersion gets a GitLab server instance version; it is only available to
// authenticated users.
//
// GitLab API docs: https://docs.gitlab.com/ee/api/version.html
func (s *VersionService) GetVersion(options ...RequestOptionFunc) (*Version, *Response, error) {
	req, err := s.client.NewRequest(http.MethodGet, "version", nil, options)
	if err != nil {
		return nil, nil, err
	}

	v := new(Version)
	resp, err := s.client.Do(req, v)
	if err != nil {
		return nil, resp, err
	}

	return v, resp, nil
}
