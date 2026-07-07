# GSNode v0.1.10

## 修复终端显示

- 检测进度：整页重绘，修复 ○/✓ 重复行；进度百分比单调递增不再回退
- 报告布局：改为 2×2 四列（每列约 66 字符宽），表格不再被截断错位
- 修复 ANSI 颜色下 padRight 宽度计算错误导致的列对齐混乱

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.10/install.sh | sh
```
