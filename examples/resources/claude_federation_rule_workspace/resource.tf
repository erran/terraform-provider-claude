# Enable an existing federation rule in an additional workspace beyond the one
# set on the rule itself.
resource "claude_federation_rule_workspace" "gha_deploy_staging" {
  federation_rule_id = claude_federation_rule.gha_deploy.id
  workspace_id       = "wrkspc_0123456789abcdef"
}
