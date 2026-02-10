const orderDetails = document.getElementById("orderDetails");
const txDetails = document.getElementById("txDetails");
const payBtn = document.getElementById("payBtn");
const checkBtn = document.getElementById("checkBtn");
const backBtn = document.getElementById("backBtn");
const statusBox = document.getElementById("statusBox");

const params = new URLSearchParams(window.location.search);
const orderId = params.get("orderId") || localStorage.getItem("dora_last_order_id");
const apiBase = (params.get("apiBase") || localStorage.getItem("dora_api_base") || "").replace(/\/$/, "");
const userId = params.get("userId") || localStorage.getItem("dora_user_id") || "";

const chainConfig = {
  chainId: "vota-testnet",
  rpcEndpoint: "https://vota-testnet-rpc.dorafactory.org/",
  restEndpoint: "https://vota-testnet-rest.dorafactory.org",
  chainName: "DORA Vota Testnet",
  denom: "peaka",
  coinDenom: "DORA",
  decimals: 18,
  gasPrice: "0.025peaka",
  gasPriceStep: { low: 10000000000, average: 10000000000, high: 10000000000 },
  bech32Prefix: "dora",
  coinType: 118,
};

if (!orderId || !apiBase) {
  orderDetails.innerHTML = "缺少 orderId 或 apiBase，请返回重新创建订单。";
  payBtn.disabled = true;
  checkBtn.disabled = true;
  backBtn.disabled = false;
}

let currentOrder = null;
let pollingTimer = null;
let stargateLib = null;

function setStatus(message, isError = false) {
  statusBox.textContent = message;
  statusBox.hidden = false;
  statusBox.classList.toggle("error", isError);
}

function formatAmountBase(amountBase, decimals, maxFraction = 6) {
  try {
    const base = BigInt(amountBase);
    const factor = 10n ** BigInt(decimals);
    const whole = base / factor;
    const fraction = base % factor;
    if (fraction === 0n) return `${whole}`;
    let fractionStr = fraction.toString().padStart(decimals, "0");
    if (maxFraction < decimals) fractionStr = fractionStr.slice(0, maxFraction);
    fractionStr = fractionStr.replace(/0+$/, "");
    return fractionStr ? `${whole}.${fractionStr}` : `${whole}`;
  } catch (err) {
    return amountBase;
  }
}

function renderOrder(order) {
  const amountDisplay = formatAmountBase(order.amountPeaka, chainConfig.decimals);
  const denom = order.denom || chainConfig.denom;
  const expiresAt = order.expiresAt ? new Date(order.expiresAt).toLocaleString() : "未知";
  const detailHtml = [
    `<div>订单 ID</div><span class="code">${orderId}</span>`,
    `<div>支付地址</div><span class="code">${order.recipientAddress}</span>`,
    `<div>支付金额</div><span>${amountDisplay} DORA</span>`,
    `<div>Base Amount</div><span class="code">${order.amountPeaka} ${denom}</span>`,
    `<div>状态</div><span>${order.status}</span>`,
    `<div>过期时间</div><span>${expiresAt}</span>`,
  ];
  orderDetails.innerHTML = detailHtml.join("");

  if (order.txHash) {
    txDetails.innerHTML = [
      `<div>Tx Hash</div><span class="code">${order.txHash}</span>`,
      order.paidAt ? `<div>确认时间</div><span>${new Date(order.paidAt).toLocaleString()}</span>` : "",
      order.creditIssued ? `<div>Credit 发放</div><span>${order.creditIssued}</span>` : "",
    ].join("");
  }

  if (order.status === "paid" || order.status === "paid_late_repriced") {
    payBtn.disabled = true;
    setStatus("支付已完成，将自动返回购买页。", false);
    setTimeout(() => {
      const url = new URL("./index.html", window.location.href);
      url.searchParams.set("status", order.status);
      url.searchParams.set("orderId", orderId);
      if (order.txHash) url.searchParams.set("txHash", order.txHash);
      window.location.href = url.toString();
    }, 2000);
  }
}

async function fetchOrder() {
  if (!orderId) return;
  const res = await fetch(`${apiBase}/payments/orders/${orderId}`);
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || "获取订单失败");
  }
  const data = await res.json();
  currentOrder = data;
  renderOrder(data);
  return data;
}

async function loadStargate() {
  if (stargateLib) return stargateLib;
  const sources = [
    "https://esm.sh/@cosmjs/stargate@0.32.4?bundle",
    "https://esm.run/@cosmjs/stargate@0.32.4",
  ];
  let lastErr = null;
  for (const src of sources) {
    try {
      const mod = await import(src);
      stargateLib = { SigningStargateClient: mod.SigningStargateClient, GasPrice: mod.GasPrice };
      return stargateLib;
    } catch (err) {
      lastErr = err;
    }
  }
  throw new Error(`CosmJS 加载失败：${lastErr?.message || "未知错误"}`);
}

