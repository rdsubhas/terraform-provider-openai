# Releasing

The `rdsubhas` public namespace and provider are managed in HCP Terraform. GitHub
Actions builds and signs each stable release, and the Terraform Cloud GitHub App
notifies HCP Terraform when the GitHub release is published.

## One-time signing setup

Generate a dedicated RSA key using `gpg --full-generate-key`. Choose RSA and RSA,
4096 bits, no expiration, a release-specific identity, and a strong unique passphrase.
Record the full fingerprint shown by:

```bash
gpg --list-secret-keys --keyid-format=long
TF_OPENAI_GPG_FPR=PASTE_FULL_FINGERPRINT_HERE
```

Register the public key in HCP Terraform under **Registry → Public namespaces →
rdsubhas → Settings → New GPG Key**:

```bash
gpg --armor --export "$TF_OPENAI_GPG_FPR" \
  > /tmp/terraform-provider-openai-public-key.asc
```

Store the encrypted private key directly in GitHub Actions without writing it to disk,
then enter the same key passphrase when prompted by the second command:

```bash
gpg --armor --export-secret-keys "$TF_OPENAI_GPG_FPR" |
  gh secret set GPG_PRIVATE_KEY --repo rdsubhas/terraform-provider-openai

gh secret set PASSPHRASE --repo rdsubhas/terraform-provider-openai
```

Verify the secret names with:

```bash
gh secret list --repo rdsubhas/terraform-provider-openai --app actions
```

Keep an encrypted offline backup of the private key and passphrase outside this
repository.

## Publish a release

1. Ensure `main` is green and contains all intended release changes.
2. In GitHub Actions, run **Bump Version** with a stable tag such as `v3.0.0`.
3. The tag triggers **Release**, which publishes signed archives and checksums.
4. Verify the release contains six platform ZIPs, the Registry manifest,
   `SHA256SUMS`, and `SHA256SUMS.sig`.
5. Verify the version under **HCP Terraform → Registry → Public namespaces →
   rdsubhas → openai**. Use **Resync** if ingestion does not start automatically.

Never replace assets for an existing release. Publish corrections as a new version.
