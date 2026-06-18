data "claude_federation_rule" "example" {
  id = "fdrl_01234567890abcdefghijklmn"
}

output "federation_rule_name" {
  value = data.claude_federation_rule.example.name
}

output "federation_rule_issuer_id" {
  value = data.claude_federation_rule.example.issuer_id
}

output "federation_rule_oauth_scope" {
  value = data.claude_federation_rule.example.oauth_scope
}
