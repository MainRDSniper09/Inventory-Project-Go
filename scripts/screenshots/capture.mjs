// Recorre la UI como lo haría un usuario y toma capturas para el README.
// También funciona como prueba de humo: termina con código 1 si hay errores
// de JavaScript o si algún flujo no produce el resultado esperado.
//
// Uso (ver run.sh): node capture.mjs <base-url> <directorio-salida>
import { chromium } from "playwright";
import { strict as assert } from "node:assert";

const [base = "http://localhost:8080", out = "."] = process.argv.slice(2);
const jsErrors = [];
const browser = await chromium.launch();

async function newPage(opts = {}) {
  const ctx = await browser.newContext({
    viewport: { width: 1280, height: 860 },
    deviceScaleFactor: 1,
    locale: "es-ES",
    timezoneId: "UTC",
    colorScheme: "light",
    ...opts,
  });
  const page = await ctx.newPage();
  page.on("pageerror", (e) => jsErrors.push(`pageerror: ${e.message}`));
  page.on("console", (m) => {
    // Los 409 son esperados: el script provoca a propósito un SKU duplicado y
    // una salida mayor al stock para verificar que la UI muestra el error.
    if (m.type() === "error" && !/status of 409/.test(m.text())) jsErrors.push(`console: ${m.text()}`);
  });
  return page;
}

const step = (msg) => console.log("•", msg);
const shot = async (page, name, opts = {}) => {
  await page.screenshot({ path: `${out}/${name}.png`, ...opts });
  console.log("  📸", `${name}.png`);
};
const waitToast = async (page, text) => {
  await page.locator("#toast:not([hidden])", { hasText: text }).waitFor();
};
const toastGone = (page) => page.locator("#toast").waitFor({ state: "hidden", timeout: 6000 });
// La CSP de la app prohíbe eval, así que no usamos page.waitForFunction:
// sondeamos con locators (que no requieren eval).
async function waitCount(locator, predicate, timeout = 5000) {
  const until = Date.now() + timeout;
  for (;;) {
    const n = await locator.count();
    if (predicate(n)) return n;
    if (Date.now() > until) throw new Error(`timeout esperando conteo; último valor: ${n}`);
    await new Promise((r) => setTimeout(r, 100));
  }
}
const optionValue = (page, select, text) =>
  select.evaluate((s, t) => [...s.options].find((o) => o.textContent.includes(t))?.value, text);

const page = await newPage();

// ------------------------------------------------------------ dashboard
step("Dashboard carga KPIs, gráfica y tablas");
await page.goto(base);
await page.locator("#kpis .kpi").first().waitFor();
await page.locator("#daily-chart .bar").first().waitFor();
assert.equal(await page.locator("#kpis .kpi").count(), 7);
assert.ok((await page.locator("#daily-chart .bar").count()) >= 10, "la gráfica debe tener historial");
assert.ok((await page.locator("#low-body tr").count()) >= 1, "debe haber productos con stock bajo");

// ------------------------------------------------------------ categorías / productos
step("Crear categoría");
await page.click('.tab[data-view="products"]');
await page.locator("#products-body tr").first().waitFor();
await page.click("#new-category");
await page.fill('#category-form [name="name"]', "Lácteos");
await page.fill('#category-form [name="description"]', "Leche, quesos y yogures");
await page.click('#category-form button:has-text("Guardar")');
await waitToast(page, "Categoría creada");

step("Crear producto con stock inicial");
await page.click("#new-product");
const pf = page.locator("#product-form");
await pf.locator('[name="sku"]').fill("lac-leche-1l");
await pf.locator('[name="name"]').fill("Leche entera 1L");
await pf.locator('[name="description"]').fill("Ultrapasteurizada, caja de 1 litro");
await pf.locator('[name="category_id"]').selectOption({ label: "Lácteos" });
await pf.locator('[name="price"]').fill("24.50");
await pf.locator('[name="initial_stock"]').fill("36");
await shot(page, "product-dialog");
await pf.locator('button:has-text("Guardar")').click();
await waitToast(page, "Producto creado");
const leche = page.locator("#products-body tr", { hasText: "LAC-LECHE-1L" });
await leche.waitFor();
assert.match(await leche.innerText(), /Lácteos[\s\S]*24[.,]50[\s\S]*36/);

step("Validación: SKU duplicado muestra error en el formulario");
await page.click("#new-product");
await pf.locator('[name="sku"]').fill("LAC-LECHE-1L");
await pf.locator('[name="name"]').fill("Duplicado");
await pf.locator('[name="price"]').fill("1");
await pf.locator('button:has-text("Guardar")').click();
await page.locator("#product-error", { hasText: "already exists" }).waitFor();
await pf.locator('button:has-text("Cancelar")').click();

