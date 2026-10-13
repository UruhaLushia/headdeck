# Headdeck

[English](README.md) | 简体中文

通过终端和 Web 管理 [Headscale](https://headscale.net)。

- 查看服务状态，管理设备、用户和认证密钥。
- 配置 DNS、访问控制、子网路由和出口节点。
- 管理 Headscale 配置，启动、停止或重启服务。
- 按需启用 Web 调试，查看 Headscale 运行指标和只读调试快照。
- 管理账户、Headscale 用户绑定和登录提供方。
- 支持密码、GitHub、OIDC、Passkey 和两步验证。

支持简体中文和英文，Web 支持深色、浅色和自动主题，终端支持鼠标和键盘操作。

## 安装

在使用 systemd 的 Linux 主机上安装，需要 Go 和 pnpm：

```sh
make build-web
sudo install -m 0755 hdkt /usr/local/bin/hdkt
sudo hdkt service install --admin-user "$(id -un)"
hdkt
```

Headdeck 通过本地 Unix 套接字直接管理 Headscale，无需 API 密钥或手动配置连接。Web 和密码登录默认关闭。

**启用 Web 后自动接管 Headscale OIDC 和 HTTP 转发。** 通过 CLI 或 TUI 配置公开 HTTPS 地址、登录方式，并授权 [Headscale 配置管理](docs/architecture.md#headscale-oidc-provider)。GitHub 和其他 OAuth/OIDC 提供方统一接入 Headdeck。已有 Headscale OIDC 用户需先显式绑定 Headdeck 账户。

Headscale OIDC 的客户端、密钥、回调和公开服务地址由 Headdeck 自动管理。只需配置一次 `server.web.public_url`，内部上游自动读取 Headscale 的 `listen_addr` 并限制到回环地址。Headdeck 与 Headscale 使用不同的内部监听端口。

Web 使用域名根路径并代理 Headscale 请求，HTTPS 只需转发到 Headdeck。新账户默认 `pending`，由 owner 审核；公开部署可将默认注册角色设为 `member`，在 Tailscale 设备首次登录时创建并绑定 Headscale 用户。

仅使用终端时，可以用 `make build` 构建。

## 使用

运行 `hdkt` 打开终端界面。设备和用户可直接打开详情，设置在对应页面编辑。

Headdeck 自身设置保存后自动生效；Headscale 配置、DNS 和访问控制需保存后应用。

本机用户需要授权才能连接：

```sh
hdkt auth add alice
hdkt auth remove alice
```

Web 账户由所有者管理，新注册账户等待授权。更多命令见 `hdkt --help`。

## 开发

```sh
make build-web
make check
```

## 文档

[需求](docs/requirements.md) · [架构设计](docs/architecture.md) · [API](docs/api/README.md)

## 许可证

[GNU AGPL-3.0](LICENSE)。
