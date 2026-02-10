const orderForm = document.getElementById("orderForm");
const apiBaseInput = document.getElementById("apiBase");
const userIdInput = document.getElementById("userId");
const creditInput = document.getElementById("credit");
const statusBox = document.getElementById("orderStatus");
const lastStatus = document.getElementById("lastStatus");

const MIN_CREDIT = 10000;

const savedBase = localStorage.getItem("dora_api_base");
if (savedBase) apiBaseInput.value = savedBase;
const savedUser = localStorage.getItem("dora_user_id");
if (savedUser) userIdInput.value = savedUser;

const params = new URLSearchParams(window.location.search);
const status = params.get("status");
const orderId = params.get("orderId");
const txHash = params.get("txHash");

if (status || orderId) {
  const items = [];
  if (status) items.push(`<div>状态</div><span>${status}</span>`);
  if (orderId) items.push(`<div>订单</div><span class="code">${orderId}</span>`);
  if (txHash) items.push(`<div>Tx Hash</div><span class="code">${txHash}</span>`);
  lastStatus.innerHTML = items.join("");
}

function setStatus(message, isError = false) {
  statusBox.textContent = message;
  statusBox.hidden = false;
  statusBox.classList.toggle("error", isError);
}

orderForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  statusBox.hidden = true;

  const apiBase = apiBaseInput.value.trim().replace(/\/$/, "");
  const userId = userIdInput.value.trim();
  const credit = Number(creditInput.value);

  if (!apiBase) return setStatus("请填写 API Base。", true);
  if (!userId) return setStatus("请填写 User ID。", true);
  if (!Number.isFinite(credit) || credit < MIN_CREDIT) {
    return setStatus(`最小购买为 ${MIN_CREDIT} credit。`, true);
  }

  localStorage.setItem("dora_api_base", apiBase);
  localStorage.setItem("dora_user_id", userId);

  try {
    setStatus("正在创建订单...");
    const res = await fetch(`${apiBase}/payments/orders`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-User-Id": userId,
      },
      body: JSON.stringify({ credit }),
    });

    if (!res.ok) {
      const text = await res.text();
      throw new Error(text || "创建订单失败");
    }

    const data = await res.json();
    localStorage.setItem("dora_last_order_id", data.orderId);
    const url = new URL("./checkout.html", window.location.href);
    url.searchParams.set("orderId", data.orderId);
    url.searchParams.set("apiBase", apiBase);
    url.searchParams.set("userId", userId);
    window.location.href = url.toString();
  } catch (err) {
    setStatus(err.message || "创建订单失败", true);
  }
});
