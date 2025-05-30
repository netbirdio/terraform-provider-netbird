// Copyright (c) HashiCorp, Inc.

package provider

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
// var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
// 	"netbird": providerserver.NewProtocol6WithError(New("test")()),
// }

// testAccProtoV6ProviderFactoriesWithEcho includes the echo provider alongside the netbird provider.
// It allows for testing assertions on data returned by an ephemeral resource during Open.
// The echoprovider is used to arrange tests by echoing ephemeral data into the Terraform state.
// This lets the data be referenced in test assertions with state checks.
// var testAccProtoV6ProviderFactoriesWithEcho = map[string]func() (tfprotov6.ProviderServer, error){
// 	"netbird": providerserver.NewProtocol6WithError(New("test")()),
// 	"echo":    echoprovider.NewProviderServer(),
// }

// func testAccPreCheck(t *testing.T) {
// You can add code here to run prior to any test case execution, for example assertions
// about the appropriate environment variables being set are common to see in a pre-check
// function.
// }

func valPtr[T any](s T) *T {
	return &s
}
