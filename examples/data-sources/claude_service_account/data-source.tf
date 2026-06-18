data "claude_service_account" "example" {
  id = "svac_01234567890abcdefghijklmn"
}

output "service_account_name" {
  value = data.claude_service_account.example.name
}

output "service_account_role" {
  value = data.claude_service_account.example.organization_role
}
