# All organization rate limit groups.
data "claude_organization_rate_limits" "all" {}

# Only the group that the given model falls under.
data "claude_organization_rate_limits" "opus" {
  model = "claude-opus-4-8"
}

# Only the Message Batches API limits.
data "claude_organization_rate_limits" "batch" {
  group_type = "batch"
}

output "opus_limits" {
  value = one(data.claude_organization_rate_limits.opus.rate_limits).limits
}
