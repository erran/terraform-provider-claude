# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Terraform provider for the [Claude Admin API](https://platform.claude.com/docs/en/manage-claude/admin-api),
built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).
The Go module path is `github.com/erran/terraform-provider-claude` and the
registry address is `registry.terraform.io/erran/claude`.

## Layout

- `internal/client/` — a thin HTTP client for the Admin API. One file per API
  object (e.g. `service_account.go`, `workspace.go`, `rate_limit.go`). All
  requests go through `Client.Do` in `client.go`, which sets auth headers,
  handles JSON encode/decode, and returns an `*APIError` for non-2xx responses.
- `internal/provider/` — the provider plus one file per resource
  (`*_resource.go`) and data source (`*_data_source.go`). `provider.go` wires
  authentication and registers every resource and data source.
- `examples/` — `.tf` snippets surfaced in the generated docs. Resources also
  have an `import.sh` showing `terraform import`.
- `docs/` — **generated**; never hand-edit. Run the generator (see below).

Authentication is solved once in `provider.go` (static `org:admin` OAuth token
or Workload Identity Federation token exchange). New resources and data sources
receive a ready `*client.Client` via `Configure` and never deal with auth.

## Conventions

- Start every Go file with:
  ```go
  // Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
  // SPDX-License-Identifier: MIT
  ```
- Decide resource vs. data source by the API surface. A **resource** needs
  create/update/delete; a read-only endpoint (e.g. the Rate Limits API) must be
  a **data source**. Do not fake a managed resource over a read-only endpoint.
- Constructors are exported (`NewXxxResource` / `NewXxxDataSource`); the
  implementing struct, its `*Model`, and helpers stay unexported.
- Assert interface conformance with `var _ resource.Resource = &xxx{}` (add
  `ResourceWithConfigure` and `ResourceWithImportState` as applicable).
- `Metadata` sets `resp.TypeName = req.ProviderTypeName + "_<name>"`.
- `Configure` type-asserts `req.ProviderData.(*client.Client)`; guard the
  `nil` ProviderData case and emit the standard "Unexpected ... Configure Type"
  error otherwise. Copy an existing one verbatim.
- Map nullable/optional API strings and ints with the helpers in `helpers.go`
  (`optionalString`, `optionalInt64`); add a helper there rather than inlining
  null checks. A `nil` Go slice maps to a Terraform null list, so preserve
  `nil` (don't allocate an empty slice) when the API returns null.
- On a 404 during `Read`, a resource calls `resp.State.RemoveResource(ctx)`; a
  data source surfaces a clear error. Detect it with `client.NotFound(err)`.
- Write descriptions for every attribute — they become the generated docs.
- Match the comment density and naming of the nearest existing file.
- In `required_providers` blocks (examples, docs templates, READMEs), write the
  provider source fully qualified as `registry.terraform.io/erran/claude`, not
  the shorthand `erran/claude`. `tfplugindocs` regenerates pages from the
  fully-qualified form, so the shorthand produces a spurious diff on the next
  `go generate`.

## Adding a data source

1. **Client method** in `internal/client/<object>.go`: define the API struct
   with `json` tags, a list-envelope struct (`Data` + `NextPage`) if the
   endpoint paginates, and a method on `*Client` that calls `c.Do`. Follow the
   `next_page` cursor loop in `workspace_list.go` / `rate_limit.go` for lists.
2. **Data source** in `internal/provider/<name>_data_source.go`: constructor,
   struct, `*Model` with `tfsdk` tags, and `Metadata` / `Schema` / `Configure`
   / `Read`. Input arguments are `Required`/`Optional`; everything fetched is
   `Computed`. Model `Read` on `organization_rate_limits_data_source.go` (filters
   + nested list) or `workspace_data_source.go` (lookup by id).
3. **Register** the constructor in `DataSources` in `provider.go`.
4. **Example** at `examples/data-sources/claude_<name>/data-source.tf`.
5. **Docs + README**: regenerate docs and add a row to the README data-source
   table.

## Adding a resource

Same as above, but implement `Create` / `Read` / `Update` / `Delete` and
usually `ImportState` (`resource.ImportStatePassthroughID` for a simple id).
Use `internal/provider/service_account_resource.go` as the reference:

- `id` and other server-set fields are `Computed` with a
  `stringplanmodifier.UseStateForUnknown()` plan modifier.
- Validate inputs with `stringvalidator` (length, `OneOf`, `RegexMatches`); set
  defaults with `stringdefault.StaticString`.
- A field that can't be changed in place should force replacement via
  `RequiresReplace()` rather than a no-op `Update`.
- If the API has no hard delete, document the soft-delete (archive) semantics
  and make `Delete` idempotent (treat 404 as success).
- Register the constructor in `Resources` in `provider.go`, add an example
  `resource.tf` and `import.sh` under `examples/resources/claude_<name>/`.

## Build, test, and docs

```shell
make build    # go build ./...
make test     # go test ./... (acceptance tests are skipped unless TF_ACC=1)
make testacc  # TF_ACC=1 go test ./... -v; creates real resources, needs org:admin creds
make fmt      # gofmt -s -w
make vet      # go vet ./...
```

Tests are `terraform-plugin-testing` acceptance tests (`resource.Test`), gated
by `TF_ACC=1` and requiring an `org:admin` credential (`ANTHROPIC_OAUTH_TOKEN`
or the Workload Identity Federation env vars — see `testAccPreCheck`). Without
`TF_ACC` they no-op, so `make test` passes offline but exercises nothing. Run a
single test by name:

```shell
TF_ACC=1 go test ./internal/provider/ -run TestAccServiceAccountResource -v
```

Regenerate `docs/` after any schema or example change:

```shell
go generate ./...
```

`go generate` (and the test harness) shells out to a Terraform CLI for
`terraform fmt` and `tfplugindocs`. Before running anything that invokes it,
check what is on `PATH` and pick a binary in this order:

1. `terraform` if `command -v terraform` succeeds.
2. Otherwise `tofu` (OpenTofu is a drop-in CLI; `/opt/homebrew/bin/tofu` here).
3. Otherwise a mise-managed Terraform, run via `mise exec`.

```shell
command -v terraform >/dev/null && go generate ./...           # 1: terraform on PATH
command -v tofu      >/dev/null && tofu fmt -recursive ./examples  # 2: tofu fallback
mise exec terraform@1.14.5 -- go generate ./...                # 3: via mise
```

`go generate`'s directives hardcode `terraform`, so when only `tofu` or a
mise-managed Terraform is available, run the underlying steps (`tofu fmt`,
`tfplugindocs generate`) directly or shim `terraform` onto `PATH` rather than
assuming the bare `go generate ./...` will resolve a CLI.

Keep generated-doc changes scoped to your feature: `go generate` may reformat
unrelated examples or rewrite other pages from templates. Revert those
incidental diffs (`git checkout -- <path>`) so the change stays reviewable.
