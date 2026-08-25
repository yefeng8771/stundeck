# 飞牛 fnOS FPK 打包

StunDeck 的 FPK 是飞牛管理的 Docker 项目。应用镜像保持通用 Docker 运行方式；FPK 只负责把 fnOS 的安装向导、生命周期、桌面入口和持久化目录映射到同一组 `STUNDECK_*` 环境变量。

FPK 的 fnOS 应用 ID 使用 `stundeck-fpk`，与普通 Docker Compose 默认使用的 `stundeck` 项目名刻意分离。fnOS 在安装、升级失败回滚和卸载时会按应用 ID 清理 Docker 资源；独立 ID 可避免这些生命周期操作误删同时运行的普通模式容器。

## 设计与目录

```text
scripts/fpk/
├── build-fpk.sh
├── assets/icon.svg
└── template/
    ├── manifest
    ├── config/{privilege,resource}
    ├── cmd/*
    ├── wizard/{install,uninstall}
    └── app/
        ├── docker/docker-compose.yaml
        └── ui/{config,images/*}
```

- 容器继续使用 `network_mode: host`，让 NATMap、UPnP 和 NAT-PMP 看到 NAS 的真实网络栈。
- fnOS 的 `${TRIM_PKGVAR}` 映射到容器 `/var/lib/stundeck`，应用数据仍固定在 `${TRIM_PKGVAR}/data`。
- 数据库和主密钥分别固定为 `/var/lib/stundeck/data/stundeck.db` 与 `/var/lib/stundeck/data/master.key`，必须一起备份。
- fnOS 会在 Docker 项目创建前调用 `install_init`，但此时包用户还不能创建 `${TRIM_PKGETC}`/`${TRIM_PKGVAR}`。因此 Docker FPK 的生命周期脚本使用 root 完成这一项受限初始化：从 `20000-59999` 随机探测空闲 TCP 端口、创建包目录并立即把所有权交还 `${TRIM_UID}:${TRIM_GID}`。`${TRIM_PKGETC}/stundeck.env` 保存配置，`${TRIM_PKGETC}/stundeck.port` 保存同一端口供桌面 CGI 读取；升级和重启不会重新随机。
- `${TRIM_PKGETC}/stundeck.env` 保存安装生成的非机密运行配置，权限为 `0600`；端口记录为非秘密值，权限为 `0644`。
- root 只用于 fnOS 生命周期目录初始化；长期运行的容器始终显式使用 `${TRIM_UID}:${TRIM_GID}`，同时保持只读根文件系统、删除 capabilities 和 `no-new-privileges`。
- 不声明 `data-share`，避免数据库、会话和加密主密钥出现在普通文件共享中。
- fnOS 覆盖升级会调用旧包的卸载钩子，但不会提供卸载向导值；钩子在该场景安全地默认为保留数据。只有用户在正式卸载向导中明确选择“永久删除数据”时才会删除 `${TRIM_PKGVAR}/data`。
- 部分 fnOS 版本会通过 root 拥有的暂存区保留升级数据；`upgrade_callback` 会在重新启用服务前把 `${TRIM_PKGETC}` 和 `${TRIM_PKGVAR}` 递归归还给新的 `${TRIM_UID}:${TRIM_GID}`，避免 SQLite 或 `master.key` 因所有权漂移而不可写。

普通 Docker/二进制模式不受影响，仍可使用 `.env.example` 中的 `STUNDECK_*` 变量和任意宿主机数据目录。

## 前置条件

