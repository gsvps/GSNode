# GSNode v0.1.4

## 终端体验优化

- **美化检测日志**：去除时间戳/ID 前缀，按模块分组显示进度，不再输出测试目标 IP
- **完整结果输出**：终端展示各模块全部指标（CPU/内存/磁盘/网络/回程/流媒体/全国延迟等）
- **上传失败兜底**：即使 GSVPS 上传失败，终端仍会显示完整检测结果

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.4/install.sh | sh
```
