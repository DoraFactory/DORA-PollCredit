# DORA 原生代币支付服务（技术说明）

范围说明：
- 链：DORA（Cosmos SDK）
- chainId：`vota-testnet`
- rpcEndpoint：`https://vota-testnet-rpc.dorafactory.org/`
- denom：`peaka`
- 精度：`18`
- bech32 前缀：`dora`
- 支付方式：用户自签 `MsgSend` 转账（Keplr/Leap）
- 收款方式：**每订单独立收款地址**（HD 派生）
- 订单有效期：**10 分钟**
- **超时策略**：超时付款 **按最新汇率自动结算 credit**

---

## 1) 业务规则

### 1.1 汇率
- MVP：固定汇率（示例：`1 DORA = 100 credit`）。
- 上线：从 DORA price 接口获取实时价格。
- 下单时 **锁定汇率 10 分钟**。
- 超时付款时 **按确认时最新汇率结算**。
- 即使固定汇率也写入 `priceSnapshot`，方便后续动态汇率无缝切换。

### 1.2 购买限制
- 设置最小购买量：`credit >= minCredit`（MVP：`minCredit = 10000`）。

### 1.3 金额计算
设：
- `creditPerDora` = 1 DORA 可兑换的 credit 数
- `creditRequested` = 用户购买的 credit
- `decimals = 18`

**下单时（锁价）：**
- `amountDora = creditRequested / creditPerDora_locked`
- `amountPeaka = ceil(amountDora * 10^18)`

**超时付款（重新结算）：**
- `paidDora = paidPeaka / 10^18`
- `creditIssued = floor(paidDora * creditPerDora_latest)`

说明：
- 计算付款金额用 `ceil` 防止少付。
- 发放 credit 用 `floor` 防止超发。

### 1.4 手续费
- Gas 由用户支付，钱包自动估算。

---

## 2) 端到端流程

### 2.1 创建订单
**前端 -> API**
`POST /payments/orders`

请求参数：
- `credit`
说明：
- `userId` 从登录态/认证上下文获取，不在请求体中传递。
请求头：
- `X-User-Id`（MVP 临时方案，用于传递用户 ID）

服务端动作：
1. 调用 DORA price 接口
2. 计算 `creditPerDora`
3. 锁定汇率 10 分钟
4. 生成订单专属收款地址
5. 计算 `amountPeaka`
6. 落库订单与 `priceSnapshot`

返回：
- `orderId`
- `amountPeaka`
- `denom = peaka`
- `recipientAddress`
- `expiresAt`
- `priceSnapshot`

### 2.2 用户支付（钱包）
**前端（Keplr/Leap）**
- `sendTokens`：
  - `toAddress = recipientAddress`
  - `amount = amountPeaka` + `denom = peaka`
  - `memo` 可选
- Keplr 弹窗，用户签名。

### 2.3 后端确认
**后台 Worker**
- 实时：WS 订阅 `tm.event='Tx'`
- 回补：定时 `tx_search` 从 lastProcessedHeight 扫描
- 解析转账到 **订单收款地址** 的交易
- 校验并结算

### 2.4 前端查询
**前端 -> API**
`GET /payments/orders/:orderId`
- 轮询直到订单状态为 paid / paid_late_repriced

可选：
`POST /payments/confirm`
- 前端上报 `txHash` 加速确认

---

## 3) API 设计

### 3.1 创建订单
`POST /payments/orders`

请求：
- `credit`
说明：
- `userId` 从登录态/认证上下文获取，不在请求体中传递。
请求头：
- `X-User-Id`（MVP 临时方案，用于传递用户 ID）

响应：
- `orderId`
- `amountPeaka`
- `denom`
- `recipientAddress`
- `expiresAt`
- `priceSnapshot`

### 3.2 查询订单
`GET /payments/orders/:orderId`

响应：
- `status`
- `amountPeaka`
- `recipientAddress`
- `paidAt`（如已支付）
- `txHash`（如已支付）
- `creditIssued`（如已支付）

### 3.3 主动确认（可选）
`POST /payments/confirm`

请求：
- `orderId`
- `txHash`

