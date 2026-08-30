const { chromium } = require('playwright');
const fs = require('fs');
const path = require('path');

const [baseURL, testRoot] = process.argv.slice(2);
const outputDir = path.join(testRoot, 'screenshots');
fs.mkdirSync(outputDir, { recursive: true });
const password = 'Release#120A';

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

async function noPageOverflow(page, label) {
  const dimensions = await page.evaluate(() => ({
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
  }));
  assert(dimensions.scrollWidth <= dimensions.clientWidth, `${label} overflow: ${JSON.stringify(dimensions)}`);
}

(async () => {
  const launchOptions = { headless: true };
  if (process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH) {
    launchOptions.executablePath = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH;
  }
  const browser = await chromium.launch(launchOptions);
  const pageErrors = [];
  const desktop = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
  const page = await desktop.newPage();
  page.on('pageerror', error => pageErrors.push(error.message));

  await page.goto(baseURL, { waitUntil: 'networkidle' });
  await page.locator('#auth-title').waitFor({ state: 'visible' });
  assert(await page.locator('#auth-title').textContent() === '设置管理员密码', 'new install did not show setup');
  await noPageOverflow(page, 'desktop setup');
  await page.screenshot({ path: path.join(outputDir, 'desktop-setup.png'), fullPage: true });

  await page.locator('#password-input').fill(password);
  await page.locator('#setup-confirm-password').fill(password);
  await page.locator('#setup-submit').click();
  await page.locator('#app-shell').waitFor({ state: 'visible' });
  await page.locator('.nav-item[data-page="schedules"]').click();
  assert((await page.locator('#stat-scheduler').textContent()).trim() === '运行中', 'dashboard scheduler status was not running');
  await noPageOverflow(page, 'desktop schedules');
  await page.screenshot({ path: path.join(outputDir, 'desktop-schedules.png'), fullPage: true });

  const storage = await page.evaluate(() => ({ local: { ...localStorage }, session: { ...sessionStorage }, cookie: document.cookie }));
  assert(Object.keys(storage.session).length === 0, `sessionStorage was not empty: ${JSON.stringify(storage.session)}`);
  assert(!Object.values(storage.local).some(value => String(value).includes(password)), 'password leaked to localStorage');
  assert(storage.cookie === '', `authentication cookie was visible to JavaScript: ${storage.cookie}`);
  const cookies = await desktop.cookies(baseURL);
  const sessionCookie = cookies.find(cookie => cookie.name === 'ncmm_session');
  assert(sessionCookie && sessionCookie.httpOnly && sessionCookie.sameSite === 'Strict', `session cookie flags invalid: ${JSON.stringify(sessionCookie)}`);

  await page.route('**/api/v1/update', route => route.fulfill({ status: 500, contentType: 'application/json', body: '{"error":"injected optional failure"}' }));
  await page.reload({ waitUntil: 'networkidle' });
  await page.locator('#app-shell').waitFor({ state: 'visible' });
  await page.locator('.toast.error').filter({ hasText: '版本信息加载失败' }).waitFor({ state: 'visible' });
  await page.unroute('**/api/v1/update');

  const notifyPath = path.join(testRoot, 'notify.yaml');
  const validNotify = fs.readFileSync(notifyPath, 'utf8');
  fs.writeFileSync(notifyPath, 'webhook: [invalid\n', 'utf8');
  await page.reload({ waitUntil: 'networkidle' });
  await page.locator('#app-shell').waitFor({ state: 'visible' });
  await page.locator('.nav-item[data-page="config"]').click();
  await page.locator('#config-target [data-target="notify"]').click();
  await page.locator('#config-parse-error').waitFor({ state: 'visible' });
  assert(await page.locator('#config-yaml').isVisible(), 'invalid notify YAML did not switch to source mode');
  assert((await page.locator('#yaml-editor').inputValue()).includes('webhook: [invalid'), 'invalid raw YAML was not returned');
  await page.locator('#yaml-editor').fill(validNotify);
  await page.locator('#config-save').click();
  await page.locator('.toast').filter({ hasText: '配置已保存' }).waitFor({ state: 'visible' });
  assert(await page.locator('#config-parse-error').isHidden(), 'parse error remained after repair');

  const failedCore = await browser.newContext({ viewport: { width: 1000, height: 800 } });
  const failedPage = await failedCore.newPage();
  await failedPage.goto(baseURL, { waitUntil: 'networkidle' });
  await failedPage.route('**/api/v1/schedules', route => route.fulfill({ status: 500, contentType: 'application/json', body: '{"error":"injected core failure"}' }));
  await failedPage.locator('#password-input').fill(password);
  await failedPage.locator('#login-submit').click();
  await failedPage.locator('#auth-error').waitFor({ state: 'visible' });
  assert(await failedPage.locator('#app-shell').isHidden(), 'core failure left an empty app shell visible');
  assert((await failedPage.locator('#auth-error-message').textContent()).includes('控制台初始化失败'), 'core failure was not visible on login view');
  await failedCore.close();

  const mobile = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true });
  const mobilePage = await mobile.newPage();
  mobilePage.on('pageerror', error => pageErrors.push(error.message));
  await mobilePage.goto(baseURL, { waitUntil: 'networkidle' });
  await mobilePage.locator('#password-input').fill(password);
  await noPageOverflow(mobilePage, 'mobile login');
  await mobilePage.screenshot({ path: path.join(outputDir, 'mobile-login.png'), fullPage: true });
  await mobilePage.locator('#login-submit').click();
  await mobilePage.locator('#app-shell').waitFor({ state: 'visible' });
  await noPageOverflow(mobilePage, 'mobile dashboard');
  await mobilePage.locator('#menu-button').click();
  await mobilePage.locator('.nav-item[data-page="schedules"]').click();
  await mobilePage.waitForTimeout(300);
  await noPageOverflow(mobilePage, 'mobile schedules');
  await mobilePage.screenshot({ path: path.join(outputDir, 'mobile-schedules.png'), fullPage: true });
  await mobile.close();

  assert(pageErrors.length === 0, `browser page errors: ${JSON.stringify(pageErrors)}`);
  await desktop.close();
  await browser.close();
  process.stdout.write(JSON.stringify({ ok: true, outputDir, cookies: cookies.map(cookie => ({ name: cookie.name, httpOnly: cookie.httpOnly, sameSite: cookie.sameSite })), pageErrors }));
})().catch(error => {
  process.stderr.write(error.stack || String(error));
  process.exit(1);
});
