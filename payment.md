# 独立支付页设计（Payment WebApp）

本方案将 Web3 支付 UI 从主站剥离，单独部署一个支付页（Payment WebApp），主站只负责下单和跳转。这样主站保持纯 Web2，降低集成成本与风险。

## 1) 架构角色

- 主站（Web2）
  - 创建订单
  - 跳转支付页
  - 查询结果
- 支付页（独立 WebApp）
  - 连接钱包（Keplr）
  - 发起转账
  - 展示支付状态
- 支付服务（后端）
  - 订单创建、校验
  - 链上监听、结算

## 2) 推荐流程（端到端）

### Step 1：主站创建订单
`POST /payments/orders`

请求：
- `credit`

响应：
- `orderId`
- `amountPeaka`
- `recipientAddress`
- `expiresAt`
- `priceSnapshot`

### Step 2：主站跳转支付页
跳转到支付页（域名待定）：

```
https://<PAYMENT_APP_DOMAIN>/checkout?orderId=<orderId>
```

说明：
- `PAYMENT_APP_DOMAIN` 暂定（后续确定）。
- 仅传 `orderId`，不传用户敏感信息。

### Step 3：支付页加载订单详情
支付页调用：
`GET /payments/orders/:orderId`

展示信息：
- 支付金额（DORA/peaka）
- 收款地址
- `expiresAt` 倒计时
- 汇率提示（超时重定价）

### Step 4：用户连接钱包并转账
支付页集成 Keplr，调用 `sendTokens` 发起转账：
- `toAddress = recipientAddress`
- `amount = amountPeaka` (denom=peaka)

### Step 5：支付页轮询订单状态
支付页每 3–5 秒调用：
`GET /payments/orders/:orderId`

状态：
- `paid` → 支付成功
- `paid_late_repriced` → 超时成功（按最新汇率结算）
- `expired` → 超时未支付
- `underpaid/overpaid` → 异常订单

### Step 6：支付成功回跳主站
完成支付后跳回主站结果页：

```
https://<MAIN_APP_DOMAIN>/payment/result?orderId=<orderId>&status=paid
```

主站再次调用 `GET /payments/orders/:orderId` 确认状态。

## 3) 支付页路由建议

- `/checkout?orderId=...`
- `/success?orderId=...`
- `/failed?orderId=...`

## 4) 体验与安全建议

- 支付页只依赖 `orderId`，不暴露用户信息
- 明确提示：订单有效期 10 分钟，超时按最新汇率结算
- 提供复制地址/金额、一键复制按钮
- 可选：二维码展示

## 5) 待定项

- 支付页域名：`<PAYMENT_APP_DOMAIN>`
- 主站域名：`<MAIN_APP_DOMAIN>`
- 是否自动回跳主站，或由用户点击按钮返回

