# GSNode v0.1.9

## 四列报告与检测进度

- 终端报告恢复四列并排布局，每列内含表格（系统 / IP 质量 / 网络质量 / 回程路由）
- 检测顺序调整为：系统 → CPU → 内存 → 磁盘 → IP 质量 → 流媒体 & AI → 网络 → 全国延迟 → 回程路由
- 检测开始时预先列出全部 9 项，执行到对应项时原地更新进度条，完成后显示 ✓

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.9/install.sh | sh
```
