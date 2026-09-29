package githubdoc

import (
	"context"
	"testing"
)

func TestFetcherBuildRequestAppliesHeaders(t *testing.T) {
	testCases := []struct {
		name                  string
		token                 string
		expectAuthorization   bool
		expectedAuthorization string
	}{
		{
			name:                  "personal access token",
			token:                 "abc123",
			expectAuthorization:   true,
			expectedAuthorization: authorizationTokenPrefix + "abc123",
		},
		{
			name:                  "explicit bearer prefix retained",
			token:                 "Bearer prefixed",
			expectAuthorization:   true,
			expectedAuthorization: "Bearer prefixed",
		},
		{
			name:                  "explicit token prefix retained",
			token:                 "token prefixed",
			expectAuthorization:   true,
			expectedAuthorization: "token prefixed",
		},
		{
			name:                  "jwt token defaults to bearer",
			token:                 "a.b.c",
			expectAuthorization:   true,
			expectedAuthorization: authorizationBearerPrefix + "a.b.c",
		},
		{
			name:                "without token",
			token:               "",
			expectAuthorization: false,
		},
	}
	for _, testCase := range testCases {

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			fetcher := NewFetcher(nil)
			if testCase.token != "" {
				fetcher = fetcher.WithAuthorizationToken(testCase.token)
			}
			request, err := fetcher.buildRequest(context.Background(), "https://example.com")
			if err != nil {
				t.Fatalf("buildRequest error: %v", err)
			}
			if request.Header.Get(headerAccept) != acceptGitHubJSON {
				t.Fatalf("expected accept header %s, got %s", acceptGitHubJSON, request.Header.Get(headerAccept))
			}
			if request.Header.Get(headerGitHubAPIVersion) != githubAPIVersionValue {
				t.Fatalf("expected API version header %s, got %s", githubAPIVersionValue, request.Header.Get(headerGitHubAPIVersion))
			}
			if request.Header.Get("User-Agent") != defaultUserAgent {
				t.Fatalf("expected user agent header to be set")
			}
			authorizationHeader := request.Header.Get(headerAuthorization)
			if testCase.expectAuthorization {
				if authorizationHeader != testCase.expectedAuthorization {
					t.Fatalf("expected authorization header %s, got %s", testCase.expectedAuthorization, authorizationHeader)
				}
			} else if authorizationHeader != "" {
				t.Fatalf("did not expect authorization header, but got %s", authorizationHeader)
			}
		})
	}
}
