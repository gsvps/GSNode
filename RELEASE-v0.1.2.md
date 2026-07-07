# GSNode v0.1.2

## 纯净一键检测

默认模式改为**临时运行 + 自动清理**：

- 检测程序下载到 `/tmp/gsnode.XXXXXX/`，不写入 `/usr/local/bin`
- 报告缓存同样放在临时目录
- 检测完成（含上传与终端摘要）后，自动删除二进制、报告目录及磁盘测试临时文件
- 服务器上不留任何 GSNode 安装痕迹

## 保留安装（可选）

若需要检测后保留程序，可设置：

```bash
GSNODE_KEEP=1 curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.2/install.sh | sh
```

## 用法

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.2/install.sh | sh
```