step("Editar precio");
await leche.locator('button:has-text("Editar")').click();
assert.ok(await pf.locator('[name="sku"]').isDisabled(), "el SKU debe ser inmutable");
await pf.locator('[name="price"]').fill("25.90");
await pf.locator('button:has-text("Guardar")').click();
await waitToast(page, "Producto actualizado");
await page.locator("#products-body tr", { hasText: "25,90" }).or(page.locator("#products-body tr", { hasText: "25.90" })).first().waitFor();

step("Buscar");
await page.fill("#product-search", "leche");
await waitCount(page.locator("#products-body tr"), (n) => n === 1);
await page.fill("#product-search", "");
await waitCount(page.locator("#products-body tr"), (n) => n > 5);
await toastGone(page);
await shot(page, "products", { fullPage: true });

// ------------------------------------------------------------ inventario
step("Entrada de mercancía y kardex");
await leche.locator('button:has-text("Stock")').click();
const mf = page.locator("#movement-form");
await mf.locator('[name="type"]').selectOption("IN");
await mf.locator('[name="quantity"]').fill("12");
await mf.locator('[name="reason"]').fill("Compra a proveedor");
await mf.locator('button:has-text("Registrar")').click();
await waitToast(page, "Movimiento registrado");
await leche.locator("td.num", { hasText: /^48$/ }).waitFor(); // 36 + 12

step("Validación: salida mayor al stock es rechazada");
await leche.locator('button:has-text("Stock")').click();
await mf.locator('[name="type"]').selectOption("OUT");
await mf.locator('[name="quantity"]').fill("999");
await mf.locator('button:has-text("Registrar")').click();
await page.locator("#movement-error", { hasText: "insufficient stock" }).waitFor();

step("Ajuste por conteo físico");
await mf.locator('[name="type"]').selectOption("ADJUST");
await mf.locator('[name="quantity"]').fill("-2");
await mf.locator('[name="reason"]').fill("Conteo físico mensual");
await page.evaluate(() => (document.querySelector("#movement-error").textContent = ""));
await shot(page, "inventory-movement");
await mf.locator('button:has-text("Registrar")').click();
await waitToast(page, "Movimiento registrado");
await leche.locator("td.num", { hasText: /^46$/ }).waitFor(); // 48 - 2

// ------------------------------------------------------------ ventas
step("Registrar venta con dos líneas");
await page.click('.tab[data-view="sales"]');
await page.locator("#sales-body tr").first().waitFor();
const line1 = page.locator(".sale-line").first().locator("select");
await line1.selectOption(await optionValue(page, line1, "LAC-LECHE-1L"));
await page.locator(".sale-line").first().locator('[name="qty"]').fill("2");
await page.click("#add-line");
const line2 = page.locator(".sale-line").nth(1).locator("select");
await line2.selectOption(await optionValue(page, line2, "BEB-COLA-600"));
await page.locator(".sale-line").nth(1).locator('[name="qty"]').fill("3");
await page.fill('#sale-form [name="notes"]', "Venta mostrador");
// 2 × 25.90 + 3 × 18.00 = 105.80
await page.locator("#sale-total", { hasText: /105[.,]80/ }).waitFor();
await toastGone(page);
await shot(page, "sales");
await page.click('#sale-form button:has-text("Registrar")');
await waitToast(page, "registrada");
await page.locator("#sales-body tr", { hasText: "Venta mostrador" }).waitFor();

// ------------------------------------------------------------ reportes
step("Reporte de ventas y CSV");
await page.click('.tab[data-view="reports"]');
await page.locator("#report-body tr").first().waitFor();
assert.ok((await page.locator("#report-body tr").count()) > 20);
const href = await page.locator("#report-csv").getAttribute("href");
const csv = await (await page.request.get(base + href)).text();
assert.ok(csv.startsWith("sale_id,sold_at,sku"), "el CSV debe tener cabecera");
assert.ok(csv.includes("LAC-LECHE-1L"), "el CSV debe incluir la venta recién registrada");
await toastGone(page);
await shot(page, "reports");

// ------------------------------------------------------------ dashboard final
step("Dashboard refleja la nueva venta (claro, oscuro y móvil)");
await page.click('.tab[data-view="dashboard"]');
await page.locator("#top-body tr").first().waitFor();
await page.locator("#daily-chart .bar").first().waitFor();
await shot(page, "dashboard", { fullPage: true });

const dark = await newPage({ colorScheme: "dark" });
await dark.goto(base);
await dark.locator("#daily-chart .bar").first().waitFor();
await shot(dark, "dashboard-dark", { fullPage: true });

const mobile = await newPage({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true });
await mobile.goto(base);
await mobile.locator("#daily-chart .bar").first().waitFor();
await shot(mobile, "dashboard-mobile");

await browser.close();

if (jsErrors.length) {
  console.error("\n❌ Errores de JavaScript en la UI:\n" + jsErrors.join("\n"));
  process.exit(1);
}
console.log("\n✅ Todos los flujos de la UI funcionaron sin errores.");
