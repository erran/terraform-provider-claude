data "claude_workspace" "example" {
  id = "wrkspc_01ABCdef"
}

output "workspace_name" {
  value = data.claude_workspace.example.name
}
