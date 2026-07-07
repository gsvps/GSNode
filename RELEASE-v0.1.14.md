# GSNode v0.1.14

## 修复 Disney+ / YouTube Premium 误报失败

- Disney+：修正 bamgrid token 请求体（latitude/longitude/platform）；GraphQL 改用 refreshToken 与正确鉴权；API 失败时回退首页检测
- YouTube Premium：添加 CONSENT Cookie；扩展解锁特征；页面可达时回退通用 HTTP 检测，减少误报「失败」

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.14/install.sh | sh
```
