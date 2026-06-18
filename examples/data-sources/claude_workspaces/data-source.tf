data "claude_workspaces" "all" {}

data "claude_workspaces" "including_archived" {
  include_archived = true
}

output "workspace_names" {
  value = [for w in data.claude_workspaces.all.workspaces : w.name]
}
