terraform {
  required_providers {
    claude = {
      source = "registry.terraform.io/erran/claude"
    }
  }
}

# Interactive use: authenticate with a static org:admin OAuth token. The
# recommended way to supply it is the ANTHROPIC_OAUTH_TOKEN environment
# variable, obtained with the `ant` CLI:
#
#   ant auth login --profile admin --scope "org:admin"
#   export ANTHROPIC_OAUTH_TOKEN=$(ant auth print-credentials --profile admin --access-token)
#
provider "claude" {
  # oauth_token = var.claude_oauth_token # optional; defaults to $ANTHROPIC_OAUTH_TOKEN
}

# CI and automation: authenticate with Workload Identity Federation. The
# provider exchanges an OIDC identity token (e.g. a GitLab CI id_token) for a
# short-lived org:admin bearer token. All of these may instead be supplied via
# ANTHROPIC_* environment variables.
#
# provider "claude" {
#   identity_token     = var.gitlab_id_token        # $ANTHROPIC_IDENTITY_TOKEN
#   federation_rule_id = "fdrl_..."                 # $ANTHROPIC_FEDERATION_RULE_ID
#   organization_id    = "00000000-0000-0000-0000-000000000000" # $ANTHROPIC_ORGANIZATION_ID
#   service_account_id = "svac_..."                 # $ANTHROPIC_SERVICE_ACCOUNT_ID
# }
