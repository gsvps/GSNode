# GSNode v0.1.3

## 新增清理脚本

新增 `cleanup.sh`，供用户在终端手动清理 GSNode 残留文件：

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.3/cleanup.sh | sh
```

支持 `-y` 跳过确认、`-n` 预览模式，以及 `GSNODE_DATA` / `GSNODE_BIN` 环境变量。
