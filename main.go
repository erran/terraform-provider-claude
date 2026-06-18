// Copyright (c) 2026 Erran Carey <ecarey@gitlab.com>
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"flag"
	"log"

	"github.com/erran/terraform-provider-claude/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// Format example terraform files so the generated docs are tidy.
//go:generate terraform fmt -recursive ./examples/

// Generate the docs/ directory consumed by the Terraform Registry from the
// provider schema and the examples/ directory.
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name claude

// these will be set by the goreleaser configuration
// to appropriate values for the compiled binary.
var version string = "dev"

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/erran/claude",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
