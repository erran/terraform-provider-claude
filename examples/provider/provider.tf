terraform {
  required_providers {
    claude = {
      source = "gitlab-org/claude"
    }
  }
}

provider "claude" {}
