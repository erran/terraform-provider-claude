# GitHub Actions, with JWKS discovery (the default).
resource "claude_federation_issuer" "github_actions" {
  name       = "github-actions"
  issuer_url = "https://token.actions.githubusercontent.com"
}

# An issuer that serves its keys from a fixed JWKS endpoint.
resource "claude_federation_issuer" "internal" {
  name       = "internal-idp"
  issuer_url = "https://idp.example.com"
  jwks_type  = "explicit_url"
  jwks_url   = "https://idp.example.com/keys"
}
