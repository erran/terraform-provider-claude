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

## Usage

```hcl
terraform {
  required_providers {
    claude = {
      source = "gitlab-org/claude"
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
