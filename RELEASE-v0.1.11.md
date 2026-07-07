# GSNode v0.1.11

## 终端报告数据补全

- 报告改为四分类全宽输出，表格不再被列宽截断
- 补全 IP 详情：ISP、坐标、时区、地区、用途分类、风险评分、邮件/黑名单
- 补全系统详情：BIOS、芯片组、网卡、CPU 规格/缓存/指令集、Swap、磁盘容量、显卡、磁盘 profile
- 补全网络：国际节点延迟；回程：完整线路与 IP 路径
- 流媒体服务名称与状态完整显示（不再截断为 YouTube…）

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.11/install.sh | sh
```
