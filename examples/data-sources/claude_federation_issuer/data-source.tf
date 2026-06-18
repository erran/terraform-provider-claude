data "claude_federation_issuer" "example" {
  id = "fdis_01abc123"
}

output "federation_issuer_name" {
  value = data.claude_federation_issuer.example.name
}

output "federation_issuer_url" {
  value = data.claude_federation_issuer.example.issuer_url
}
