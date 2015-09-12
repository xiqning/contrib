/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	github_test "k8s.io/contrib/github/testing"

	"github.com/google/go-github/github"
)

func stringPtr(val string) *string     { return &val }
func timePtr(val time.Time) *time.Time { return &val }
func intPtr(val int) *int              { return &val }

func TestHasLabel(t *testing.T) {
	tests := []struct {
		labels   []github.Label
		label    string
		hasLabel bool
	}{
		{
			labels: []github.Label{
				{Name: stringPtr("foo")},
			},
			label:    "foo",
			hasLabel: true,
		},
		{
			labels: []github.Label{
				{Name: stringPtr("bar")},
			},
			label:    "foo",
			hasLabel: false,
		},
		{
			labels: []github.Label{
				{Name: stringPtr("bar")},
				{Name: stringPtr("foo")},
			},
			label:    "foo",
			hasLabel: true,
		},
		{
			labels: []github.Label{
				{Name: stringPtr("bar")},
				{Name: stringPtr("baz")},
			},
			label:    "foo",
			hasLabel: false,
		},
	}

	for _, test := range tests {
		if test.hasLabel != HasLabel(test.labels, test.label) {
			t.Errorf("Unexpected output: %v", test)
		}
	}
}

func TestHasLabels(t *testing.T) {
	tests := []struct {
		labels     []github.Label
		seekLabels []string
		hasLabel   bool
	}{
		{
			labels: []github.Label{
				{Name: stringPtr("foo")},
			},
			seekLabels: []string{"foo"},
			hasLabel:   true,
		},
		{
			labels: []github.Label{
				{Name: stringPtr("bar")},
			},
			seekLabels: []string{"foo"},
			hasLabel:   false,
		},
		{
			labels: []github.Label{
				{Name: stringPtr("bar")},
				{Name: stringPtr("foo")},
			},
			seekLabels: []string{"foo"},
			hasLabel:   true,
		},
		{
			labels: []github.Label{
				{Name: stringPtr("bar")},
				{Name: stringPtr("baz")},
			},
			seekLabels: []string{"foo"},
			hasLabel:   false,
		},
		{
			labels: []github.Label{
				{Name: stringPtr("foo")},
			},
			seekLabels: []string{"foo", "bar"},
			hasLabel:   false,
		},
	}

	for _, test := range tests {
		if test.hasLabel != HasLabels(test.labels, test.seekLabels) {
			t.Errorf("Unexpected output: %v", test)
		}
	}
}

func TestFetchAllPRs(t *testing.T) {
	tests := []struct {
		PullRequests [][]github.PullRequest
		Pages        []int
	}{
		{
			PullRequests: [][]github.PullRequest{
				{
					{},
				},
			},
			Pages: []int{0},
		},
		{
			PullRequests: [][]github.PullRequest{
				{
					{},
				},
				{
					{},
				},
				{
					{},
				},
				{
					{},
				},
			},
			Pages: []int{4, 4, 4, 0},
		},
		{
			PullRequests: [][]github.PullRequest{
				{
					{},
				},
				{
					{},
				},
				{
					{},
					{},
					{},
				},
			},
			Pages: []int{3, 3, 0},
		},
	}

	for _, test := range tests {
		client, server, mux := github_test.InitTest()
		config := &GithubConfig{
			client:  client,
			Org:     "foo",
			Project: "bar",
		}
		count := 0
		prCount := 0
		mux.HandleFunc("/repos/foo/bar/pulls", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				t.Errorf("Unexpected method: %s", r.Method)
			}
			page := r.URL.Query().Get("page")
			if page == "" {
				page = "0"
			}
			if page != strconv.Itoa(count) {
				t.Errorf("Unexpected page: %s", r.URL.Query().Get("page"))
			}
			if r.URL.Query().Get("sort") != "desc" {
				t.Errorf("Unexpected sort: %s", r.URL.Query().Get("sort"))
			}
			if r.URL.Query().Get("per_page") != "100" {
				t.Errorf("Unexpected per_page: %s", r.URL.Query().Get("per_page"))
			}
			w.Header().Add("Link",
				fmt.Sprintf("<https://api.github.com/?page=%d>; rel=\"last\"", test.Pages[count]))
			w.WriteHeader(http.StatusOK)
			data, err := json.Marshal(test.PullRequests[count])
			prCount += len(test.PullRequests[count])
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			w.Write(data)
			count++
		})
		prs, err := config.fetchAllPRs()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(prs) != prCount {
			t.Errorf("unexpected output %d vs %d", len(prs), prCount)
		}

		if count != len(test.PullRequests) {
			t.Errorf("unexpected number of fetches: %d", count)
		}
		server.Close()
	}
}

