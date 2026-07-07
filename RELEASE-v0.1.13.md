# GSNode v0.1.13

## 流媒体解锁检测优化

- Netflix：双片源探测，区分完整解锁 / 仅自制剧 / 封禁，并提取地区码
- Disney+：bamgrid API 三步检测（devices → token → GraphQL）
- YouTube / ChatGPT / Gemini / DAZN：平台专用 API 与特征码检测
- ChatGPT 支持「仅网页 / 仅 APP」部分可用状态
- 其余服务增强 403/451 判定与地区提取

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.13/install.sh | sh
```
