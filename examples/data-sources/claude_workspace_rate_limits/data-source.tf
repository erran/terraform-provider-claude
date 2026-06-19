# Rate limit overrides configured for a single workspace. Anything absent from
# the result is inherited from the organization-level limits.
data "claude_workspace_rate_limits" "example" {
  workspace_id = "wrkspc_01ABCdef"
}

# Restrict the response to a single category.
data "claude_workspace_rate_limits" "models" {
  workspace_id = "wrkspc_01ABCdef"
  group_type   = "model_group"
}

output "workspace_overrides" {
  value = data.claude_workspace_rate_limits.example.rate_limits
}
