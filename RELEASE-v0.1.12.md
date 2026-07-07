# GSNode v0.1.12

## 回程路由显示对齐网页端

- IP 路径改用部分掩码（如 `8.218.*.*`），不再全部显示为 `*.*.*.*`
- 节点详情改为「节点 | 路线图 | 延迟」，显示 `本机 → [地区]线路 → …`
- 路线图优先使用 `path` 数据，与网页端 `routeMapHtml` 一致

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.12/install.sh | sh
```
