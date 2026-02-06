
 一、核心规则

  1. 订单有效期 10 分钟，下单时锁定汇率
  2. 10 分钟内支付 → 按锁定汇率结算
  3. 超过 10 分钟仍支付 → 按“付款确认时”的最新汇率结算
  4. 下单时前端明确提示：“超过 10 分钟付款将按最新汇率结算 credit”

  ———

  二、汇率与金额计算

  假设：

  - creditPerDora = 1 DORA 能兑换多少 credit
  - 链上最小单位 peaka（18 位）

  下单时（锁定汇率）

  - amountDora = credit / creditPerDora_locked
  - amountPeaka = ceil(amountDora * 10^18)
  - expiresAt = now + 10min

  超时付款时（重新结算 credit）

  - 先计算付款实际 DORA：
    paidDora = paidPeaka / 10^18
  - 获取最新汇率 creditPerDora_latest
  - 发放信用：
    creditIssued = floor(paidDora * creditPerDora_latest)

  > 建议“向上取整金额、向下取整 credit”，避免少付或系统发超额 credit。

  ———

  三、订单状态机

  - created：订单已创建，未支付
  - paid：在有效期内支付，按锁定汇率发货
  - expired：超过有效期未支付
  - paid_late_repriced：超时后支付，按最新汇率发货
  - underpaid / overpaid（可选）：金额不匹配时标记


  ———

  四、接口调用流程

  1. POST /payments/orders

  - 入参：productId, quantity, credit, buyerAddress
  - 服务端：
      - 拉取 DORA 实时价格
      - 计算 creditPerDora
      - 生成派生地址
      - 计算 amountPeaka
      - 设定 expiresAt
  - 返回：
      - orderId
      - amountPeaka, denom=peaka
      - recipientAddress
      - expiresAt
      - rateSnapshot（建议返回给前端提示）

  2. 前端发起支付（Keplr 弹窗）

  - 转账到 recipientAddress
  - 手续费用户自理

  3. GET /payments/orders/:orderId

  - 返回订单状态与是否已发货
  - 如果已支付，返回 txHash, paidAt, creditIssued

  4. （可选）POST /payments/confirm

  - 前端主动上报 txHash
  - 后端立即校验与更新状态

  ———

  五、后端校验流程（简化版）

  - tx success == true
  - toAddress == recipientAddress
  - denom == peaka
  - amount == amountPeaka（如果容忍少付/多付，你们可扩展规则）
  - paidAt <= expiresAt → paid
  - paidAt > expiresAt → paid_late_repriced（重新算 credit）

  ———

  六、前端提示文案（建议）

  - “订单汇率锁定 10 分钟，超时付款将按最新汇率结算 credit。”
  - “请勿少付/多付，可能影响到账 credit。”

