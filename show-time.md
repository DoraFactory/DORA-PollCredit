# MVP 演示流程（Show Time）

本文用于快速演示当前 DORA Poll Credit 的 MVP 购买与支付闭环。

## 0. 前置条件

- 已安装 Keplr 浏览器插件
- Keplr 中已存在 `vota-testnet`（如未内置，会在支付页提示添加）
- 本地可访问 Postgres（或已启动 Docker）
- 已配置 `configs/config.yaml`（含 XPub、RPC、Denom、Prefix 等）

## 1. 启动后端

1) 启动数据库（如本地无 Postgres）

```bash
docker run --name dora-pay-db -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=dora_pay -p 5432:5432 -d postgres:16
```

2) 初始化数据库表

```bash
go run ./cmd/migrate
```

3) 启动 API

```bash
go run ./cmd/api
```

4) 启动 Worker（扫描与 WS 监听）

```bash
go run ./cmd/worker
```

## 2. 启动前端演示页面

```bash
npx serve payment-app
```

默认地址：`http://localhost:3000`

## 3. 演示购买流程

1) 打开购买页（`payment-app/index.html`）
2) 填写：
   - API Base: `http://localhost:8080`
   - User ID: 任意（如 `user_demo`）
   - Credit 数量（最小 10000）
3) 点击“创建订单并前往支付”
4) 跳转支付页，展示订单信息
5) 点击“连接钱包并支付”
6) Keplr 弹窗确认交易 → 发送
7) 支付页轮询订单状态，确认后自动返回购买页
8) 购买页显示订单状态与 TxHash

## 4. 观测与验证

- Worker 日志：可看到区块扫描与 pending 订单数
- API：

```bash
curl http://localhost:8080/payments/orders/<orderId>
```

可观察订单状态变为 `paid` 或 `paid_late_repriced`。

## 5. 常见问题

- **页面提示 CORS**：确认已重启 API（CORS 已开启）
- **支付页按钮不可点击**：检查 URL 是否带 `orderId` 与 `apiBase`
- **Keplr 未弹出**：确认已安装插件，或浏览器未禁用弹窗

---

如需扩展演示（真实价格、信用发放、管理后台），可按 roadmap 继续推进。
