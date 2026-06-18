// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccServiceAccountResource(t *testing.T) {
	name := fmt.Sprintf("tf-acc-%s", acctest.RandStringFromCharSet(8, "abcdefghijklmnopqrstuvwxyz0123456789"))
	renamed := name + "-renamed"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and read.
			{
				Config: testAccServiceAccountConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("claude_service_account.test", "name", name),
					resource.TestCheckResourceAttr("claude_service_account.test", "organization_role", "developer"),
					resource.TestCheckResourceAttr("claude_service_account.test", "type", "service_account"),
					resource.TestMatchResourceAttr("claude_service_account.test", "id", regexp.MustCompile(`^svac_`)),
					resource.TestCheckResourceAttrSet("claude_service_account.test", "created_at"),
				),
			},
			// Import.
			{
				ResourceName:      "claude_service_account.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update the name in place.
			{
				Config: testAccServiceAccountConfig(renamed),
				Check:  resource.TestCheckResourceAttr("claude_service_account.test", "name", renamed),
			},
		},
	})
}

func testAccServiceAccountConfig(name string) string {
	return fmt.Sprintf(`
resource "claude_service_account" "test" {
  name = %[1]q
}
`, name)
}
