terraform {
  required_version = ">= 1.0"

  required_providers {
    openai = {
      source  = "rdsubhas/openai"
      version = "~> 3.0"
    }
  }
}