响应：
- `status`

---

## 4) 状态机

建议状态：
- `created`：订单已创建，待支付
- `paid`：有效期内支付（锁价）
- `expired`：超时未支付
- `paid_late_repriced`：超时支付，按最新汇率结算
- `underpaid`（可选）
- `overpaid`（可选）

说明：
- 订单可从 `expired` 转为 `paid_late_repriced`（超时到账）。

---

## 5) 校验与结算逻辑

对每笔匹配到的交易：
1. `tx success`（code == 0）
2. `toAddress == order.recipientAddress`
3. `denom == peaka`
4. 解析 `paidPeaka`
5. 使用区块时间 `blockTime` 作为 `paidAt`

结算：
- 若 `paidAt <= expiresAt`：
  - `status = paid`
  - `creditIssued = creditRequested`
- 若 `paidAt > expiresAt`：
  - 拉取最新汇率
  - `status = paid_late_repriced`
  - `creditIssued = floor(paidDora * creditPerDora_latest)`

幂等：
- `txHash` 唯一索引，重复不重复发货。

---

## 6) 地址派生（每订单地址）

- Cosmos HD 路径：`m/44'/118'/0'/0/i`
- 订单保存 `derivationIndex`
- 服务端仅使用 **xpub** 派生地址（不暴露私钥）
- 私钥（xprv）离线或 HSM 保管

优点：
- 无需 memo
- 匹配简单

---

## 7) 监听与回补（防漏单）

### 7.1 实时监听（WS）
- 订阅 `tm.event='Tx'`
- 解析 transfer 事件
- `toAddress` 与订单地址匹配

### 7.2 回补扫描
- 持久化 `lastProcessedHeight`
- 每 N 秒：
  - `latestHeight = status()`
  - `from = lastProcessedHeight + 1`
  - `to = latestHeight - confirmDepth`
  - `tx_search` 扫描
- 处理所有匹配交易（幂等）

### 7.3 确认数
- 建议 2-5 个块确认后再结算

### 7.4 Archive Node
- Archive node 保证历史交易可查
- 必须开启 `tx_index`

---

## 8) 数据库（最小表）

### 8.1 orders
- `orderId`
- `userId`（来自登录态）
- `recipientAddress`
- `derivationIndex`
- `creditRequested`
- `amountPeaka`
- `denom`
- `priceSnapshot`
- `expiresAt`
- `status`
- `paidAt`
- `txHash`
- `creditIssued`

### 8.2 payments
- `txHash`（唯一）
- `orderId`
- `fromAddress`
- `toAddress`
- `amountPeaka`
- `denom`
- `height`
- `blockTime`

### 8.3 sync_state
- `lastProcessedHeight`

---

## 9) 运维要点

- 汇率接口需可靠并可缓存。
- 下单时校验 `minCredit`。
- 汇率接口失败时拒绝下单。
- 过期订单地址建议保留 30 天再归档。
- 资金归集（可选）：
  - 订单地址资金分散
  - 定期 sweep 到主钱包

---

## 10) 前端提示

- 下单页展示：
  - DORA 付款金额
  - 对应 credit
  - `expiresAt` 倒计时
  - 提示："订单有效期 10 分钟，超时付款按最新汇率结算 credit。"

- 支付后：
  - 展示“支付中”
  - 轮询 `GET /payments/orders/:orderId`
  - 成功后显示到账 credit

---

## 11) 配置项

- `CHAIN_ID = vota-testnet`
- `RPC_ENDPOINT = https://vota-testnet-rpc.dorafactory.org/`
- `DENOM = peaka`
- `DECIMALS = 18`
- `BECH32_PREFIX = dora`
- `ORDER_TTL = 10min`
- `CONFIRM_DEPTH = 2..5`
- `MIN_CREDIT = 10000`
- `FIXED_RATE = 1 DORA : 100 credit`（MVP）

---

## 12) 总结

- 用户通过向 **每订单独立地址** 转账完成支付。
- 订单锁价 10 分钟；超时付款自动按最新汇率结算。
- 监听采用 **WS + 回补**，确保不漏单。
- 结算逻辑具备幂等性。
