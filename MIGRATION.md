# Migrating to `rdsubhas/openai`

Terraform treats `mkdev-me/openai` and `rdsubhas/openai` as different providers.
The v3 fork preserves the existing resource and data-source schemas, so migrate the
provider address in state rather than recreating managed resources.

1. Back up the current state using the safeguards appropriate for your backend.
2. In the initialized Terraform working directory, run:

   ```bash
   terraform state replace-provider \
     registry.terraform.io/mkdev-me/openai \
     registry.terraform.io/rdsubhas/openai
   ```

3. Update the provider requirement:

   ```hcl
   terraform {
     required_providers {
       openai = {
         source  = "rdsubhas/openai"
         version = "~> 3.0"
       }
     }
   }
   ```

4. Reinitialize and review the plan:

   ```bash
   terraform init -upgrade
   terraform plan
   ```

Do not apply if the plan proposes unexpected replacement of existing OpenAI resources.
