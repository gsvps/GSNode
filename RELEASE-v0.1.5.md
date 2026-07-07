# GSNode v0.1.5

## Ping 目标库更新 (2026-07-07)

- 重新生成并验证全国 31 省三网 + 回程目标 IP（446 → 238 个有效目标）
- 剔除验证不可达的 IP，按延迟评分排序
- 全国延迟 failover 从 2 个增至 4 个备选 IP
- 客户端默认从 GitHub 拉取 `ping_targets.json`，无需等二进制发版即可更新目标库

```bash
curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.5/install.sh | sh
```

维护目标库：

```bash
cd GSProbe
go run ./cmd/gen-ping-json
go run ./cmd/target-collector -timeout 4s -workers 40
cp data/ping_targets.json ../GSNode/ping_targets.json
```
