//
// Copyright 2021, Patrick Webster
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

import (
	"fmt"
	"net/http"
)

type (
	GroupIssueBoardsServiceInterface interface {
		ListGroupIssueBoards(gid interface{}, opt *ListGroupIssueBoardsOptions, options ...RequestOptionFunc) ([]*GroupIssueBoard, *Response, error)
		CreateGroupIssueBoard(gid interface{}, opt *CreateGroupIssueBoardOptions, options ...RequestOptionFunc) (*GroupIssueBoard, *Response, error)
		GetGroupIssueBoard(gid interface{}, board int, options ...RequestOptionFunc) (*GroupIssueBoard, *Response, error)
		UpdateIssueBoard(gid interface{}, board int, opt *UpdateGroupIssueBoardOptions, options ...RequestOptionFunc) (*GroupIssueBoard, *Response, error)
		DeleteIssueBoard(gid interface{}, board int, options ...RequestOptionFunc) (*Response, error)
		ListGroupIssueBoardLists(gid interface{}, board int, opt *ListGroupIssueBoardListsOptions, options ...RequestOptionFunc) ([]*BoardList, *Response, error)
		GetGroupIssueBoardList(gid interface{}, board, list int, options ...RequestOptionFunc) (*BoardList, *Response, error)
		CreateGroupIssueBoardList(gid interface{}, board int, opt *CreateGroupIssueBoardListOptions, options ...RequestOptionFunc) (*BoardList, *Response, error)
		UpdateIssueBoardList(gid interface{}, board, list int, opt *UpdateGroupIssueBoardListOptions, options ...RequestOptionFunc) ([]*BoardList, *Response, error)
		DeleteGroupIssueBoardList(gid interface{}, board, list int, options ...RequestOptionFunc) (*Response, error)
	}

	// GroupIssueBoardsService handles communication with the group issue board
	// related methods of the GitLab API.
	//
	// GitLab API docs:
	// https://docs.gitlab.com/api/group_boards/
	GroupIssueBoardsService struct {
		client *Client
	}
)

var _ GroupIssueBoardsServiceInterface = (*GroupIssueBoardsService)(nil)

// GroupIssueBoard represents a GitLab group issue board.
//
// GitLab API docs:
// https://docs.gitlab.com/api/group_boards/
type GroupIssueBoard struct {
	ID        int           `json:"id"`
	Name      string        `json:"name"`
	Group     *Group        `json:"group"`
	Milestone *Milestone    `json:"milestone"`
	Labels    []*GroupLabel `json:"labels"`
	Lists     []*BoardList  `json:"lists"`
}

func (b GroupIssueBoard) String() string {
	return Stringify(b)
}

// ListGroupIssueBoardsOptions represents the available
// ListGroupIssueBoards() options.
//
// GitLab API docs:
// https://docs.gitlab.com/api/group_boards/#list-all-group-issue-boards-in-a-group
type ListGroupIssueBoardsOptions ListOptions

