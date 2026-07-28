// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	netbird "github.com/netbirdio/netbird/shared/management/client/rest"
	"github.com/netbirdio/netbird/shared/management/http/api"
)

// agentNetworkClient wraps the netbird REST client for Agent Network endpoints.
// The upstream Go client does not yet expose these routes, so we call them directly.
type agentNetworkClient struct {
	c *netbird.Client
}

func newAgentNetworkClient(c *netbird.Client) *agentNetworkClient {
	return &agentNetworkClient{c: c}
}

// ---- helpers ----------------------------------------------------------------

func anDo[T any](ctx context.Context, c *netbird.Client, method, path string, body any) (T, error) {
	var zero T
	// bodyReader must stay a nil io.Reader interface (not a typed-nil
	// *bytes.Reader) when there is no body: http.NewRequestWithContext
	// type-switches on the concrete type and dereferences a *bytes.Reader,
	// so a typed-nil would panic on every body-less request.
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return zero, err
		}
		bodyReader = bytes.NewReader(b)
	}
	resp, err := c.NewRequest(ctx, method, path, bodyReader, nil)
	if err != nil {
		return zero, err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if method == "DELETE" {
		return zero, nil
	}
	var result T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return zero, err
	}
	return result, nil
}

// ---- Providers --------------------------------------------------------------

func (a *agentNetworkClient) ListProviders(ctx context.Context) ([]api.AgentNetworkProvider, error) {
	return anDo[[]api.AgentNetworkProvider](ctx, a.c, "GET", "/api/agent-network/providers", nil)
}

func (a *agentNetworkClient) GetProvider(ctx context.Context, id string) (*api.AgentNetworkProvider, error) {
	r, err := anDo[api.AgentNetworkProvider](ctx, a.c, "GET", "/api/agent-network/providers/"+id, nil)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (a *agentNetworkClient) CreateProvider(ctx context.Context, req api.PostApiAgentNetworkProvidersJSONRequestBody) (*api.AgentNetworkProvider, error) {
	r, err := anDo[api.AgentNetworkProvider](ctx, a.c, "POST", "/api/agent-network/providers", req)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (a *agentNetworkClient) UpdateProvider(ctx context.Context, id string, req api.PutApiAgentNetworkProvidersProviderIdJSONRequestBody) (*api.AgentNetworkProvider, error) {
	r, err := anDo[api.AgentNetworkProvider](ctx, a.c, "PUT", "/api/agent-network/providers/"+id, req)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (a *agentNetworkClient) DeleteProvider(ctx context.Context, id string) error {
	_, err := anDo[struct{}](ctx, a.c, "DELETE", "/api/agent-network/providers/"+id, nil)
	return err
}

// ---- Policies ---------------------------------------------------------------

func (a *agentNetworkClient) ListPolicies(ctx context.Context) ([]api.AgentNetworkPolicy, error) {
	return anDo[[]api.AgentNetworkPolicy](ctx, a.c, "GET", "/api/agent-network/policies", nil)
}

func (a *agentNetworkClient) GetPolicy(ctx context.Context, id string) (*api.AgentNetworkPolicy, error) {
	r, err := anDo[api.AgentNetworkPolicy](ctx, a.c, "GET", "/api/agent-network/policies/"+id, nil)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (a *agentNetworkClient) CreatePolicy(ctx context.Context, req api.PostApiAgentNetworkPoliciesJSONRequestBody) (*api.AgentNetworkPolicy, error) {
	r, err := anDo[api.AgentNetworkPolicy](ctx, a.c, "POST", "/api/agent-network/policies", req)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (a *agentNetworkClient) UpdatePolicy(ctx context.Context, id string, req api.PutApiAgentNetworkPoliciesPolicyIdJSONRequestBody) (*api.AgentNetworkPolicy, error) {
	r, err := anDo[api.AgentNetworkPolicy](ctx, a.c, "PUT", "/api/agent-network/policies/"+id, req)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (a *agentNetworkClient) DeletePolicy(ctx context.Context, id string) error {
	_, err := anDo[struct{}](ctx, a.c, "DELETE", "/api/agent-network/policies/"+id, nil)
	return err
}

// ---- Guardrails -------------------------------------------------------------

func (a *agentNetworkClient) ListGuardrails(ctx context.Context) ([]api.AgentNetworkGuardrail, error) {
	return anDo[[]api.AgentNetworkGuardrail](ctx, a.c, "GET", "/api/agent-network/guardrails", nil)
}

func (a *agentNetworkClient) GetGuardrail(ctx context.Context, id string) (*api.AgentNetworkGuardrail, error) {
	r, err := anDo[api.AgentNetworkGuardrail](ctx, a.c, "GET", "/api/agent-network/guardrails/"+id, nil)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (a *agentNetworkClient) CreateGuardrail(ctx context.Context, req api.PostApiAgentNetworkGuardrailsJSONRequestBody) (*api.AgentNetworkGuardrail, error) {
	r, err := anDo[api.AgentNetworkGuardrail](ctx, a.c, "POST", "/api/agent-network/guardrails", req)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (a *agentNetworkClient) UpdateGuardrail(ctx context.Context, id string, req api.PutApiAgentNetworkGuardrailsGuardrailIdJSONRequestBody) (*api.AgentNetworkGuardrail, error) {
	r, err := anDo[api.AgentNetworkGuardrail](ctx, a.c, "PUT", "/api/agent-network/guardrails/"+id, req)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (a *agentNetworkClient) DeleteGuardrail(ctx context.Context, id string) error {
	_, err := anDo[struct{}](ctx, a.c, "DELETE", "/api/agent-network/guardrails/"+id, nil)
	return err
}

// ---- Settings ---------------------------------------------------------------

func (a *agentNetworkClient) GetSettings(ctx context.Context) (*api.AgentNetworkSettings, error) {
	resp, err := a.c.NewRequest(ctx, "GET", "/api/agent-network/settings", nil, nil)
	if err != nil {
		return nil, err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// The server deliberately answers 200 with a JSON `null` body (not 404)
	// when the account's agent-network settings row has not been bootstrapped.
	// Decoding that would yield a bogus all-zero settings row, so translate it
	// into a NotFound error and let callers treat the resource as absent.
	if trimmed := bytes.TrimSpace(body); len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, &netbird.APIError{StatusCode: http.StatusNotFound, Message: "agent network settings not found"}
	}
	var result api.AgentNetworkSettings
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (a *agentNetworkClient) UpdateSettings(ctx context.Context, req api.PutApiAgentNetworkSettingsJSONRequestBody) (*api.AgentNetworkSettings, error) {
	r, err := anDo[api.AgentNetworkSettings](ctx, a.c, "PUT", "/api/agent-network/settings", req)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
