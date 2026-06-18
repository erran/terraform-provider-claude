data "claude_organization" "current" {}

output "organization_name" {
  value = data.claude_organization.current.name
}
