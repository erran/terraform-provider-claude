// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories instantiates the provider under test for
// acceptance tests, returning a protocol v6 server.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"claude": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck verifies that some org:admin credential is available before
// running acceptance tests, which create real resources. Either a static token
// (ANTHROPIC_OAUTH_TOKEN) or the Workload Identity Federation variables must be
// set.
func testAccPreCheck(t *testing.T) {
	if os.Getenv("ANTHROPIC_OAUTH_TOKEN") != "" {
		return
	}
	if os.Getenv("ANTHROPIC_FEDERATION_RULE_ID") != "" {
		return
	}
	t.Fatal("acceptance tests require ANTHROPIC_OAUTH_TOKEN or the Workload Identity Federation variables (ANTHROPIC_FEDERATION_RULE_ID, ...) to be set")
}
