# Releasing

Releases are built by [`.github/workflows/release.yml`](.github/workflows/release.yml). Installed apps update to a release once it's published.

## Cut a release

1. Make sure `main` is clean and CI passes:

   ```bash
   CGO_ENABLED=0 go test ./...
   ```

2. Add a `## <version>` section at the top of [`CHANGELOG.md`](CHANGELOG.md), in the style of the ones below it. It becomes the release notes. Leave out test-only changes.

3. Bump `version` in [`mygo.json`](mygo.json), and the download links in both READMEs.

4. Commit, tag and push:

   ```bash
   git commit -am "chore(release): 0.0.2"
   git tag v0.0.2
   git push origin main v0.0.2
   ```

   The tag must match `mygo.json`, or the workflow stops. Instead of pushing a tag you can run `gh workflow run release.yml`, which tags the version in `mygo.json` itself.

5. Watch it with `gh run watch`. The workflow runs CI, builds macOS, Windows and Linux, checks that all six targets' packages and update manifests are there, and only then uploads them and publishes the release as the latest.

## Secrets

| Secret | Used for |
| --- | --- |
| `MYGO_UPDATER_PRIVATE_KEY` | Signing the updates; its public key is `updates.publicKey` in `mygo.json` |
| `MACOS_CERTIFICATE`, `MACOS_CERTIFICATE_PASSWORD` | The Developer ID Application certificate (base64 `.p12`) that signs the macOS app |
| `APPLE_ID`, `APPLE_TEAM_ID`, `APPLE_APP_PASSWORD` | Notarizing the macOS disk images; the password is an app-specific one from [account.apple.com](https://account.apple.com) |
| `WINDOWS_CERTIFICATE`, `WINDOWS_CERTIFICATE_PASSWORD` | The code signing certificate (base64 `.pfx`) for the Windows apps and installers |

Each platform signs once its secrets are set; until then the workflow warns and builds as before, signed ad hoc on macOS and unsigned on Windows. The Developer ID needs the [Apple Developer Program](https://developer.apple.com/programs/); export it with its private key from Keychain Access.

```bash
base64 -i certificate.p12 | gh secret set MACOS_CERTIFICATE
gh secret set MACOS_CERTIFICATE_PASSWORD
gh secret set APPLE_ID
gh secret set APPLE_TEAM_ID
gh secret set APPLE_APP_PASSWORD
base64 -i code-signing.pfx | gh secret set WINDOWS_CERTIFICATE
gh secret set WINDOWS_CERTIFICATE_PASSWORD
```
