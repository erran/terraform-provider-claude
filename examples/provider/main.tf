data "claude_organization" "current" {}

output "organization_name" {
  value = data.claude_organization.current.name
}

resource "claude_service_account" "inference-worker" {
  name              = "inference-worker"
  organization_role = "developer"
}
