# Import an existing API key into Terraform state using its ID ("apikey_...").
# Find the ID in the Anthropic Console: https://console.anthropic.com
#
# The import ID is the API key's id field (same as api_key_id in the resource).
terraform import claude_api_key.example apikey_01Rj2N8SVvo6BePZj99NhmiT
