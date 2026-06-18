resource "claude_service_account" "inference_worker" {
  name              = "inference-worker"
  organization_role = "developer"
}
