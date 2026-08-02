# Terraform Provider for OpenAI

[![CI](https://github.com/rdsubhas/terraform-provider-openai/actions/workflows/ci.yml/badge.svg)](https://github.com/rdsubhas/terraform-provider-openai/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/rdsubhas/terraform-provider-openai)](https://github.com/rdsubhas/terraform-provider-openai/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/rdsubhas/terraform-provider-openai/v3)](https://goreportcard.com/report/github.com/rdsubhas/terraform-provider-openai/v3)
[![License: MPL-2.0](https://img.shields.io/badge/license-MPL--2.0-brightgreen.svg)](LICENSE)

Manage OpenAI projects, access, files, models, vector stores, fine-tuning, and model
operations with Terraform.

## Built for high-volume OpenAI administration

This fork is for Terraform configurations that manage hundreds of OpenAI projects and
thousands of project rate limits. At that volume, one API request per resource causes
slow refreshes and frequent `429 Too Many Requests` failures.

The provider reduces that load by:

- Sharing and coalescing list calls across concurrent resources
- Caching list results briefly and invalidating them after creates, updates, or deletes
- Pacing and retrying administrative API calls from OpenAI's rate-limit headers

It also adds project and organization spend limits and alerts, project model
permissions, and correct handling of rate-limit fields where `null` and `0` have
different meanings.

This is an independent fork of
[mkdev-me/terraform-provider-openai](https://github.com/mkdev-me/terraform-provider-openai).

## Quick start

Requirements: Terraform 1.0 or later and an OpenAI project API key.

```hcl
terraform {
  required_providers {
    openai = {
      source  = "rdsubhas/openai"
      version = "~> 3.0"
    }
  }
}

provider "openai" {}
```

Set credentials, then initialize Terraform:

```bash
export OPENAI_API_KEY="sk-proj-..."
terraform init
terraform plan
```

Keep credentials out of Terraform configuration, variable files, state, and version
control whenever possible.

## What it manages

- Model operations: responses, chat completions, embeddings, images, audio, moderation,
  batches, and fine-tuning
- Data and retrieval: files, uploads, vector stores, and vector-store files
- Administration: projects, users, groups, invitations, service accounts, and API keys
- Governance: roles, model permissions, rate limits, spend limits, and spend alerts

See the generated [resource](docs/resources/) and
[data-source](docs/data-sources/) references for the complete surface area.

## Authentication

The provider reads these environment variables:

| Variable | Purpose |
| --- | --- |
| `OPENAI_API_KEY` | Project-scoped model and data operations |
| `OPENAI_ADMIN_KEY` | Organization and project administration |
| `OPENAI_ORGANIZATION` | Optional organization ID |
| `OPENAI_API_URL` | Optional API endpoint override |
| `OPENAI_TIMEOUT` | Optional request timeout in seconds; defaults to `300` |

For mixed project and administrative resources, set both API keys. Equivalent provider
arguments are `api_key`, `admin_key`, `organization`, `api_url`, and `timeout`.

## Migrating from `mkdev-me/openai`

Terraform treats the fork as a different provider address. Back up your state, update
the address, change `required_providers`, and reinitialize:

```bash
terraform state replace-provider \
  registry.terraform.io/mkdev-me/openai \
  registry.terraform.io/rdsubhas/openai
terraform init -upgrade
terraform plan
```

Review the plan before applying. See [MIGRATION.md](MIGRATION.md) for the complete
procedure.

## Documentation

- [Terraform Registry documentation](https://registry.terraform.io/providers/rdsubhas/openai/latest/docs)
- [Resources](docs/resources/)
- [Data sources](docs/data-sources/)
- [Examples](examples/)
- [Reusable modules](modules/)

## Development

Development requires Go 1.25.8 or later and Terraform 1.0 or later.

```bash
git clone https://github.com/rdsubhas/terraform-provider-openai.git
cd terraform-provider-openai
go mod download
make build
make test
make lint
go generate ./...
```

Generated documentation must remain clean after `go generate ./...`. Acceptance tests
require valid OpenAI credentials and may create billable resources:

```bash
make testacc
```

See [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.

## Releases

Stable tags trigger signed GitHub releases that are ingested by the
`rdsubhas` HCP Terraform public namespace. Maintainers should follow
[RELEASING.md](RELEASING.md); release tags and assets are immutable.

## License and support

Licensed under the [Mozilla Public License 2.0](LICENSE).

Use [GitHub Issues](https://github.com/rdsubhas/terraform-provider-openai/issues) for
bugs and feature requests. Include a minimal Terraform configuration, provider version,
and relevant diagnostics with secrets removed.
