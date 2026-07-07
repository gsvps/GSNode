# GSNode v0.1.7

## 终端体验优化

- 检测报告改为分块表格输出，延迟/状态/线路着色，与网页端对齐
- 检测进度改为单行展示：转圈动画 → 完成后显示 ✓ 与中文描述
- 检测过程不再刷屏输出子步骤详情

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.7/install.sh | sh
```

可通过 `NO_COLOR=1` 关闭终端颜色。
