terraform {
  required_providers {
    openai = {
      source = "rdsubhas/openai"
    }
  }
}

provider "openai" {
  # API key is loaded from OPENAI_API_KEY environment variable
}