async function ensureKeplr() {
  if (!window.keplr) {
    throw new Error("未检测到 Keplr，请安装浏览器插件。 ");
  }
  try {
    await window.keplr.enable(chainConfig.chainId);
  } catch (err) {
    if (!window.keplr.experimentalSuggestChain) {
      throw new Error("Keplr 不支持自动添加链，请手动添加该测试网。");
    }
    await window.keplr.experimentalSuggestChain({
      chainId: chainConfig.chainId,
      chainName: chainConfig.chainName,
      rpc: chainConfig.rpcEndpoint,
      rest: chainConfig.restEndpoint,
      bip44: { coinType: chainConfig.coinType },
      bech32Config: {
        bech32PrefixAccAddr: chainConfig.bech32Prefix,
        bech32PrefixAccPub: `${chainConfig.bech32Prefix}pub`,
        bech32PrefixValAddr: `${chainConfig.bech32Prefix}valoper`,
        bech32PrefixValPub: `${chainConfig.bech32Prefix}valoperpub`,
        bech32PrefixConsAddr: `${chainConfig.bech32Prefix}valcons`,
        bech32PrefixConsPub: `${chainConfig.bech32Prefix}valconspub`,
      },
      currencies: [
        {
          coinDenom: chainConfig.coinDenom,
          coinMinimalDenom: chainConfig.denom,
          coinDecimals: chainConfig.decimals,
        },
      ],
      stakeCurrency: {
        coinDenom: chainConfig.coinDenom,
        coinMinimalDenom: chainConfig.denom,
        coinDecimals: chainConfig.decimals,
      },
      feeCurrencies: [
        {
          coinDenom: chainConfig.coinDenom,
          coinMinimalDenom: chainConfig.denom,
          coinDecimals: chainConfig.decimals,
          gasPriceStep: chainConfig.gasPriceStep,
        },
      ],
      features: ["stargate"],
    });
    await window.keplr.enable(chainConfig.chainId);
  }
  const signer = window.getOfflineSigner
    ? window.getOfflineSigner(chainConfig.chainId)
    : window.keplr.getOfflineSigner(chainConfig.chainId);
  return signer;
}

async function sendPayment() {
  if (!currentOrder) {
    setStatus("订单尚未加载，请稍后重试。", true);
    return;
  }
  try {
    payBtn.disabled = true;
    setStatus("正在连接 Keplr 并发送交易...");

    const signer = await ensureKeplr();
    const accounts = await signer.getAccounts();
    if (!accounts.length) throw new Error("未获取到钱包地址。");

    const { SigningStargateClient, GasPrice } = await loadStargate();
    const gasPrice = GasPrice.fromString(chainConfig.gasPrice);
    const client = await SigningStargateClient.connectWithSigner(
      chainConfig.rpcEndpoint,
      signer,
      { gasPrice }
    );

    const amount = [{ denom: chainConfig.denom, amount: currentOrder.amountPeaka }];
    const memo = `order:${orderId}`;
    const result = await client.sendTokens(
      accounts[0].address,
      currentOrder.recipientAddress,
      amount,
      "auto",
      memo
    );

    if (result.code !== 0) {
      throw new Error(result.rawLog || "交易失败");
    }

    txDetails.innerHTML = `<div>Tx Hash</div><span class="code">${result.transactionHash}</span>`;
    setStatus("交易已广播，等待确认中...");
    startPolling();
  } catch (err) {
    setStatus(err.message || "支付失败", true);
    payBtn.disabled = false;
  }
}

function startPolling() {
  if (!orderId || !apiBase) return;
  if (pollingTimer) clearInterval(pollingTimer);
  pollingTimer = setInterval(async () => {
    try {
      const order = await fetchOrder();
      if (!order) return;
      if (order.status === "paid" || order.status === "paid_late_repriced") {
        clearInterval(pollingTimer);
      }
    } catch (err) {
      setStatus(err.message || "查询失败", true);
    }
  }, 4000);
}

checkBtn.addEventListener("click", async () => {
  try {
    await fetchOrder();
    setStatus("已刷新订单状态。", false);
  } catch (err) {
    setStatus(err.message || "查询失败", true);
  }
});

backBtn.addEventListener("click", () => {
  const url = new URL("./index.html", window.location.href);
  if (orderId) url.searchParams.set("orderId", orderId);
  window.location.href = url.toString();
});

payBtn.addEventListener("click", sendPayment);

(async () => {
  try {
    if (!orderId || !apiBase) return;
    payBtn.disabled = true;
    await fetchOrder();
    payBtn.disabled = false;
    startPolling();
  } catch (err) {
    setStatus(err.message || "加载订单失败", true);
  }
})();
