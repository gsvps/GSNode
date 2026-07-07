# GSNode v0.1.6

## 终端四列报告

检测完成后，终端报告与网页端一致，按四列并排显示：

| 系统报告 | IP 质量 | 网络质量 | 回程路由 |
|---------|---------|---------|---------|

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.6/install.sh | sh
```

可通过环境变量 `COLUMNS` 调整终端宽度（默认 136 字符）。
