// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import (
	"context"
	"sort"

	"github.com/erran/terraform-provider-claude/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// optionalString maps an API string to a Terraform value, treating the empty
// string as null so unset optional fields do not show spurious diffs.
func optionalString(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// skillFilesFromPlan converts a Terraform map of filename to file content into
// the client's file list, ordered by path for a deterministic upload. A null or
// empty map returns a nil slice (no files to upload).
func skillFilesFromPlan(ctx context.Context, files types.Map) ([]client.SkillFile, diag.Diagnostics) {
	if files.IsNull() || files.IsUnknown() {
		return nil, nil
	}

	var contents map[string]string
	diags := files.ElementsAs(ctx, &contents, false)
	if diags.HasError() {
		return nil, diags
	}

	paths := make([]string, 0, len(contents))
	for path := range contents {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	out := make([]client.SkillFile, 0, len(paths))
	for _, path := range paths {
		out = append(out, client.SkillFile{Path: path, Content: []byte(contents[path])})
	}
	return out, diags
}

// optionalInt64 maps an API integer pointer to a Terraform value, treating a
// nil pointer as null so unset optional fields do not show spurious diffs.
func optionalInt64(v *int64) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*v)
}

// int64PointerFromPlan returns a pointer to the value, or nil when the planned
// value is null or unknown, so it is omitted from the request body.
func int64PointerFromPlan(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return v.ValueInt64Pointer()
}

// boolPointerFromPlan returns a pointer to the value, or nil when the planned
// value is null or unknown, so it is omitted from the request body.
func boolPointerFromPlan(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return v.ValueBoolPointer()
}
