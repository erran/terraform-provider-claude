# Users cannot be created via the API — they join the organization by accepting
# an invite. This resource ADOPTS an existing member and manages their role.
# The user identified by user_id must already be a member of the organization.
resource "claude_organization_member" "alice" {
  user_id = "user_01WCz1FkmYMm4gnmykNKUu3Q"
  role    = "developer"
}