1. 从[飞牛开发者平台](https://developer.fnnas.com/docs/cli/fnpack)下载 `fnpack 1.2.3`，放进 `PATH`、项目根目录，或通过 `FNPACK_BIN` 指定。
2. 先把多架构镜像发布到 GHCR。标签构建推荐使用不可变的 `vX.Y.Z`，不要让正式 FPK 依赖 `latest`。
3. 目标 fnOS 设备能访问 GHCR，并且未同时运行名为 `stundeck-fpk` 的容器。

## 打包

发布版本时先创建并推送标签，仓库的 Container workflow 会发布同名的 amd64/arm64 镜像，并生成 FPK artifact：

```bash
git tag v0.1.0
git push origin v0.1.0
```

本地手动打包：

```bash
FNPACK_BIN=/absolute/path/to/fnpack-1.2.3-darwin-arm64 \
FPK_VERSION=0.1.0 \
FPK_IMAGE_TAG=v0.1.0 \
make fpk
```

默认镜像仓库是 `ghcr.io/nciae-zyh/stundeck`，默认版本来自 `web/package.json`，默认镜像标签为 `v<version>`。可用变量：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `FNPACK_BIN` | 自动探测 | 官方 `fnpack` 的绝对路径 |
| `FPK_VERSION` | `web/package.json` | 写入 manifest 的版本 |
| `FPK_IMAGE_REPOSITORY` | `ghcr.io/nciae-zyh/stundeck` | 镜像仓库 |
| `FPK_IMAGE_TAG` | `v<FPK_VERSION>` | 必须与已发布镜像标签完全一致 |
| `FPK_OUT_DIR` | `dist-fpk` | 输出目录 |
| `FPK_SKIP_IMAGE_CHECK` | `0` | 仅离线检查时设为 `1`；正式包不建议跳过 |

产物为 `dist-fpk/stundeck-fpk-<version>.fpk`。打包脚本会在 Docker Buildx 可用时先验证远端镜像，避免安装时才发现标签不存在。

## 安装与首次配置

1. 在 fnOS「应用中心 → 设置 → 手动安装应用」选择 `.fpk`。
2. 安装脚本自动选择空闲随机高位端口；安装向导只设置时区，以及是否启用 HTTPS 安全 Cookie。
3. 从桌面打开 StunDeck。第一次进入时创建管理员，并选择 `local`、`lan` 或 `public` 访问模式。
4. 登录后再在 StunDeck 中添加 Cloudflare API Token、服务与 Webhook。FPK 和安装向导不会保存这些秘密。

只有通过 HTTPS 反向代理访问时才启用安全 Cookie。桌面入口打开的是 NAS 的 HTTP 端口，启用后不能直接通过这个 HTTP 入口登录。

桌面入口使用 CGI 读取持久化端口，再由浏览器沿用当前 fnOS 页面实际使用的 NAS 主机名跳转。这样不会把 fnOS 内部 CGI 网关的 `localhost` 误当成客户端可访问地址，也能保留 IP、IPv6 或反向代理域名。

## 数据、升级和卸载

fnOS 模式的真实持久化目录是 `${TRIM_PKGVAR}/data`，其中至少包含：

- `stundeck.db`：管理员、服务、事件及加密后的凭据。
- `master.key`：解密 Cloudflare Token、Webhook Secret 和 TOTP Secret 所需的主密钥。

升级前同时备份这两个文件。升级脚本会拒绝在数据目录或配置缺失时继续。卸载向导默认保留数据；只有明确选择“永久删除数据”时，脚本才删除 `${TRIM_PKGVAR}/data` 这个包专属子目录。

## 验证与排障

```bash
# 静态展开 Compose（用临时值模拟 fnOS 环境）
TRIM_UID=1000 TRIM_GID=1000 \
TRIM_PKGETC=/tmp/stundeck-etc TRIM_PKGVAR=/tmp/stundeck-var \
docker compose -f scripts/fpk/template/app/docker/docker-compose.yaml config

# NAS 上查看状态与日志
docker inspect --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}}' stundeck-fpk
docker logs --tail 200 stundeck-fpk
```

- 容器无法拉取：确认 FPK 中的镜像标签已经发布，并确认 GHCR 包可公开读取。
- 页面打不开：读取 `${TRIM_PKGETC}/stundeck.port`，确认该端口仍由 `stundeck-fpk` 监听；若被安装后的其他服务抢占，停止冲突服务后重启 StunDeck。
- 数据库只读或无法创建：确认容器以 `${TRIM_UID}:${TRIM_GID}` 运行，且 `${TRIM_PKGVAR}/data` 属于 StunDeck 包用户。
- STUN 检测提示代理：不要把 Docker 守护进程的拉取代理传进运行容器；FPK 已主动清空常见代理变量。
- 外网无法回连：健康检查只证明控制面可用；仍需按 StunDeck 的诊断结果配置 UPnP/NAT-PMP、DMZ 或端口转发，并从独立外部网络验证。
