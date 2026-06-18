terraform {
  required_providers {
    claude = {
      source = "gitlab-org/claude"
    }
  }
}

# Authenticate with an org:admin OAuth bearer token. The recommended way to
# supply it is the ANTHROPIC_OAUTH_TOKEN environment variable, obtained with
# the `ant` CLI:
#
#   ant auth login --profile admin --scope "org:admin"
#   export ANTHROPIC_OAUTH_TOKEN=$(ant auth print-credentials --profile admin --access-token)
#
provider "claude" {
  # oauth_token = var.claude_oauth_token # optional; defaults to $ANTHROPIC_OAUTH_TOKEN
}
