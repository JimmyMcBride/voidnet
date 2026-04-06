# Voidnet Release and Installer Hosting

## Release Flow

1. Merge changes to `trunk`.
2. Create and push a version tag:

```sh
git tag v0.1.0
git push origin v0.1.0
```

3. GitHub Actions publishes these assets to the tagged release:
   - `voidnet-linux-amd64.tar.gz`
   - `voidnet-darwin-amd64.tar.gz`
   - `voidnet-darwin-arm64.tar.gz`
   - `voidnet-windows-amd64.zip`
   - `voidnet-checksums.txt`

The installers at `voidnet.jimmymcbride.dev` resolve to `releases/latest/download/...`, so they automatically pick up the newest tagged release.

## Coolify Static App

Create one static app in Coolify with these settings:

- Source: this GitHub repo
- Branch: `trunk`
- Build pack: `Static`
- Base directory: `deploy/install-site`
- Publish directory: `.`
- Build command: leave empty
- Domain: `voidnet.jimmymcbride.dev`
- Auto deploy: enabled

After the first deploy, verify:

- `https://voidnet.jimmymcbride.dev/`
- `https://voidnet.jimmymcbride.dev/install.sh`
- `https://voidnet.jimmymcbride.dev/install.ps1`

## Manual Smoke Checks

macOS / Linux:

```sh
curl -fsSL https://voidnet.jimmymcbride.dev/install.sh | sh
voidnet -version
```

Windows PowerShell:

```powershell
irm https://voidnet.jimmymcbride.dev/install.ps1 | iex
voidnet -version
```

Linux users need `libasound.so.2` available at runtime. The installer checks for it and prints distro-specific package guidance when it is missing.
