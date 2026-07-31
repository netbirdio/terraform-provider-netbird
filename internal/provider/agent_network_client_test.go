package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
)

// Body-less requests (every GET/DELETE) must not panic. Regression test for a
// typed-nil *bytes.Reader being passed as the io.Reader body: as a non-nil
// interface wrapping a nil pointer it made http.NewRequestWithContext
// dereference the nil reader and panic on every Read/List/Delete.
func Test_agentNetworkClient_bodylessRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			defer r.Body.Close()
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Valid JSON for the GET decode path; ignored by DELETE.
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := newAgentNetworkClient(netbird.New(server.URL, "test-token"))
	ctx := context.Background()

	if _, err := client.ListProviders(ctx); err != nil {
		t.Fatalf("ListProviders (body-less GET) returned error: %v", err)
	}
	if err := client.DeleteProvider(ctx, "some-id"); err != nil {
		t.Fatalf("DeleteProvider (body-less DELETE) returned error: %v", err)
	}
}

// The server deliberately answers 200 with a JSON `null` body (not 404) when the
// account's agent-network settings row has not been bootstrapped. GetSettings
// must report that as NotFound so Read removes the resource instead of writing
// an all-zero settings row into state.
func Test_agentNetworkClient_GetSettings(t *testing.T) {
	cases := []struct {
		name         string
		body         string
		wantNotFound bool
		wantCluster  string
	}{
		{name: "null body is not found", body: "null", wantNotFound: true},
		{name: "empty body is not found", body: "", wantNotFound: true},
		{
			name:        "populated body decodes",
			body:        `{"cluster":"c1","subdomain":"sub","endpoint":"sub.c1","enable_log_collection":true}`,
			wantCluster: "c1",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(c.body))
			}))
			defer server.Close()

			client := newAgentNetworkClient(netbird.New(server.URL, "test-token"))
			s, err := client.GetSettings(context.Background())

			if c.wantNotFound {
				if err == nil {
					t.Fatalf("Expected an error, got settings %+v", s)
				}
				if !netbird.IsNotFound(err) {
					t.Fatalf("Expected netbird.IsNotFound(err), got: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
			if s.Cluster != c.wantCluster {
				t.Fatalf("Expected cluster %q, got %q", c.wantCluster, s.Cluster)
			}
		})
	}
}
