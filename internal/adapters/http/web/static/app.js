// Cliente web mínimo: consume la API REST /api/v1. Sin frameworks ni CDN.
// Todo el contenido dinámico se inserta con textContent (previene XSS).
"use strict";

const $ = (sel, root = document) => root.querySelector(sel);
// useGrouping "always": en español, por defecto no se agrupan los números de 4 dígitos (9337,60).
const money = (cents) =>
  (cents / 100).toLocaleString("es", { minimumFractionDigits: 2, maximumFractionDigits: 2, useGrouping: "always" });
const isoDay = (d) => d.toISOString().slice(0, 10);

let products = [];
let categories = [];

// ------------------------------------------------------------------ API
async function api(path, options = {}) {
  const res = await fetch("/api/v1" + path, {
    headers: { "Content-Type": "application/json" },
    ...options,
    body: options.body ? JSON.stringify(options.body) : undefined,
  });
  if (res.status === 204) return null;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    const e = data.error || {};
    throw new Error(e.field ? `${e.field}: ${e.message}` : e.message || res.statusText);
  }
  return data;
}

function toast(msg, isError = false) {
  const t = $("#toast");
  t.textContent = msg;
  t.className = "toast" + (isError ? " error" : "");
  t.hidden = false;
  clearTimeout(toast._t);
  toast._t = setTimeout(() => (t.hidden = true), 3500);
}

// Crea un elemento con hijos/texto de forma segura.
function el(tag, attrs = {}, ...children) {
  const n = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (k === "class") n.className = v;
    else if (k.startsWith("on")) n.addEventListener(k.slice(2), v);
    else n.setAttribute(k, v);
  }
  for (const c of children) n.append(c instanceof Node ? c : document.createTextNode(String(c ?? "")));
  return n;
}

function fillBody(tbody, rows, cols) {
  tbody.replaceChildren();
  if (!rows.length) {
    tbody.append(el("tr", {}, el("td", { colspan: cols, class: "empty" }, "Sin datos")));
    return;
  }
  rows.forEach((r) => tbody.append(r));
}

// ------------------------------------------------------------------ navegación
function show(view) {
  document.querySelectorAll(".tab").forEach((b) => b.classList.toggle("active", b.dataset.view === view));
  document.querySelectorAll(".view").forEach((v) => (v.hidden = v.id !== "view-" + view));
  ({ dashboard: loadDashboard, products: loadProducts, sales: loadSales, reports: loadReport })[view]?.();
}

// ------------------------------------------------------------------ dashboard
async function loadDashboard() {
  const f = $("#dash-filter");
  const qs = new URLSearchParams();
  if (f.from.value) qs.set("from", f.from.value);
  if (f.to.value) qs.set("to", f.to.value);
  try {
    const d = await api("/dashboard?" + qs);
    const s = d.summary;
    const kpis = [
      ["Ventas", s.sales_count],
      ["Unidades vendidas", s.units_sold],
      ["Ingresos", "$" + money(s.revenue_cents)],
      ["Productos activos", s.active_products],
      ["Unidades en stock", s.units_in_stock],
      ["Valor inventario", "$" + money(s.inventory_value_cents)],
      ["Stock bajo", s.low_stock_count, s.low_stock_count > 0 ? "warn" : ""],
    ];
    $("#kpis").replaceChildren(...kpis.map(([l, v, c]) =>
      el("div", { class: "kpi " + (c || "") }, el("div", { class: "label" }, l), el("div", { class: "value" }, v))));

    drawDaily(d.daily);
    fillBody($("#top-body"), d.top_products.map((p) =>
      el("tr", {}, el("td", {}, p.sku), el("td", {}, p.name), el("td", { class: "num" }, p.units_sold),
        el("td", { class: "num" }, "$" + money(p.revenue_cents)))), 4);
    $("#low-threshold").textContent = `(≤ ${s.low_stock_threshold} unidades)`;
    fillBody($("#low-body"), d.low_stock.map((p) =>
      el("tr", {}, el("td", {}, p.sku), el("td", {}, p.name),
        el("td", { class: "num" }, el("span", { class: "badge low" }, p.quantity)))), 3);
  } catch (e) { toast(e.message, true); }
}

function drawDaily(days) {
  const chart = $("#daily-chart");
  chart.replaceChildren();
  if (!days.length) { chart.append(el("p", { class: "empty" }, "Sin ventas en el periodo")); return; }
  const max = Math.max(...days.map((d) => d.revenue_cents), 1);
  for (const d of days) {
    const bar = el("div", { class: "bar", title: `${d.day}: $${money(d.revenue_cents)} · ${d.units_sold} unid.` });
    bar.style.height = Math.max(2, (d.revenue_cents / max) * 100) + "%";
    chart.append(el("div", { class: "bar-wrap" }, bar, el("span", { class: "bar-label" }, d.day.slice(5))));
  }
}

