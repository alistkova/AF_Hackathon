import fs from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { execSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
const puppeteer = require("puppeteer-core");

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const OUT = process.env.OUT_DIR || path.join(ROOT, "docs", "screenshots");
const BASE = process.env.BASE_URL || "http://localhost:3000";

function findChrome() {
  if (process.env.CHROME_PATH) return process.env.CHROME_PATH;
  for (const name of ["google-chrome", "google-chrome-stable", "chromium", "chromium-browser"]) {
    try {
      return execSync(`command -v ${name}`, { encoding: "utf8" }).trim();
    } catch {
      continue;
    }
  }
  throw new Error("Chrome not found: set CHROME_PATH");
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function waitText(page, text, timeout = 30000) {
  await page.waitForFunction(
    (value) => document.body.innerText.toLowerCase().includes(value.toLowerCase()),
    { timeout },
    text,
  );
}

async function clickByText(page, text) {
  const handle = await page.evaluateHandle((value) => {
    const nodes = Array.from(document.querySelectorAll("button, a"));
    return nodes.find((node) => node.textContent?.trim().startsWith(value)) || null;
  }, text);
  const element = handle.asElement();
  if (!element) throw new Error(`element not found: ${text}`);
  await element.evaluate((node) => node.scrollIntoView({ block: "center" }));
  await sleep(200);
  await element.click();
}

fs.mkdirSync(OUT, { recursive: true });

const browser = await puppeteer.launch({
  executablePath: findChrome(),
  headless: "new",
  args: ["--no-sandbox", "--disable-dev-shm-usage", "--lang=ru-RU"],
  defaultViewport: { width: 1440, height: 900, deviceScaleFactor: 1 },
});

try {
  const page = await browser.newPage();
  await page.goto(BASE, { waitUntil: "networkidle2" });
  await waitText(page, "Новостная аналитика");
  await sleep(300);
  await page.screenshot({ path: path.join(OUT, "01-login.png") });
  console.log("01-login");

  await clickByText(page, "Войти");
  await waitText(page, "Лента событий", 30000);
  await page.waitForFunction(() => document.querySelectorAll("article").length > 0, {
    timeout: 30000,
  });
  await sleep(600);
  await page.screenshot({ path: path.join(OUT, "02-feed.png") });
  console.log("02-feed");

  const filtersCard = await page.evaluateHandle(() => {
    const input = document.querySelector('input[aria-label="Поиск по ленте"]');
    return input ? input.closest(".rounded-2xl") : null;
  });
  const filtersEl = filtersCard.asElement();
  if (filtersEl) {
    await filtersEl.screenshot({
      path: path.join(OUT, "03-filters.png"),
      captureBeyondViewport: true,
    });
    console.log("03-filters");
  }

  await clickByText(page, "Разобрать событие");
  await page.waitForFunction(() => document.body.innerText.includes("Уверенность"), {
    timeout: 90000,
  });
  await sleep(1500);
  const article = await page.evaluateHandle(() => {
    const nodes = Array.from(document.querySelectorAll("article"));
    return nodes.find((node) => node.innerText.includes("Разбор события")) || null;
  });
  const articleEl = article.asElement();
  if (!articleEl) throw new Error("analysis article not found");
  await articleEl.screenshot({
    path: path.join(OUT, "04-analysis.png"),
    captureBeyondViewport: true,
  });
  console.log("04-analysis");

  await page.goto(`${BASE}/#/profile`, { waitUntil: "networkidle2" });
  await waitText(page, "Мои компании", 30000);
  await page.evaluate(() => window.scrollTo(0, 0));
  await sleep(600);
  await page.screenshot({ path: path.join(OUT, "05-profile.png") });
  console.log("05-profile");

  await page.goto(`${BASE}/#/companies/magnit/members`, { waitUntil: "networkidle2" });
  await waitText(page, "Участники", 30000);
  await page.evaluate(() => window.scrollTo(0, 0));
  await sleep(600);
  await page.screenshot({ path: path.join(OUT, "06-members.png") });
  console.log("06-members");
} finally {
  await browser.close();
}
