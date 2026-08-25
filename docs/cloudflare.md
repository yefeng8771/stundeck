# Cloudflare 配置

## API Token

StunDeck 不支持 Global API Key。创建 API Token 时，将资源限制到一个 Zone，并按实际功能授予：

| 权限 | 用途 |
| --- | --- |
| Zone > Zone > Read | 在连接向导中列出 Zone |
| Zone > DNS > Edit | 创建或更新入口与目标 DNS；不启用自动 DNS 时可省略 |
| Zone > Single Redirect > Edit | 创建和更新 Single Redirect |

资源范围必须选择 `Zone Resources > Include > Specific zone`，不要授权全部 Zone。Cloudflare 的首个 API Token 必须在 Dashboard 中由用户确认创建；StunDeck 不申请 `API Tokens Edit`，因为该权限可以创建具备任意资源权限的新 Token。

Cloudflare 页可一键打开[官方 API Token 管理页面](https://dash.cloudflare.com/profile/api-tokens)，并复制上述最小权限清单。Token 会先通过 `/user/tokens/verify` 检查状态，再列出可访问 Zone。Token 在保存前使用本地 AES-256-GCM 主密钥加密；API 和 Dashboard 后续不会返回完整值。

## Redirect 工作方式

入口域名必须是 Cloudflare Proxied DNS 记录。StunDeck 创建的规则：

- Phase：`http_request_dynamic_redirect`
- Action：`redirect`
- Expression：精确匹配入口 hostname
- Ref：`stundeck_<service-id>`
- Status：302 或 307

映射变化时，StunDeck 使用单规则 `PATCH`，不会替换完整 ruleset。

## DNS 所有权

自动 DNS 只会修改带有以下注释的记录：

```text
managed-by=stundeck:<service-id>
```

同名记录已存在但没有该标记时，同步会失败并要求人工处理。这是为了避免覆盖邮件、Tunnel、现有站点或其他自动化管理的 DNS。

## 302、307 与 303

第一版仅使用 Cloudflare Single Redirect 产品文档明确支持的 302 和 307：

- 302：普通浏览器 GET 页面。
- 307：需要保留请求方法的 API。

303 不依赖 Single Redirect API 的通用 schema。后续如需支持，将使用独立 Worker 返回 303，并采用独立、权限更窄的 Workers Token。