// ------------------------------------------------------------------ productos
async function loadCategories() {
  categories = await api("/categories");
}

async function loadProducts() {
  try {
    const q = $("#product-search").value.trim();
    [products] = await Promise.all([api("/products?limit=500&q=" + encodeURIComponent(q)), loadCategories()]);
    fillBody($("#products-body"), products.map((p) => el("tr", {},
      el("td", {}, p.sku), el("td", {}, p.name), el("td", {}, p.category_name || "—"),
      el("td", { class: "num" }, "$" + money(p.price_cents)),
      el("td", { class: "num" }, p.quantity),
      el("td", { class: "actions" },
        el("button", { class: "btn small ghost", onclick: () => openMovement(p) }, "Stock"), " ",
        el("button", { class: "btn small ghost", onclick: () => openProduct(p) }, "Editar"), " ",
        el("button", { class: "btn small danger", onclick: () => archiveProduct(p) }, "Archivar")))), 6);
  } catch (e) { toast(e.message, true); }
}

function openProduct(p = null) {
  const f = $("#product-form");
  f.reset();
  $("#product-error").textContent = "";
  $("#product-dialog-title").textContent = p ? "Editar producto" : "Nuevo producto";
  const sel = f.category_id;
  sel.replaceChildren(el("option", { value: "" }, "— Sin categoría —"),
    ...categories.map((c) => el("option", { value: c.id }, c.name)));
  f.id.value = p?.id ?? "";
  f.sku.value = p?.sku ?? "";
  f.sku.disabled = !!p; // el SKU es inmutable
  f.name.value = p?.name ?? "";
  f.description.value = p?.description ?? "";
  sel.value = p?.category_id ?? "";
  f.price.value = p ? (p.price_cents / 100).toFixed(2) : "";
  $("#initial-stock-field").hidden = !!p;
  $("#product-dialog").showModal();
}

async function saveProduct(ev) {
  ev.preventDefault();
  const f = ev.target;
  const body = {
    name: f.name.value,
    description: f.description.value,
    category_id: f.category_id.value ? Number(f.category_id.value) : null,
    price_cents: Math.round(Number(f.price.value) * 100),
  };
  try {
    if (f.id.value) {
      await api("/products/" + f.id.value, { method: "PUT", body });
      toast("Producto actualizado");
    } else {
      await api("/products", { method: "POST", body: { ...body, sku: f.sku.value, initial_stock: Number(f.initial_stock.value || 0) } });
      toast("Producto creado");
    }
    $("#product-dialog").close();
    loadProducts();
  } catch (e) { $("#product-error").textContent = e.message; }
}

async function archiveProduct(p) {
  // confirm() no está disponible en todos los entornos embebidos; pedimos doble clic.
  if (archiveProduct._pending !== p.id) {
    archiveProduct._pending = p.id;
    toast(`Pulsa "Archivar" otra vez para confirmar ${p.sku}`);
    setTimeout(() => (archiveProduct._pending = null), 4000);
    return;
  }
  archiveProduct._pending = null;
  try {
    await api("/products/" + p.id, { method: "DELETE" });
    toast("Producto archivado");
    loadProducts();
  } catch (e) { toast(e.message, true); }
}

async function openMovement(p) {
  const f = $("#movement-form");
  f.reset();
  f.product_id.value = p.id;
  $("#movement-error").textContent = "";
  $("#movement-product").textContent = `${p.sku} · ${p.name} — existencia actual: ${p.quantity}`;
  await loadMovements(p.id);
  $("#movement-dialog").showModal();
}

async function loadMovements(id) {
  const ms = await api(`/products/${id}/movements?limit=20`);
  fillBody($("#movements-body"), ms.map((m) => el("tr", {},
    el("td", {}, new Date(m.created_at).toLocaleString()), el("td", {}, el("span", { class: "badge" }, m.type)),
    el("td", { class: "num" }, (m.delta > 0 ? "+" : "") + m.delta), el("td", {}, m.reason))), 4);
}

async function saveMovement(ev) {
  ev.preventDefault();
  const f = ev.target;
  try {
    await api(`/products/${f.product_id.value}/movements`, {
      method: "POST",
      body: { type: f.type.value, quantity: Number(f.quantity.value), reason: f.reason.value },
    });
    toast("Movimiento registrado");
    $("#movement-dialog").close();
    loadProducts();
  } catch (e) { $("#movement-error").textContent = e.message; }
}

async function saveCategory(ev) {
  ev.preventDefault();
  const f = ev.target;
  try {
    await api("/categories", { method: "POST", body: { name: f.name.value, description: f.description.value } });
    toast("Categoría creada");
    $("#category-dialog").close();
    loadCategories();
  } catch (e) { $("#category-error").textContent = e.message; }
}

