terraform {
  required_providers {
    openai = {
      source = "rdsubhas/openai"
    }
  }
}

provider "openai" {
  # Admin key is loaded from OPENAI_ADMIN_KEY environment variable
}

