package github

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestDefaultBranchUsesRepositoryMetadata(t *testing.T) {
	extension := New()
	extension.httpClient = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodGet {
				t.Fatalf("method = %q, want GET", request.Method)
			}
			if request.URL.Path != "/repos/owner/repo" {
				t.Fatalf("path = %q, want /repos/owner/repo", request.URL.Path)
			}
			if got := request.Header.Get("Authorization"); got != "Bearer token" {
				t.Fatalf("authorization = %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"default_branch":"master"}`)),
			}, nil
		}),
	}

	branch, err := extension.defaultBranch(context.Background(), "token", "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if branch != "master" {
		t.Fatalf("branch = %q, want master", branch)
	}
}

func TestDefaultBranchRejectsEmptyRepositoryMetadata(t *testing.T) {
	extension := New()
	extension.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"default_branch":""}`)),
			}, nil
		}),
	}

	if _, err := extension.defaultBranch(context.Background(), "token", "owner", "repo"); err == nil {
		t.Fatal("expected empty default branch to fail")
	}
}
