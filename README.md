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

## Usage

```hcl
terraform {
  required_providers {
    claude = {
      source = "gitlab-org/claude"
    }
  }
}

provider "claude" {}
```

## License

Released under the [MIT License](./LICENSE). Copyright (c) 2026 Erran Carey
<ecarey@gitlab.com>.
