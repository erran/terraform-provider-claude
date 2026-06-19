# Bootstrapping Workload Identity Federation

This guide explains how to authenticate the provider with
[Workload Identity Federation](https://platform.claude.com/docs/en/manage-claude/wif-admin-api)
(WIF) so that CI and automation can manage Claude Admin API resources without a
long-lived secret.

For a complete, working example, see
[gitlab-org/ai/claude-federation](https://gitlab.com/gitlab-org/ai/claude-federation),
which bootstraps and manages federation with this provider end to end.

## How it works

Instead of a static `org:admin` OAuth token, the provider exchanges a
short-lived OIDC identity token (such as a GitLab CI `id_token`) for a
short-lived `org:admin` bearer token. It posts the identity token to
`POST /v1/oauth/token` (the [RFC 7523](https://www.rfc-editor.org/rfc/rfc7523)
`jwt-bearer` grant) along with the federation rule, organization, and service
account IDs, then uses the minted token for every Admin API call.

The granted scope is determined by the **federation rule**, which must already
exist in the Claude Console with `oauth_scope: org:admin` targeting an admin
service account. Granting a workload organization-admin access is a deliberate
human action that automation cannot bootstrap for itself — create the rule once
by hand, then let CI assume it.

## Inputs

The provider reads each value from a provider attribute or its environment
variable (the attribute wins when both are set):

| Attribute | Environment variable | Purpose |
| --- | --- | --- |
| `identity_token` | `ANTHROPIC_IDENTITY_TOKEN` | The OIDC identity token (JWT) to exchange. Mutually exclusive with `identity_token_file`. |
| `identity_token_file` | `ANTHROPIC_IDENTITY_TOKEN_FILE` | Path to a file containing the identity token. |
| `federation_rule_id` | `ANTHROPIC_FEDERATION_RULE_ID` | Federation rule (`fdrl_...`) to evaluate the token against; its `oauth_scope` must be `org:admin`. |
| `organization_id` | `ANTHROPIC_ORGANIZATION_ID` | Anthropic organization UUID for the exchange. |
| `service_account_id` | `ANTHROPIC_SERVICE_ACCOUNT_ID` | Target service account (`svac_...`); must have the admin organization role. |
| `workspace_id` | `ANTHROPIC_WORKSPACE_ID` | Optional. Required only when the federation rule covers more than one workspace. |

A static `oauth_token` (or `ANTHROPIC_OAUTH_TOKEN`) takes precedence over
federation when both are present, so leave it unset in CI.

## Passing the variables in

The simplest path is to set the environment variables and leave the provider
block empty — the provider reads them automatically:

```hcl
terraform {
  required_providers {
    claude = {
      source = "registry.terraform.io/erran/claude"
    }
  }
}

# Reads ANTHROPIC_IDENTITY_TOKEN and the ANTHROPIC_* federation IDs from the
# environment.
provider "claude" {}
```

Or pass them explicitly as attributes (for example, wired to Terraform
variables):

```hcl
provider "claude" {
  identity_token     = var.anthropic_identity_token
  federation_rule_id = var.anthropic_federation_rule_id
  organization_id    = var.anthropic_organization_id
  service_account_id = var.anthropic_service_account_id
}
```

### GitLab CI

Mint the identity token with an `id_tokens` block and supply the federation IDs
as CI/CD variables. The token's `aud` must match the audience configured on the
federation rule:

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

The `id_tokens` block exports `ANTHROPIC_IDENTITY_TOKEN` into the job
environment, and the provider exchanges it on the first Admin API call. Store
the federation IDs as project or group CI/CD variables; none of them are
secret, but keeping them out of `.gitlab-ci.yml` makes the pipeline portable.

See [gitlab-org/ai/claude-federation](https://gitlab.com/gitlab-org/ai/claude-federation)
for a real pipeline that uses this pattern.
