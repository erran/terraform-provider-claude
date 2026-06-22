# Grant a service account access to a non-default workspace so federated
# tokens can act in it.
resource "claude_service_account_workspace" "inference_worker_staging" {
  service_account_id = claude_service_account.inference_worker.id
  workspace_id       = "wrkspc_0123456789abcdef"
  workspace_role     = "workspace_developer"
}
