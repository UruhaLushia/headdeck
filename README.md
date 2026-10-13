# Headdeck

English | [简体中文](README.zh-CN.md)

Manage [Headscale](https://headscale.net) from your terminal or browser.

- View service status and manage devices, users and authentication keys.
- Configure DNS, access rules, subnet routes and exit nodes.
- Edit Headscale settings and start, stop or restart the service.
- Optionally enable Web diagnostics to inspect Headscale runtime metrics and read-only debug snapshots.
- Manage accounts, Headscale user bindings and login providers.
- Sign in with a password, GitHub, OIDC or Passkey, with two-factor authentication support.

Available in English and Simplified Chinese. The Web UI supports light, dark and system themes; the terminal UI supports mouse and keyboard controls.

## Install

Install on a Linux host running systemd. Go and pnpm are required:

```sh
make build-web
sudo install -m 0755 hdkt /usr/local/bin/hdkt
sudo hdkt service install --admin-user "$(id -un)"
hdkt
```

Headdeck manages Headscale directly through its local Unix socket. No API key or manual connection settings are needed. Web and password login are disabled by default.

**Web automatically takes over Headscale OIDC and HTTP forwarding.** Configure the public HTTPS URL and login methods through CLI/TUI, and authorize [Headscale configuration management](docs/architecture.md#headscale-oidc-provider). GitHub and other OAuth/OIDC providers connect to Headdeck. Existing Headscale OIDC users must have explicit Headdeck account bindings before enabling Web.

Headdeck manages the Headscale OIDC client, secret, callback and public server URL automatically. Set `server.web.public_url` once; the internal upstream is read from Headscale's `listen_addr` and restricted to loopback. Headdeck and Headscale use separate internal listening ports.

Web lives at the domain root and proxies Headscale requests, so HTTPS only needs to forward to Headdeck. New accounts default to `pending` for owner approval. Set the default registration role to `member` for public deployments; Headscale users are created and bound when a Tailscale device first signs in.

For a terminal-only build, use `make build`.

## Use

Run `hdkt` to open the terminal UI. Open devices and users to view their details and actions, and edit settings on their pages.

Headdeck settings take effect when saved. Headscale configuration, DNS and access rules must be saved and then applied.

Authorize local users before they can connect:

```sh
hdkt auth add alice
hdkt auth remove alice
```

The owner manages Web accounts; newly registered accounts wait for approval. See `hdkt --help` for more commands.

## Development

```sh
make build-web
make check
```

## Documentation

[Requirements](docs/requirements.md) · [Architecture](docs/architecture.md) · [API](docs/api/README.md)

## License

[GNU AGPL-3.0](LICENSE).
