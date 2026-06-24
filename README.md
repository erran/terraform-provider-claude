# Terraform Provider for Claude

A [Terraform](https://www.terraform.io) provider for managing
[Claude Admin API](https://platform.claude.com/docs/en/manage-claude/admin-api)
resources, with a focus on
[Workload Identity Federation](https://platform.claude.com/docs/en/manage-claude/wif-admin-api).

This provider is built on the
[Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework)
and is based on the
[terraform-provider-scaffolding-framework](https://github.com/hashicorp/terraform-provider-scaffolding-framework)
template.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://go.dev/doc/install) >= 1.24 (to build the provider from source)

## Building

```shell
go build ./...
```

## Authentication

The provider authenticates against the Claude Admin API with an OAuth bearer
token carrying the `org:admin` scope. The scope is granted only to organization
members with the admin, owner, or primary owner role. Admin API keys
(`x-api-key`) are **not** accepted on these endpoints.

Obtain an interactive token with the [`ant` CLI](https://platform.claude.com/docs/en/cli-sdks-libraries/cli/quickstart)
and export it as `ANTHROPIC_OAUTH_TOKEN`:

```shell
ant auth login --profile admin --scope "org:admin"
export ANTHROPIC_OAUTH_TOKEN=$(ant auth print-credentials --profile admin --access-token)
```

Interactive tokens are short-lived; if requests start returning `401`, re-run
the export command (it refreshes the token automatically).

The token may also be passed explicitly via the provider's `oauth_token`
attribute, but the environment variable is recommended so secrets stay out of
configuration and state.

### Workload Identity Federation (CI and automation)

For CI and automation, the provider can authenticate without any static secret
by exchanging an OIDC identity token for a short-lived `org:admin` bearer token
(see
[Bootstrap a workload to manage WIF](https://platform.claude.com/docs/en/manage-claude/wif-admin-api#bootstrap-a-workload-to-manage-wif)).
The `org:admin` scope is granted by a federation rule, which must be created
once in the Claude Console with `oauth_scope: org:admin` targeting an admin
service account — granting a workload organization-admin access is a deliberate
human action that automation cannot bootstrap for itself.

The provider posts the identity token to `POST /v1/oauth/token` (the
[RFC 7523](https://www.rfc-editor.org/rfc/rfc7523) `jwt-bearer` grant) along
with the federation rule, organization, and service account IDs, then uses the
minted token for all Admin API calls.

In GitLab CI, mint the identity token with an `id_tokens` block and supply the
federation IDs as CI/CD variables:

```yaml
manage-wif:
  id_tokens:
    ANTHROPIC_IDENTITY_TOKEN:
      aud: https://api.anthropic.com # must match the federation rule's audience
  variables:
    ANTHROPIC_FEDERATION_RULE_ID: $ANTHROPIC_FEDERATION_RULE_ID
    ANTHROPIC_ORGANIZATION_ID: $ANTHROPIC_ORGANIZATION_ID
    ANTHROPIC_SERVICE_ACCOUNT_ID: $ANTHROPIC_SERVICE_ACCOUNT_ID
  script:
    - terraform apply -auto-approve
```

The provider reads `ANTHROPIC_IDENTITY_TOKEN` (or `ANTHROPIC_IDENTITY_TOKEN_FILE`),
`ANTHROPIC_FEDERATION_RULE_ID`, `ANTHROPIC_ORGANIZATION_ID`,
`ANTHROPIC_SERVICE_ACCOUNT_ID`, and the optional `ANTHROPIC_WORKSPACE_ID` from
the environment, or the equivalent provider attributes. A static `oauth_token`
takes precedence over federation when both are present.

## Usage

```hcl
terraform {
  required_providers {
    claude = {
      source = "erran/claude"
    }
  }
}

# Reads ANTHROPIC_OAUTH_TOKEN from the environment.
provider "claude" {}
```

## Resources

### `claude_service_account`

Manages a Workload Identity Federation
[service account](https://platform.claude.com/docs/en/manage-claude/wif-admin-api#service-accounts)
— the non-human identity (`svac_...`) that a federated token acts as.

```hcl
resource "claude_service_account" "inference_worker" {
  name              = "inference-worker"
  organization_role = "developer"
}
```

Deleting the resource archives the service account (a soft delete). Archiving
fails while a live federation rule still references the account, so archive the
rule first. Existing service accounts can be imported by id:

```shell
terraform import claude_service_account.inference_worker svac_0123456789abcdef
```

### `claude_federation_issuer`

Manages a
[federation issuer](https://platform.claude.com/docs/en/manage-claude/wif-admin-api#federation-issuers)
(`fdis_...`) — an external OIDC provider Anthropic trusts. The JWKS source is a
discriminated union selected by `jwks_type` (`discovery`, `explicit_url`, or
`inline`).

```hcl
resource "claude_federation_issuer" "github_actions" {
  name       = "github-actions"
  issuer_url = "https://token.actions.githubusercontent.com"
  # jwks_type defaults to "discovery"
}
```

Deleting the resource archives the issuer; archiving fails while a live
federation rule still references it.

### `claude_federation_rule`

Manages a
[federation rule](https://platform.claude.com/docs/en/manage-claude/wif-admin-api#federation-rules)
(`fdrl_...`) binding an issuer to a service account. OAuth-authenticated callers
may only manage rules whose `oauth_scope` is `workspace:developer` or
`workspace:inference`; other scopes (such as `org:admin`) must be managed in the
Console.

```hcl
resource "claude_federation_rule" "gha_deploy" {
  name        = "gha-deploy"
  issuer_id   = claude_federation_issuer.github_actions.id
  oauth_scope = "workspace:developer"

  match = {
    subject_prefix = "repo:my-org/my-repo:ref:refs/heads/main"
  }

  target = {
    service_account_id = claude_service_account.inference_worker.id
  }

  workspace_id = "wrkspc_..."
}
```

Changing `issuer_id` or `workspace_id` forces replacement. Deleting the
resource archives the rule.

### Other Admin API resources

| Resource | Manages | Notes |
| --- | --- | --- |
| `claude_service_account_workspace` | A service account's membership in a workspace | Composite id `<service_account_id>/<workspace_id>`; the implicit default-workspace membership is not managed here. |
| `claude_federation_rule_workspace` | A workspace a federation rule is enabled in | Composite id `<federation_rule_id>/<workspace_id>`; for workspaces beyond the rule's initial `workspace_id`. |
| `claude_workspace` | Organization workspaces (`wrkspc_...`) | Delete archives the workspace. |
| `claude_workspace_member` | A user's membership/role in a workspace | Composite id `<workspace_id>/<user_id>`. |
| `claude_organization_invite` | Pending organization invites | No update endpoint; `email`/`role` force replacement. |
| `claude_organization_member` | The role of an existing org member | Members are created via invites, not the API; the resource adopts an existing user. |
| `claude_api_key` | Name and status of an existing API key | Keys can't be created or deleted via the API; `create` adopts an existing key and `delete` is a no-op. |
| `claude_skill` | A custom [Agent Skill](https://platform.claude.com/docs/en/api/python/beta/skills/create) (`skill_...`) | Beta API; `files` seed the first version and are write-only. Changing `files` uploads a new version; `display_title` is immutable. |
| `claude_skill_version` | An immutable version of a skill (`skillver_...`) | Beta API; composite import `<skill_id>/<version>`. `files` are write-only; `skill_id`/`files` force replacement. |

## Data sources

Read-only lookups of existing resources:

| Data source | Returns |
| --- | --- |
| `claude_organization` | The current organization (`/v1/organizations/me`). |
| `claude_workspace` | A single workspace by `id`. |
| `claude_workspaces` | All workspaces, with an optional `include_archived` filter. |
| `claude_service_account` | A single service account by `id`. |
| `claude_federation_issuer` | A single federation issuer by `id`. |
| `claude_federation_rule` | A single federation rule by `id`. |
| `claude_organization_rate_limits` | Organization-level [rate limits](https://platform.claude.com/docs/en/manage-claude/rate-limits-api), with optional `model` and `group_type` filters. |
| `claude_workspace_rate_limits` | A workspace's rate limit overrides by `workspace_id`, with an optional `group_type` filter. |

```hcl
data "claude_organization" "current" {}

data "claude_workspaces" "all" {}

output "org_name" {
  value = data.claude_organization.current.name
}
```

## Development

```shell
make build    # compile
make test     # unit tests
make testacc  # acceptance tests (creates real resources; needs org:admin creds)
```

## Continuous integration

`.gitlab-ci.yml` defines three stages: `build`, `test`, and `acceptance`. The
acceptance job authenticates with Workload Identity Federation instead of a
static token, so no long-lived secret lives in CI. See
[Authentication](#authentication) and the WIF section below. It runs only when
`ANTHROPIC_FEDERATION_RULE_ID` is set, so unconfigured pipelines (e.g. forks)
are not failed.

## License

Released under the [MIT License](./LICENSE). Copyright (c) 2026 Erran Carey
<ecarey@gitlab.com>.
