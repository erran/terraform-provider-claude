# claude_api_key adopts an EXISTING API key and manages its name and status.
#
# API keys cannot be created or deleted through the Admin API. They must be
# created (and ultimately deleted) in the Anthropic Console:
#   https://console.anthropic.com
#
# Copy the key ID from the Console ("apikey_...") and supply it as api_key_id.
# Destroying this resource removes it from Terraform state only; the key itself
# continues to exist in the Console.

resource "claude_api_key" "example" {
  # Required: the ID of the pre-existing API key to manage.
  api_key_id = "apikey_01Rj2N8SVvo6BePZj99NhmiT"

  # Optional: rename the key in place.
  name = "CI Deploy Key"

  # Optional: set the key status. One of: active, inactive, archived.
  # Note: "expired" may appear as a read value when a key has passed its
  # expiry date, but it cannot be set via the API.
  status = "active"
}

# Output the partial hint so operators can confirm which raw key is managed.
output "api_key_hint" {
  value = claude_api_key.example.partial_key_hint
}
