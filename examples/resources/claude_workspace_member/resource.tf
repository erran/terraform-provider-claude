resource "claude_workspace_member" "example" {
  workspace_id   = "wrkspc_0123456789abcdef"
  user_id        = "user_0123456789abcdef"
  workspace_role = "workspace_developer"
}
