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

async function hasHorizontalScroll(locator, label) {
  const dimensions = await locator.evaluate(element => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
    overflowX: getComputedStyle(element).overflowX,
  }));
  assert(dimensions.scrollWidth > dimensions.clientWidth && ['auto', 'scroll'].includes(dimensions.overflowX), `${label} horizontal scrollbar unavailable: ${JSON.stringify(dimensions)}`);
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

  const now = new Date().toISOString();
  const earlierToday = new Date(Date.now() - 60 * 60 * 1000).toISOString();
  const yesterday = new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString();
  const dayBeforeYesterday = new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString();
  const mockRuns = [{
    id: 'run-dashboard-metrics', jobId: 'job-dashboard-metrics', jobName: '每日任务', command: 'task',
    status: 'success', source: 'manual', triggeredAt: now, startedAt: now, finishedAt: now, exitCode: 0,
    rewards: [
      { account: './cookie.json', uid: '10001', nickname: '测试主账号', cookieKnown: true, cookieValid: true, vipKnown: true, vip: true, yunbei: 12, yunbeiKnown: true, yunbeiCumulative: true, yunbeiBalance: 345, yunbeiBalanceKnown: true, growthToday: 8, growthTotal: 99, growthKnown: true, effectivePlays: 35, effectiveTarget: 100, effectiveKnown: true, signed: true, signKnown: true },
      { account: './fan1.json', uid: '10002', nickname: '测试辅助账号', cookieKnown: true, cookieValid: true, vipKnown: true, vip: false, yunbei: 5, yunbeiKnown: true, yunbeiCumulative: true, yunbeiBalance: 66, yunbeiBalanceKnown: true, signed: true, signKnown: true },
    ],
  }, {
    id: 'run-dashboard-metrics-earlier', jobId: 'job-dashboard-metrics-earlier', jobName: '今日较早任务', command: 'sign',
    status: 'success', source: 'scheduler', triggeredAt: earlierToday, startedAt: earlierToday, finishedAt: earlierToday, exitCode: 0,
    rewards: [
      { account: './cookie.json', musicianKnown: true, musician: true, yunbei: 8, yunbeiKnown: true, yunbeiCumulative: true, yunbeiBalance: 340, yunbeiBalanceKnown: true, growthToday: 5, growthTotal: 96, growthKnown: true },
      { account: './fan1.json', musicianKnown: true, musician: false, yunbei: 4, yunbeiKnown: true, yunbeiCumulative: true, yunbeiBalance: 65, yunbeiBalanceKnown: true, vipKnown: true, vip: false },
    ],
  }, {
    id: 'run-dashboard-history', jobId: 'job-dashboard-history', jobName: '昨日任务', command: 'task',
    status: 'success', source: 'scheduler', triggeredAt: yesterday, startedAt: yesterday, finishedAt: yesterday, exitCode: 0,
    rewards: [
      { account: './cookie.json', uid: '10001', nickname: '测试主账号', cookieKnown: true, cookieValid: true, vipKnown: true, vip: true, yunbei: 7, yunbeiKnown: true, yunbeiCumulative: true, yunbeiBalance: 333, yunbeiBalanceKnown: true, growthToday: 5, growthTotal: 91, growthKnown: true, effectivePlays: 22, effectiveTarget: 90, effectiveKnown: true, signed: true, signKnown: true },
      { account: './fan1.json', uid: '10002', nickname: '测试辅助账号', cookieKnown: true, cookieValid: true, vipKnown: true, vip: false, yunbei: 3, yunbeiKnown: true, yunbeiCumulative: true, yunbeiBalance: 61, yunbeiBalanceKnown: true, signed: true, signKnown: true },
    ],
  }, {
    id: 'run-dashboard-earlier-history', jobId: 'job-dashboard-earlier-history', jobName: '前日任务', command: 'task',
    status: 'success', source: 'scheduler', triggeredAt: dayBeforeYesterday, startedAt: dayBeforeYesterday, finishedAt: dayBeforeYesterday, exitCode: 0,
    rewards: [
      { account: './cookie.json', uid: '10001', nickname: '测试主账号', cookieKnown: true, cookieValid: true, vipKnown: true, vip: true, yunbei: 10, yunbeiKnown: true, yunbeiCumulative: true, yunbeiBalance: 326, yunbeiBalanceKnown: true, growthToday: 7, growthTotal: 86, growthKnown: true, effectivePlays: 30, effectiveTarget: 90, effectiveKnown: true, signed: true, signKnown: true },
      { account: './fan1.json', uid: '10002', nickname: '测试辅助账号', cookieKnown: true, cookieValid: true, vipKnown: true, vip: false, yunbei: 4, yunbeiKnown: true, yunbeiCumulative: true, yunbeiBalance: 58, yunbeiBalanceKnown: true, signed: true, signKnown: true },
    ],
  }];
  await page.route('**/api/v1/runs', route => route.request().method() === 'GET'
    ? route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(mockRuns) })
    : route.continue());
  const officialPlayDates = Array.from({ length: 7 }, (_, index) => {
    const date = new Date();
    date.setHours(0, 0, 0, 0);
    date.setDate(date.getDate() - 7 + index);
    const pad = number => String(number).padStart(2, '0');
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
  });
  const officialPlayStats = [
    { account: './cookie.json', points: [90, 192, 100, 156, 147, 120, 265].map((count, index) => ({ date: officialPlayDates[index], count, known: true, source: index === 6 ? 'songs/daily' : 'trend/get' })) },
    { account: './fan1.json', points: [8, 12, 10, 9, 15, 11, 18].map((count, index) => ({ date: officialPlayDates[index], count, known: true, source: index === 6 ? 'songs/daily' : 'trend/get' })) },
  ];
  await page.route('**/api/v1/play-stats', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(officialPlayStats) }));

  await page.goto(baseURL, { waitUntil: 'networkidle' });
  await page.locator('#auth-title').waitFor({ state: 'visible' });
  assert(await page.locator('#auth-title').textContent() === '设置管理员密码', 'new install did not show setup');
  await noPageOverflow(page, 'desktop setup');
  await page.screenshot({ path: path.join(outputDir, 'desktop-setup.png'), fullPage: true });

  await page.locator('#password-input').fill(password);
  await page.locator('#setup-confirm-password').fill(password);
  await page.locator('#setup-submit').click();
  await page.locator('#app-shell').waitFor({ state: 'visible' });
  const navLabels = await page.locator('.nav-item-left > span').allTextContents();
  assert(JSON.stringify(navLabels) === JSON.stringify(['仪表盘', '账号中心', '定时任务', '策略配置', '运行日志', '系统设置']), `unexpected navigation: ${JSON.stringify(navLabels)}`);
  assert((await page.locator('#page-title').textContent()).trim() === '仪表盘', 'dashboard title was not restored');
  assert((await page.locator('#dashboard-uptime').textContent()).trim() !== '--', 'dashboard uptime was not exposed');
  const upcomingHeaders = (await page.locator('.dashboard-schedule-table th').allTextContents()).map(value => value.trim());
  assert(JSON.stringify(upcomingHeaders) === JSON.stringify(['任务名称', '指令', 'CRON 规则', '下次触发', '状态']), `unexpected dashboard schedule columns: ${JSON.stringify(upcomingHeaders)}`);
  assert(await page.locator('.turntable-player-wrap img').evaluate(image => image.complete && image.naturalWidth > 0), 'dashboard CD logo was not loaded');
  assert(await page.locator('.stat-value').first().evaluate(element => getComputedStyle(element).fontSize === '14px'), 'dashboard stat value was not 14px');
  assert(await page.locator('.stat-card .badge-pill').first().evaluate(element => getComputedStyle(element).fontSize === '11px'), 'dashboard stat chip was not 11px');
  assert((await page.locator('#stat-account-health').textContent()).includes('2 有效'), 'account availability did not use task results');
  const rewardText = (await page.locator('#account-rewards-list').innerText()).replaceAll('\n', ' ');
  assert(rewardText.includes('云贝 +12 / 345') && rewardText.includes('成长值 8 / 99'), `dashboard reward format was incomplete: ${rewardText}`);
  const auxiliaryRewardText = await page.locator('#account-rewards-list .reward-row').filter({ hasText: '测试辅助账号' }).innerText();
  assert(!auxiliaryRewardText.includes('成长值'), `non-VIP account still displayed growth: ${auxiliaryRewardText}`);
  assert(await page.locator('#dashboard-chart').evaluate(element => element.classList.contains('metric-chart')), 'main-account seven-day chart was not rendered');
  assert(await page.locator('#dashboard-chart .metric-line-svg').count() === 1, 'dashboard metric was not rendered as a line chart');
  const officialPlayValues = await page.locator('#dashboard-chart .metric-line-value').allTextContents();
  assert(officialPlayValues.includes('265') && !officialPlayValues.includes('35'), `effective plays did not use official daily statistics: ${JSON.stringify(officialPlayValues)}`);
  assert((await page.locator('#dashboard-account-select option:checked').textContent()).includes('测试主账号'), 'dashboard chart did not default to the main account');
  await page.locator('#dashboard-account-select').selectOption('./fan1.json');
  assert((await page.locator('#dashboard-account-select option:checked').textContent()).includes('测试辅助账号'), 'dashboard account chart could not be switched');
  await page.locator('#dashboard-account-select').selectOption('./cookie.json');
  for (const metric of ['growth', 'yunbei', 'plays']) {
    await page.locator(`#dashboard-metric [data-metric="${metric}"]`).click();
    assert(await page.locator('#dashboard-chart .metric-line-svg').count() === 1, `${metric} metric was not rendered as a line chart`);
    assert(await page.locator('#dashboard-chart .metric-line-path').count() >= 1, `${metric} metric did not render a line path`);
    assert(await page.locator('#dashboard-chart .metric-line-path').first().evaluate(path => path.tagName === 'path' && path.getAttribute('d').includes(' C ')), `${metric} metric was not rendered as a smooth cubic curve`);
    assert(await page.locator('#dashboard-chart .metric-area-path').count() >= 1, `${metric} metric area fill was missing`);
    assert(await page.locator('#dashboard-chart .metric-average-line').count() === 1, `${metric} metric average line was missing`);
    assert(await page.locator('#dashboard-chart polyline').count() === 0, `${metric} metric still contained a polyline`);
    assert(await page.locator('#dashboard-metric-title').evaluate(element => element.getBoundingClientRect().height < 24), `${metric} chart title wrapped to multiple lines`);
  }
  await page.waitForTimeout(150);
  const chartLayout = await page.evaluate(() => {
    const panel = document.querySelector('.dashboard-chart-panel').getBoundingClientRect();
    const chart = document.querySelector('#dashboard-chart').getBoundingClientRect();
    const svg = document.querySelector('#dashboard-chart .metric-line-svg').getBoundingClientRect();
    const viewBox = document.querySelector('#dashboard-chart .metric-line-svg').viewBox.baseVal;
    const weights = [
      document.querySelector('#dashboard-metric-title'),
      document.querySelector('#dashboard-account-select'),
      document.querySelector('#dashboard-metric button'),
      document.querySelector('.metric-line-value'),
      document.querySelector('.metric-line-label'),
    ].map(element => getComputedStyle(element).fontWeight);
    return {
      panel: { left: panel.left, right: panel.right, bottom: panel.bottom },
      chart: { left: chart.left, right: chart.right, bottom: chart.bottom, width: chart.width, height: chart.height },
      svg: { width: svg.width, height: svg.height, viewBoxWidth: viewBox.width, viewBoxHeight: viewBox.height },
      weights,
      textStroke: getComputedStyle(document.querySelector('.metric-line-label')).stroke,
    };
  });
  assert(chartLayout.chart.width > (chartLayout.panel.right - chartLayout.panel.left) - 50, `chart did not fill the card horizontally: ${JSON.stringify(chartLayout)}`);
  assert(chartLayout.panel.bottom - chartLayout.chart.bottom < 24 && chartLayout.svg.height >= chartLayout.chart.height - 1, `chart did not fill the card vertically: ${JSON.stringify(chartLayout)}`);
  assert(Math.abs(chartLayout.svg.viewBoxWidth - chartLayout.svg.width) < 2 && Math.abs(chartLayout.svg.viewBoxHeight - chartLayout.svg.height) < 2, `chart coordinates did not follow its container: ${JSON.stringify(chartLayout)}`);
  assert(chartLayout.weights.every(weight => weight === '400'), `chart typography was still bold: ${JSON.stringify(chartLayout.weights)}`);
  assert(chartLayout.textStroke === 'none', `chart text still inherited an SVG stroke: ${chartLayout.textStroke}`);
  await page.screenshot({ path: path.join(outputDir, 'desktop-dashboard.png'), fullPage: true });
  const scrollingLayout = await page.evaluate(() => ({
    workspace: getComputedStyle(document.querySelector('.workspace')).overflowY,
    pageBody: getComputedStyle(document.querySelector('.page-body')).overflowY,
    topbarBottom: document.querySelector('.topbar').getBoundingClientRect().bottom,
    bodyTop: document.querySelector('.page-body').getBoundingClientRect().top,
  }));
  assert(scrollingLayout.workspace === 'hidden' && scrollingLayout.pageBody === 'auto' && scrollingLayout.bodyTop >= scrollingLayout.topbarBottom, `topbar was not isolated from scrolling content: ${JSON.stringify(scrollingLayout)}`);
  const darkSurface = await page.locator('.stat-card').first().evaluate(element => getComputedStyle(element).backgroundColor);
  await page.locator('#theme-button').click();
  await page.waitForTimeout(250);
  const lightSurface = await page.locator('.stat-card').first().evaluate(element => getComputedStyle(element).backgroundColor);
  assert(darkSurface !== lightSurface, 'light and dark glass surfaces were identical');
  await page.locator('#menu-button').click();
  assert(await page.locator('.nav-item[data-page="dashboard"] svg').isVisible(), 'collapsed navigation icon was hidden');
  assert(await page.locator('.nav-item[data-page="dashboard"] .nav-item-left > span').isHidden(), 'collapsed navigation text was visible');
  assert(await page.locator('.sidebar-github-link svg').isVisible(), 'collapsed GitHub icon was hidden');
  assert(await page.locator('#version-label').isHidden(), 'collapsed version metadata was visible');
  await page.locator('#menu-button').click();
  await page.locator('.nav-item[data-page="accounts"]').click();
  assert(new URL(page.url()).pathname === '/account', `account route was not reflected in URL: ${page.url()}`);
  const accountHeaders = (await page.locator('#page-accounts thead th').allTextContents()).map(value => value.trim());
  assert(accountHeaders[0] === '账号昵称' && accountHeaders[1] === 'Cookie', `unexpected account columns: ${JSON.stringify(accountHeaders)}`);
  const firstAccountRow = page.locator('#accounts-table tr').first();
  if (await firstAccountRow.locator('[data-account-action="edit"]').count()) {
    const accountName = (await firstAccountRow.locator('.account-identity strong').textContent()).trim();
    assert(!['主账号', '辅助账号 1'].includes(accountName), `account nickname was still a fixed role label: ${accountName}`);
    await page.locator('.page-body').evaluate(element => { element.scrollTop = 0; });
    const musicianChip = firstAccountRow.locator('td:nth-child(4) .badge-pill').filter({ hasText: '音乐人' });
    assert(await musicianChip.isVisible(), 'musician identity chip was missing or hidden');
    await page.screenshot({ path: path.join(outputDir, 'desktop-accounts.png'), fullPage: true });
    const accountColumnsOverlap = await firstAccountRow.evaluate(row => {
      const status = row.querySelector('td:nth-child(7) .badge-pill')?.getBoundingClientRect();
      const actions = row.querySelector('td:last-child')?.getBoundingClientRect();
      return !!status && !!actions && status.right > actions.left;
    });
    assert(!accountColumnsOverlap, 'account status was covered by the action column');
    await firstAccountRow.locator('[data-account-action="edit"]').click();
    assert(await page.locator('#account-filename').evaluate(input => input.readOnly), 'editing an account allowed its cookie filename to change');
    assert(await page.locator('#account-target-fields').isHidden(), 'editing an account still displayed account type and target file');
    await page.locator('#account-close').click();
  }
  await page.locator('#account-add').click();
  assert(await page.locator('#account-panel').isVisible(), 'add account panel did not open');
  assert(Math.round((await page.locator('#account-panel').boundingBox()).width) === 340, 'add account panel did not match the 340px prototype width');
  assert(await page.locator('.account-type [data-main="false"]').evaluate(button => button.classList.contains('active')), 'new accounts did not default to auxiliary');
  assert((await page.locator('#account-filename').inputValue()) === 'fan1.json', 'new auxiliary account did not use fan1.json by default');
  await page.locator('#account-close').click();
  await page.locator('.nav-item[data-page="schedules"]').click();
  assert(new URL(page.url()).pathname === '/task', `schedule route was not reflected in URL: ${page.url()}`);
  assert(await page.locator('#schedule-refresh').count() === 0, 'schedule refresh action was not removed');
  assert(await page.locator('#schedule-select-all').count() === 0, 'schedule multi-select was not removed');
  await page.locator('#schedule-add').click();
  const scheduleDialog = await page.locator('#schedule-dialog').boundingBox();
  assert(Math.abs((scheduleDialog.x + scheduleDialog.width / 2) - 720) < 2, `schedule dialog was not centered: ${JSON.stringify(scheduleDialog)}`);
  await page.locator('#schedule-dialog .dialog-close').first().click();
  const schedulerStatus = page.locator('#stat-scheduler');
  assert((await schedulerStatus.textContent()).trim() === '调度正常', 'dashboard scheduler status was not healthy');
  assert(await schedulerStatus.evaluate(element => element.classList.contains('success')), 'dashboard scheduler status style was not healthy');
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
  const toastBox = await page.locator('.toast.error').filter({ hasText: '版本信息加载失败' }).boundingBox();
  assert(toastBox.y < 100, `toast was not displayed at the top: ${JSON.stringify(toastBox)}`);
  assert(Math.abs((toastBox.x + toastBox.width / 2) - 720) < 12, `toast was not centered: ${JSON.stringify(toastBox)}`);
  await page.unroute('**/api/v1/update');

  const notifyPath = path.join(testRoot, 'notify.yaml');
  const validNotify = fs.readFileSync(notifyPath, 'utf8');
  fs.writeFileSync(notifyPath, 'webhook: [invalid\n', 'utf8');
  await page.reload({ waitUntil: 'networkidle' });
  await page.locator('#app-shell').waitFor({ state: 'visible' });
  await page.locator('.nav-item[data-page="config"]').click();
  await page.locator('#config-target [data-target="notify"]').click();
  assert((await page.locator('#config-sidebar-title').textContent()).trim() === '推送通道', 'notify sidebar title was not updated');
  assert(await page.locator('#config-presets-bar').isHidden(), 'common presets remained visible in notify configuration');
  assert(await page.locator('#config-reload').isVisible() && await page.locator('#config-save').isVisible(), 'notify configuration actions were hidden');
  assert(await page.locator('#config-save-actions').evaluate(actions => actions.parentElement?.classList.contains('config-view-actions')), 'notify save actions were not placed beside the view switch');
  assert(await page.locator('#config-mode').evaluate((mode, actions) => !!(mode.compareDocumentPosition(actions) & Node.DOCUMENT_POSITION_FOLLOWING), await page.locator('#config-save-actions').elementHandle()), 'notify save actions were not placed to the right of the view switch');
  assert(!(await page.locator('#config-sections').textContent()).includes('(webhook)'), 'notify channel list still displayed key parentheses');
  await page.locator('#config-parse-error').waitFor({ state: 'visible' });
  assert(await page.locator('#config-yaml').isVisible(), 'invalid notify YAML did not switch to source mode');
  assert((await page.locator('#yaml-editor').inputValue()).includes('webhook: [invalid'), 'invalid raw YAML was not returned');
  assert(await page.locator('#yaml-line-numbers').isVisible(), 'YAML line numbers were not visible');
  await page.locator('#yaml-editor').fill(validNotify);
  await page.locator('#config-save').click();
  await page.locator('.toast').filter({ hasText: '配置已保存' }).waitFor({ state: 'visible' });
  assert(await page.locator('#config-parse-error').isHidden(), 'parse error remained after repair');
  assert((await page.locator('#yaml-editor').inputValue()).trimStart().startsWith('webhook:'), 'notify YAML did not narrow to the selected channel');
  await page.locator('#config-mode [data-mode="visual"]').click();
  await page.locator('#config-form .config-card-box').first().waitFor({ state: 'visible' });
  const notifyCardLayout = await page.evaluate(() => {
    const form = document.querySelector('#config-form').getBoundingClientRect();
    const card = document.querySelector('#config-form .config-card-box').getBoundingClientRect();
    return { formWidth: form.width, cardWidth: card.width };
  });
  assert(notifyCardLayout.cardWidth >= notifyCardLayout.formWidth - 2, `notify visual card did not fill the content area: ${JSON.stringify(notifyCardLayout)}`);
  await page.screenshot({ path: path.join(outputDir, 'desktop-config-notify.png'), fullPage: true });
  await page.locator('#config-target [data-target="config"]').click();
  await page.locator('#config-mode [data-mode="visual"]').click();
  await page.locator('.card-flow-section').first().waitFor({ state: 'visible' });
  assert(await page.locator('#config-presets-bar').isVisible(), 'common presets were hidden in rules configuration');
  assert(await page.locator('#config-save-actions').evaluate(actions => actions.parentElement?.id === 'config-preset-actions'), 'rule save actions did not return to the common presets bar');
  const ruleSurface = await page.evaluate(() => {
    const presets = getComputedStyle(document.querySelector('#config-presets-bar'));
    const card = getComputedStyle(document.querySelector('#page-config .sim-rules-card'));
    const sidebar = document.querySelector('#page-config .sim-rules-sidebar').getBoundingClientRect();
    return { presetBackground: presets.backgroundColor, cardBackground: card.backgroundColor, presetRadius: presets.borderRadius, cardRadius: card.borderRadius, sidebarWidth: sidebar.width };
  });
  assert(ruleSurface.presetBackground === ruleSurface.cardBackground && ruleSurface.presetRadius === ruleSurface.cardRadius, `preset and rules surfaces diverged: ${JSON.stringify(ruleSurface)}`);
  assert(ruleSurface.sidebarWidth >= 258, `configuration sidebar was not widened: ${JSON.stringify(ruleSurface)}`);
  const firstConfigSection = page.locator('#config-sections [data-section]').first();
  assert(await firstConfigSection.getAttribute('data-section') === 'task' && (await firstConfigSection.innerText()).includes('批量任务'), 'batch tasks were not the first simplified navigation item');
  assert((await page.locator('#config-current-title').innerText()).includes('规则配置面板'), 'continuous flow title was not rendered');
  assert(await page.locator('.card-flow-section').count() === 15, 'configuration did not render all 15 modules');
  assert(await page.locator('.config-card-box').count() === 29, 'configuration card flow did not match the prototype');
  assert(await page.locator('#config-block-task .task-card').count() === 8, 'task matrix did not render eight task cards');
  assert(await page.locator('#config-block-task .schema-segmented').count() === 1, 'task mode segmented control was missing');
  assert(await page.locator('.schema-fallback').count() === 0, 'registered task fields fell back to generic controls');
  const flowSize = await page.locator('#config-visual').evaluate(element => ({ clientHeight: element.clientHeight, scrollHeight: element.scrollHeight }));
  assert(flowSize.scrollHeight > flowSize.clientHeight, `configuration flow was not independently scrollable: ${JSON.stringify(flowSize)}`);
  await page.locator('#config-search').fill('任务队列调度模式');
  assert(await page.locator('.card-flow-section:not(.search-hidden)').count() === 1, 'full-flow search did not narrow the matching module');
  assert(await page.locator('#config-block-task .task-card:not(.search-hidden)').count() === 0, 'full-flow search left unrelated task cards visible');
  await page.locator('#config-search').fill('');
  for (const [section, control] of [['accounts', '.config-token-control'], ['musician', '.schema-stepper'], ['mixPlay', '.single-range-input'], ['note', '.schema-textarea'], ['dailySongShare', '.schema-json'], ['network', '.quick-pill'], ['playids', '.schema-tags']]) {
    await page.locator(`#config-sections [data-section="${section}"]`).click();
    await page.waitForFunction(key => document.querySelector(`#config-sections [data-section="${key}"]`)?.classList.contains('active'), section);
    assert(await page.locator(`#config-block-${section} ${control}`).count() > 0, `${section} prototype control ${control} was missing`);
  }
  await page.locator('#config-sections [data-section="task"]').click();
  await page.waitForFunction(() => document.querySelector('#config-visual').scrollTop < 40);
  await page.screenshot({ path: path.join(outputDir, 'desktop-config-schema.png'), fullPage: true });
  const lightConfigSurface = await page.locator('.config-card-box').first().evaluate(element => getComputedStyle(element).backgroundColor);
  await page.locator('#theme-button').click();
  await page.waitForTimeout(200);
  const darkConfigSurface = await page.locator('.config-card-box').first().evaluate(element => getComputedStyle(element).backgroundColor);
  assert(lightConfigSurface !== darkConfigSurface, 'schema cards did not respond to theme changes');
  await page.screenshot({ path: path.join(outputDir, 'desktop-config-schema-dark.png'), fullPage: true });
  await page.locator('#theme-button').click();
  await page.locator('#config-sections [data-section="task"]').click();
  await page.waitForFunction(() => document.querySelector('#config-sections [data-section="task"]')?.classList.contains('active'));
  await page.locator('#config-mode [data-mode="yaml"]').click();
  const businessYAML = await page.locator('#yaml-editor').inputValue();
  assert(/(^|\n)task:\s*(\n|$)/.test(businessYAML), 'business YAML did not contain the selected module');
  assert(!businessYAML.includes('\naccounts:'), 'business YAML displayed another module');

  await page.locator('.nav-item[data-page="runs"]').click();
  assert(await page.locator('#run-select-all, .run-row-checkbox').count() === 0, 'run log multi-select controls were not removed');
  assert(await page.locator('#logs-auto-open').isVisible(), 'automatic cleanup action was missing');
  assert(await page.locator('#logs-advanced-open').isVisible(), 'advanced cleanup action was missing');
  assert(await page.locator('#logs-cleanup').count() === 0, 'legacy immediate cleanup action still existed');

  await page.locator('.nav-item[data-page="system"]').click();
  assert(await page.locator('#update-panel').isHidden(), 'unchecked update details were visible');
  assert(await page.locator('.service-metrics > div').count() === 4, 'system overview did not restore four runtime metrics');
  const lightMetricSurface = await page.locator('.service-metrics > div').first().evaluate(element => getComputedStyle(element).backgroundColor);
  assert(lightMetricSurface !== 'rgba(0, 0, 0, 0)' && lightMetricSurface !== 'transparent', `light runtime metric background was invisible: ${lightMetricSurface}`);
  assert(await page.locator('.service-addresses').evaluate(element => getComputedStyle(element).borderTopWidth === '0px'), 'service addresses still had an outer border');
  assert(await page.locator('.service-addresses > div').first().evaluate(element => getComputedStyle(element).borderBottomWidth === '0px'), 'service address rows still had divider borders');
  assert(await page.locator('.storage-location-grid > div').first().evaluate(element => getComputedStyle(element).borderTopWidth === '0px'), 'storage items still had borders');
  const overviewLayout = await page.evaluate(() => {
    const product = document.querySelector('.product-card').getBoundingClientRect();
    const service = document.querySelector('.service-card').getBoundingClientRect();
    const addresses = document.querySelector('.service-addresses').getBoundingClientRect();
    return { productHeight: product.height, serviceHeight: service.height, serviceBottomSpace: service.bottom - addresses.bottom };
  });
  assert(Math.abs(overviewLayout.productHeight - overviewLayout.serviceHeight) < 1, `system overview cards were not equal height: ${JSON.stringify(overviewLayout)}`);
  assert(overviewLayout.serviceBottomSpace > 20, `service addresses were still pinned to the card bottom: ${JSON.stringify(overviewLayout)}`);
  await page.route('**/api/v1/update/check', route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ current_version: '1.2.0', latest_version: '1.2.0', last_check_time: now, os: 'windows', arch: 'amd64', available: false, message: '当前已是最新版本。' }),
  }));
  await page.locator('#update-check').click();
  await page.locator('#update-panel').waitFor({ state: 'visible' });
  await page.waitForFunction(() => document.querySelector('#update-status-title')?.textContent === '当前已是最新版本');
  const updatePanelText = await page.locator('#update-panel').innerText();
  assert((updatePanelText.match(/当前已是最新版本/g) || []).length === 1, `latest-version result was duplicated: ${updatePanelText}`);
  assert((await page.locator('#update-message').textContent()).startsWith('检查时间：'), 'checked update details did not show the check time');
  await page.screenshot({ path: path.join(outputDir, 'desktop-system.png'), fullPage: true });
  await page.unroute('**/api/v1/update/check');
  await page.locator('[data-system-tab="security"]').click();
  assert(Number(await page.locator('#password-min-length').inputValue()) === 1, 'default password minimum length was not 1');
  assert(!(await page.locator('#require-letters').isChecked()) && !(await page.locator('#require-digits').isChecked()) && !(await page.locator('#require-symbols').isChecked()), 'default password character requirements were enabled');
  assert(await page.locator('.security-action-bar').isHidden(), 'security action bar was visible without changes');
  await page.locator('#require-letters + .slider').click();
  await page.locator('.security-action-bar').waitFor({ state: 'visible' });
  assert((await page.locator('#security-dirty-note').textContent()).trim() === '有未保存的设置项：1', 'security dirty count was not dynamic');
  const actionBarPosition = await page.locator('.security-action-bar').evaluate(element => getComputedStyle(element).position);
  assert(actionBarPosition === 'fixed', `security action bar was not fixed: ${actionBarPosition}`);
  const actionBarEdges = await page.locator('.security-action-bar').evaluate(element => {
    const rect = element.getBoundingClientRect();
    const sidebar = document.querySelector('#sidebar').getBoundingClientRect();
    return { left: rect.left, sidebarRight: sidebar.right, right: innerWidth - rect.right, bottom: innerHeight - rect.bottom };
  });
  assert(Math.abs(actionBarEdges.left - actionBarEdges.sidebarRight) < 1 && actionBarEdges.right < 1 && actionBarEdges.bottom < 1, `security action bar was not flush to workspace edges: ${JSON.stringify(actionBarEdges)}`);
  await page.locator('#auth-settings-reset').click();
  assert(await page.locator('.security-action-bar').isHidden(), 'security action bar remained after restoring settings');

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
  await mobilePage.locator('.nav-item[data-page="accounts"]').click();
  await mobilePage.waitForTimeout(200);
  await hasHorizontalScroll(mobilePage.locator('.account-table-wrap'), 'mobile account table');
  await mobilePage.locator('#menu-button').click();
  await mobilePage.locator('.nav-item[data-page="schedules"]').click();
  await mobilePage.waitForTimeout(300);
  await hasHorizontalScroll(mobilePage.locator('#page-schedules .schedule-table-card'), 'mobile schedule table');
  await noPageOverflow(mobilePage, 'mobile schedules');
  await mobilePage.screenshot({ path: path.join(outputDir, 'mobile-schedules.png'), fullPage: true });
  await mobilePage.locator('#menu-button').click();
  await mobilePage.locator('.nav-item[data-page="config"]').click();
  await mobilePage.locator('.card-flow-section').first().waitFor({ state: 'visible' });
  await mobilePage.waitForTimeout(350);
  assert(await mobilePage.locator('.card-flow-section').count() === 15, 'mobile configuration flow did not render all modules');
  assert(await mobilePage.locator('#config-block-task .task-card').count() === 8, 'mobile task matrix was incomplete');
  await noPageOverflow(mobilePage, 'mobile configuration');
  await mobilePage.screenshot({ path: path.join(outputDir, 'mobile-config.png'), fullPage: true });
  await mobilePage.locator('#menu-button').click();
  await mobilePage.locator('.nav-item[data-page="runs"]').click();
  await mobilePage.waitForTimeout(200);
  await hasHorizontalScroll(mobilePage.locator('#page-runs .schedule-table-card'), 'mobile run table');
  await mobile.close();

  assert(pageErrors.length === 0, `browser page errors: ${JSON.stringify(pageErrors)}`);
  await desktop.close();
  await browser.close();
  process.stdout.write(JSON.stringify({ ok: true, outputDir, cookies: cookies.map(cookie => ({ name: cookie.name, httpOnly: cookie.httpOnly, sameSite: cookie.sameSite })), pageErrors }));
})().catch(error => {
  process.stderr.write(error.stack || String(error));
  process.exit(1);
});