func TestComputeStatus(t *testing.T) {
	tests := []struct {
		statusList       []*github.CombinedStatus
		requiredContexts []string
		expected         string
	}{
		{
			statusList: []*github.CombinedStatus{
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
			},
			expected: "success",
		},
		{
			statusList: []*github.CombinedStatus{
				{State: stringPtr("error"), SHA: stringPtr("abcdef")},
				{State: stringPtr("pending"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
			},
			expected: "pending",
		},
		{
			statusList: []*github.CombinedStatus{
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
				{State: stringPtr("pending"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
			},
			expected: "pending",
		},
		{
			statusList: []*github.CombinedStatus{
				{State: stringPtr("failure"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
			},
			expected: "failure",
		},
		{
			statusList: []*github.CombinedStatus{
				{State: stringPtr("failure"), SHA: stringPtr("abcdef")},
				{State: stringPtr("error"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
			},
			expected: "error",
		},
		{
			statusList: []*github.CombinedStatus{
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
			},
			requiredContexts: []string{"context"},
			expected:         "incomplete",
		},
		{
			statusList: []*github.CombinedStatus{
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
				{State: stringPtr("pending"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
			},
			requiredContexts: []string{"context"},
			expected:         "incomplete",
		},
		{
			statusList: []*github.CombinedStatus{
				{State: stringPtr("failure"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
			},
			requiredContexts: []string{"context"},
			expected:         "incomplete",
		},
		{
			statusList: []*github.CombinedStatus{
				{State: stringPtr("failure"), SHA: stringPtr("abcdef")},
				{State: stringPtr("error"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
			},
			requiredContexts: []string{"context"},
			expected:         "incomplete",
		},
		{
			statusList: []*github.CombinedStatus{
				{
					State: stringPtr("success"),
					SHA:   stringPtr("abcdef"),
					Statuses: []github.RepoStatus{
						{Context: stringPtr("context")},
					},
				},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
			},
			requiredContexts: []string{"context"},
			expected:         "success",
		},
		{
			statusList: []*github.CombinedStatus{
				{
					State: stringPtr("pending"),
					SHA:   stringPtr("abcdef"),
					Statuses: []github.RepoStatus{
						{Context: stringPtr("context")},
					},
				},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
			},
			requiredContexts: []string{"context"},
			expected:         "pending",
		},
		{
			statusList: []*github.CombinedStatus{
				{
					State: stringPtr("error"),
					SHA:   stringPtr("abcdef"),
					Statuses: []github.RepoStatus{
						{Context: stringPtr("context")},
					},
				},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
			},
			requiredContexts: []string{"context"},
			expected:         "error",
		},
		{
			statusList: []*github.CombinedStatus{
				{
					State: stringPtr("failure"),
					SHA:   stringPtr("abcdef"),
					Statuses: []github.RepoStatus{
						{Context: stringPtr("context")},
					},
				},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
				{State: stringPtr("success"), SHA: stringPtr("abcdef")},
			},
			requiredContexts: []string{"context"},
			expected:         "failure",
		},
	}

	for _, test := range tests {
		// ease of use, reduce boilerplate in test cases
		if test.requiredContexts == nil {
			test.requiredContexts = []string{}
		}
		status := computeStatus(test.statusList, test.requiredContexts)
		if test.expected != status {
			t.Errorf("expected: %s, saw %s", test.expected, status)
		}
	}
}

func TestGetLastModified(t *testing.T) {
	tests := []struct {
		commits      []github.RepositoryCommit
		expectedTime *time.Time
	}{
		{
			commits: []github.RepositoryCommit{
				{
					Commit: &github.Commit{
						Committer: &github.CommitAuthor{
							Date: timePtr(time.Unix(10, 0)),
						},
					},
				},
			},
			expectedTime: timePtr(time.Unix(10, 0)),
		},
		{
			commits: []github.RepositoryCommit{
				{
					Commit: &github.Commit{
						Committer: &github.CommitAuthor{
							Date: timePtr(time.Unix(10, 0)),
						},
					},
				},
				{
					Commit: &github.Commit{
						Committer: &github.CommitAuthor{
							Date: timePtr(time.Unix(11, 0)),
						},
					},
				},
				{
					Commit: &github.Commit{
						Committer: &github.CommitAuthor{
							Date: timePtr(time.Unix(12, 0)),
						},
					},
				},
			},
			expectedTime: timePtr(time.Unix(12, 0)),
		},
		{
			commits: []github.RepositoryCommit{
				{
					Commit: &github.Commit{
						Committer: &github.CommitAuthor{
							Date: timePtr(time.Unix(10, 0)),
						},
					},
				},
				{
					Commit: &github.Commit{
						Committer: &github.CommitAuthor{
							Date: timePtr(time.Unix(9, 0)),
						},
					},
				},
				{
					Commit: &github.Commit{
						Committer: &github.CommitAuthor{
							Date: timePtr(time.Unix(8, 0)),
						},
					},
				},
			},
			expectedTime: timePtr(time.Unix(10, 0)),
		},
		{
			commits: []github.RepositoryCommit{
				{
					Commit: &github.Commit{
						Committer: &github.CommitAuthor{
							Date: timePtr(time.Unix(9, 0)),
						},
					},
				},
				{
					Commit: &github.Commit{
						Committer: &github.CommitAuthor{
							Date: timePtr(time.Unix(10, 0)),
						},
					},
				},
				{
					Commit: &github.Commit{
						Committer: &github.CommitAuthor{
							Date: timePtr(time.Unix(9, 0)),
						},
					},
				},
			},
			expectedTime: timePtr(time.Unix(10, 0)),
		},
	}
	for _, test := range tests {
		client, server, mux := github_test.InitTest()
		config := &GithubConfig{
			client:  client,
			Org:     "o",
			Project: "r",
		}
		mux.HandleFunc(fmt.Sprintf("/repos/o/r/pulls/1/commits"), func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				t.Errorf("Unexpected method: %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
			data, err := json.Marshal(test.commits)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			w.Write(data)
			ts, err := config.LastModifiedTime(1)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !ts.Equal(*test.expectedTime) {
				t.Errorf("expected: %v, saw: %v", test.expectedTime, ts)
			}
		})
		server.Close()
	}
}
