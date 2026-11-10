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