// ListGroupIssueBoards gets a list of all issue boards in a group.
//
// GitLab API docs:
// https://docs.gitlab.com/api/group_boards/#list-all-group-issue-boards-in-a-group
func (s *GroupIssueBoardsService) ListGroupIssueBoards(gid interface{}, opt *ListGroupIssueBoardsOptions, options ...RequestOptionFunc) ([]*GroupIssueBoard, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/boards", PathEscape(group))

	req, err := s.client.NewRequest(http.MethodGet, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	var gs []*GroupIssueBoard
	resp, err := s.client.Do(req, &gs)
	if err != nil {
		return nil, resp, err
	}

	return gs, resp, nil
}

// CreateGroupIssueBoardOptions represents the available
// CreateGroupIssueBoard() options.
//
// GitLab API docs:
// https://docs.gitlab.com/api/group_boards/#create-a-group-issue-board
type CreateGroupIssueBoardOptions struct {
	Name *string `url:"name" json:"name"`
}

// CreateGroupIssueBoard creates a new issue board.
//
// GitLab API docs:
// https://docs.gitlab.com/api/group_boards/#create-a-group-issue-board
func (s *GroupIssueBoardsService) CreateGroupIssueBoard(gid interface{}, opt *CreateGroupIssueBoardOptions, options ...RequestOptionFunc) (*GroupIssueBoard, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/boards", PathEscape(group))

	req, err := s.client.NewRequest(http.MethodPost, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	gib := new(GroupIssueBoard)
	resp, err := s.client.Do(req, gib)
	if err != nil {
		return nil, resp, err
	}

	return gib, resp, nil
}

// GetGroupIssueBoard gets a single issue board of a group.
//
// GitLab API docs:
// https://docs.gitlab.com/api/group_boards/#single-group-issue-board
func (s *GroupIssueBoardsService) GetGroupIssueBoard(gid interface{}, board int, options ...RequestOptionFunc) (*GroupIssueBoard, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/boards/%d", PathEscape(group), board)

	req, err := s.client.NewRequest(http.MethodGet, u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	gib := new(GroupIssueBoard)
	resp, err := s.client.Do(req, gib)
	if err != nil {
		return nil, resp, err
	}

	return gib, resp, nil
}

// UpdateGroupIssueBoardOptions represents a group issue board.
//
// GitLab API docs:
// https://docs.gitlab.com/api/group_boards/#update-a-group-issue-board
type UpdateGroupIssueBoardOptions struct {
	Name        *string       `url:"name,omitempty" json:"name,omitempty"`
	AssigneeID  *int          `url:"assignee_id,omitempty" json:"assignee_id,omitempty"`
	MilestoneID *int          `url:"milestone_id,omitempty" json:"milestone_id,omitempty"`
	Labels      *LabelOptions `url:"labels,omitempty" json:"labels,omitempty"`
	Weight      *int          `url:"weight,omitempty" json:"weight,omitempty"`
}

// UpdateIssueBoard updates a single issue board of a group.
//
// GitLab API docs:
// https://docs.gitlab.com/api/group_boards/#update-a-group-issue-board
func (s *GroupIssueBoardsService) UpdateIssueBoard(gid interface{}, board int, opt *UpdateGroupIssueBoardOptions, options ...RequestOptionFunc) (*GroupIssueBoard, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/boards/%d", PathEscape(group), board)

	req, err := s.client.NewRequest(http.MethodPut, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	gib := new(GroupIssueBoard)
	resp, err := s.client.Do(req, gib)
	if err != nil {
		return nil, resp, err
	}

	return gib, resp, nil
}

// DeleteIssueBoard delete a single issue board of a group.
//
// GitLab API docs:
// https://docs.gitlab.com/api/group_boards/#delete-a-group-issue-board
func (s *GroupIssueBoardsService) DeleteIssueBoard(gid interface{}, board int, options ...RequestOptionFunc) (*Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("groups/%s/boards/%d", PathEscape(group), board)

	req, err := s.client.NewRequest(http.MethodDelete, u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// ListGroupIssueBoardListsOptions represents the available
// ListGroupIssueBoardLists() options.
//
// GitLab API docs:
// https://docs.gitlab.com/api/group_boards/#list-group-issue-board-lists
type ListGroupIssueBoardListsOptions ListOptions

// ListGroupIssueBoardLists gets a list of the issue board's lists. Does not include
// backlog and closed lists.
//
// GitLab API docs: https://docs.gitlab.com/api/group_boards/#list-group-issue-board-lists
func (s *GroupIssueBoardsService) ListGroupIssueBoardLists(gid interface{}, board int, opt *ListGroupIssueBoardListsOptions, options ...RequestOptionFunc) ([]*BoardList, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/boards/%d/lists", PathEscape(group), board)

	req, err := s.client.NewRequest(http.MethodGet, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	var gbl []*BoardList
	resp, err := s.client.Do(req, &gbl)
	if err != nil {
		return nil, resp, err
	}

	return gbl, resp, nil
}

// GetGroupIssueBoardList gets a single issue board list.
//
// GitLab API docs:
// https://docs.gitlab.com/api/group_boards/#single-group-issue-board-list
func (s *GroupIssueBoardsService) GetGroupIssueBoardList(gid interface{}, board, list int, options ...RequestOptionFunc) (*BoardList, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/boards/%d/lists/%d",
		PathEscape(group),
		board,
		list,
	)

	req, err := s.client.NewRequest(http.MethodGet, u, nil, options)
	if err != nil {
		return nil, nil, err
	}

	gbl := new(BoardList)
	resp, err := s.client.Do(req, gbl)
	if err != nil {
		return nil, resp, err
	}

	return gbl, resp, nil
}

// CreateGroupIssueBoardListOptions represents the available
// CreateGroupIssueBoardList() options.
//
// GitLab API docs:
// https://docs.gitlab.com/api/group_boards/#new-group-issue-board-list
type CreateGroupIssueBoardListOptions struct {
	LabelID *int `url:"label_id" json:"label_id"`
}

// CreateGroupIssueBoardList creates a new issue board list.
//
// GitLab API docs:
// https://docs.gitlab.com/api/group_boards/#new-group-issue-board-list
func (s *GroupIssueBoardsService) CreateGroupIssueBoardList(gid interface{}, board int, opt *CreateGroupIssueBoardListOptions, options ...RequestOptionFunc) (*BoardList, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/boards/%d/lists", PathEscape(group), board)

	req, err := s.client.NewRequest(http.MethodPost, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	gbl := new(BoardList)
	resp, err := s.client.Do(req, gbl)
	if err != nil {
		return nil, resp, err
	}

	return gbl, resp, nil
}

// UpdateGroupIssueBoardListOptions represents the available
// UpdateGroupIssueBoardList() options.
//
// GitLab API docs:
// https://docs.gitlab.com/api/group_boards/#edit-group-issue-board-list
type UpdateGroupIssueBoardListOptions struct {
	Position *int `url:"position" json:"position"`
}

// UpdateIssueBoardList updates the position of an existing
// group issue board list.
//
// GitLab API docs:
// https://docs.gitlab.com/api/group_boards/#edit-group-issue-board-list
func (s *GroupIssueBoardsService) UpdateIssueBoardList(gid interface{}, board, list int, opt *UpdateGroupIssueBoardListOptions, options ...RequestOptionFunc) ([]*BoardList, *Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, nil, err
	}
	u := fmt.Sprintf("groups/%s/boards/%d/lists/%d",
		PathEscape(group),
		board,
		list,
	)

	req, err := s.client.NewRequest(http.MethodPut, u, opt, options)
	if err != nil {
		return nil, nil, err
	}

	var gbl []*BoardList
	resp, err := s.client.Do(req, &gbl)
	if err != nil {
		return nil, resp, err
	}

	return gbl, resp, nil
}

// DeleteGroupIssueBoardList soft deletes a group issue board list.
// Only for admins and group owners.
//
// GitLab API docs:
// https://docs.gitlab.com/api/group_boards/#delete-a-group-issue-board-list
func (s *GroupIssueBoardsService) DeleteGroupIssueBoardList(gid interface{}, board, list int, options ...RequestOptionFunc) (*Response, error) {
	group, err := parseID(gid)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("groups/%s/boards/%d/lists/%d",
		PathEscape(group),
		board,
		list,
	)

	req, err := s.client.NewRequest(http.MethodDelete, u, nil, options)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
