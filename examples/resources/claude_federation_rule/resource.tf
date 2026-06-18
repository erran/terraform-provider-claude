# GitHub Actions deploys from the main branch act as a service account.
resource "claude_federation_rule" "gha_deploy" {
  name       = "gha-deploy"
  issuer_id  = claude_federation_issuer.github_actions.id
  oauth_scope = "workspace:developer"

  match = {
    subject_prefix = "repo:my-org/my-repo:ref:refs/heads/main"
    claims = {
      repository_owner = "my-org"
    }
  }

  target = {
    service_account_id = claude_service_account.inference_worker.id
  }

  workspace_id           = "wrkspc_..."
  token_lifetime_seconds = 600
}
