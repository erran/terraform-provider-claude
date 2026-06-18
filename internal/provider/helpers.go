// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package provider

import "github.com/hashicorp/terraform-plugin-framework/types"

// optionalString maps an API string to a Terraform value, treating the empty
// string as null so unset optional fields do not show spurious diffs.
func optionalString(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
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
