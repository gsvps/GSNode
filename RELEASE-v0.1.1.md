# GSNode v0.1.1

## 新增

- **一键检测脚本** `install.sh`：安装 → 完整检测 → 上传 GSVPS → 终端显示报告链接
- **终端报告摘要**：`-run` 模式在终端输出模块评分与在线报告 URL
- **自动上传**：检测完成后上传至 `https://www.gsvps.com/api/reports/upload`

## 用法

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.1/install.sh | sh
```

## 变更

- 移除 `detect.sh`、`start.sh`，统一为单个 `install.sh`
- 新增 `-json` 参数用于脚本化输出完整 JSON