// ------------------------------------------------------------------ ventas
function addSaleLine() {
  const sel = el("select", { name: "product", required: "" }, el("option", { value: "" }, "Producto…"),
    ...products.filter((p) => p.quantity > 0).map((p) =>
      el("option", { value: p.id }, `${p.sku} · ${p.name} ($${money(p.price_cents)}, ${p.quantity} disp.)`)));
  const qty = el("input", { name: "qty", type: "number", min: "1", step: "1", value: "1", required: "" });
  const line = el("div", { class: "sale-line" }, sel, qty,
    el("button", { type: "button", class: "btn small danger", onclick: () => { line.remove(); updateTotal(); } }, "✕"));
  sel.addEventListener("change", updateTotal);
  qty.addEventListener("input", updateTotal);
  $("#sale-lines").append(line);
}

function saleItems() {
  return [...document.querySelectorAll(".sale-line")]
    .map((l) => ({ product_id: Number($("[name=product]", l).value), quantity: Number($("[name=qty]", l).value) }))
    .filter((i) => i.product_id && i.quantity > 0);
}

function updateTotal() {
  const total = saleItems().reduce((acc, i) => acc + (products.find((p) => p.id === i.product_id)?.price_cents || 0) * i.quantity, 0);
  $("#sale-total").textContent = money(total);
}

async function loadSales() {
  try {
    products = await api("/products?limit=500");
    if (!$("#sale-lines").children.length) addSaleLine();
    const sales = await api("/sales?limit=50");
    fillBody($("#sales-body"), sales.map((s) => el("tr", {},
      el("td", {}, s.id), el("td", {}, new Date(s.created_at).toLocaleString()),
      el("td", { class: "num" }, s.units_sold), el("td", { class: "num" }, "$" + money(s.total_cents)), el("td", {}, s.notes))), 5);
  } catch (e) { toast(e.message, true); }
}

async function registerSale(ev) {
  ev.preventDefault();
  const items = saleItems();
  if (!items.length) { toast("Agrega al menos un producto", true); return; }
  try {
    const s = await api("/sales", { method: "POST", body: { notes: ev.target.notes.value, items } });
    toast(`Venta #${s.id} registrada: $${money(s.total_cents)}`);
    ev.target.reset();
    $("#sale-lines").replaceChildren();
    loadSales();
  } catch (e) { toast(e.message, true); }
}

// ------------------------------------------------------------------ reportes
async function loadReport(ev) {
  ev?.preventDefault();
  const f = $("#report-filter");
  const qs = new URLSearchParams({ from: f.from.value, to: f.to.value });
  $("#report-csv").href = "/api/v1/reports/sales?format=csv&" + qs;
  try {
    const lines = await api("/reports/sales?" + qs);
    fillBody($("#report-body"), lines.map((l) => el("tr", {},
      el("td", {}, l.sale_id), el("td", {}, new Date(l.sold_at).toLocaleString()), el("td", {}, l.sku), el("td", {}, l.product),
      el("td", { class: "num" }, l.quantity), el("td", { class: "num" }, "$" + money(l.unit_price_cents)),
      el("td", { class: "num" }, "$" + money(l.subtotal_cents)))), 7);
    $("#report-units").textContent = lines.reduce((a, l) => a + l.quantity, 0);
    $("#report-total").textContent = "$" + money(lines.reduce((a, l) => a + l.subtotal_cents, 0));
  } catch (e) { toast(e.message, true); }
}

// ------------------------------------------------------------------ init
document.addEventListener("DOMContentLoaded", () => {
  const today = new Date();
  const monthAgo = new Date(Date.now() - 29 * 864e5);
  for (const f of [$("#dash-filter"), $("#report-filter")]) {
    f.from.value = isoDay(monthAgo);
    f.to.value = isoDay(today);
  }

  document.querySelectorAll(".tab").forEach((b) => b.addEventListener("click", () => show(b.dataset.view)));
  document.querySelectorAll("[data-close]").forEach((b) => b.addEventListener("click", () => b.closest("dialog").close()));

  $("#dash-filter").addEventListener("submit", (e) => { e.preventDefault(); loadDashboard(); });
  $("#report-filter").addEventListener("submit", loadReport);
  $("#product-form").addEventListener("submit", saveProduct);
  $("#movement-form").addEventListener("submit", saveMovement);
  $("#category-form").addEventListener("submit", saveCategory);
  $("#sale-form").addEventListener("submit", registerSale);
  $("#add-line").addEventListener("click", addSaleLine);
  $("#new-product").addEventListener("click", async () => { await loadCategories(); openProduct(); });
  $("#new-category").addEventListener("click", () => { $("#category-form").reset(); $("#category-error").textContent = ""; $("#category-dialog").showModal(); });

  let searchTimer;
  $("#product-search").addEventListener("input", () => { clearTimeout(searchTimer); searchTimer = setTimeout(loadProducts, 250); });

  show("dashboard");
});
