(() => {
  'use strict';

  const state = {
    authenticated: false,
    csrf: '',
    auth: null,
    authMode: 'login',
    authSettings: null,
    savedAuthSettings: null,
    authSessions: [],
    status: null,
    config: null,
    notify: null,
    configSchema: null,
    configTarget: 'config',
    configMode: 'visual',
    configSection: 'task',
    configSearch: '',
    configDirty: false,
    configDirtyMode: '',
    schedules: [],
    runs: [],
	playStats: [],
	playStatsRunToken: null,
    settings: null,
    update: null,
    updateChecked: false,
    accountMain: false,
    accountMethod: 'cookie',
    accountEditing: null,
    dashboardMetric: 'plays',
    dashboardAccountPath: '',
    currentPage: 'dashboard',
    scheduleSearch: '',
    runFilter: 'all',
    runSearch: '',
    qrcodeSession: null,
    qrcodePoller: null,
    qrcodeImageURL: '',
    poller: null,
    pollInFlight: false,
    pollGeneration: 0,
    lastOnlineAt: null,
    consecutivePollFailures: 0,
    openLogContent: '',
  };
  let dashboardChartObserver = null;
  let dashboardChartResizeTimer = 0;

  const pageTitles = { dashboard: '仪表盘', accounts: '账号中心', schedules: '定时任务', config: '策略配置', runs: '运行日志', system: '系统设置' };
  const pageRoutes = { dashboard: '/', accounts: '/account', schedules: '/task', config: '/config', runs: '/logs', system: '/system' };
  const routePages = Object.fromEntries(Object.entries(pageRoutes).map(([page, route]) => [route, page]));
	const playStatsCommands = new Set(['task', 'musician', 'musician-sign', 'musician-vip']);
	const databaseRunCommands = new Set(['task', 'playids', 'musician', 'musician-sign', 'musician-vip', 'vip-member-gift']);
  const sectionLabels = {
    version: '配置版本', accounts: '账号与凭据', task: '批量任务', network: '网络与代理', playids: '指定播放',
    sign: '日常签到', mixPlay: '日推混听', note: '动态笔记', dailySongShare: '每日推歌',
    vipMemberGift: '会员礼品卡', musician: '音乐人与 VIP', fansgroup: '乐迷团打卡', database: '本地数据库',
    log: '日志与轮转', updater: '自动更新', notify: '失败通知策略',
    webhook: 'Webhook', bark: 'Bark', serverchan: 'Server 酱', telegram: 'Telegram', dingtalk: '钉钉',
    coolpush: 'CoolPush', pushplus: 'PushPlus', wecom_key: '企业微信群', wecom_app: '企业微信应用',
  };
  const sectionIcons = { accounts: '', task: '', musician: '', playids: '', sign: '', mixPlay: '', note: '', dailySongShare: '', vipMemberGift: '', fansgroup: '', network: '', log: '', database: '', notify: '', updater: '', version: '' };
  const fieldLabels = {
    main: '主账号', primary: '旧版主账号', secondary: '辅助账号', antiCheatTokens: '反作弊令牌',
    enabled: '启用', enableMain: '启用主账号', enableSecondaries: '启用辅助账号', automatic: '自动执行',
    enableVipTask: '黑胶 VIP 任务', mode: '执行模式', fast_tasks: '快速任务', slow_tasks: '慢速任务',
    timeout: '超时时间', retry: '重试次数', debug: '调试模式', user_agent: 'User-Agent',
    daily_min: '每日最小数量', daily_max: '每日最大数量', run_min: '单次最小数量', run_max: '单次最大数量',
    gap_min: '最小间隔（秒）', gap_max: '最大间隔（秒）', ids: '歌曲 ID', idsFile: '歌曲 ID 文件',
    filepath: '文件路径', interval: '刷新间隔', driver: '驱动', directory: '目录',
    format: '格式', level: '级别', stdout: '输出到控制台', rotate: '日志轮转', filename: '文件名',
    maxsize: '单文件上限（MB）', maxage: '保留天数', maxbackups: '备份数量', localtime: '使用本地时间', compress: '压缩备份',
    check: '检查更新', auto_update: '自动更新', proxy_mirrors: '代理镜像',
    title_prefix: '标题前缀', on_skip: '跳过时通知', file: '配置文件',
    titles: '标题列表', titlesFile: '标题文件', messages: '正文列表', messagesFile: '正文文件', imageUrls: '图片地址',
    autoDelete: '自动删除', songId: '歌曲 ID', playlistId: '歌单 ID', imageMode: '图片模式', titleMode: '标题模式',
    topics: '话题', lottery: '抽奖', activityId: '活动 ID', autoRegister: '自动报名',
    enableGift: '发布赠礼', enableClaim: '领取赠礼', cloud: '云端服务', baseUrl: '服务地址', token: '令牌',
    webhook: '自定义 Webhook', bark: 'Bark', serverchan: 'Server 酱', telegram: 'Telegram', dingtalk: '钉钉机器人',
    coolpush: 'CoolPush / Qmsg', pushplus: 'PushPlus', wecom_key: '企业微信群机器人', wecom_app: '企业微信应用消息',
    url: '地址', method: '请求方法', headers: '请求头', body_template: '请求体模板', key: 'Key', server: '服务地址',
    sckey: 'SCKEY', bot_token: 'Bot Token', user_id: '用户 ID', api_host: 'API 地址', proxy: '代理',
    access_token: 'Access Token', secret: '加签 Secret', skey: 'SKEY', corp_id: '企业 ID', corp_secret: '应用 Secret',
    to_user: '接收用户', agent_id: '应用 ID', media_id: '媒体 ID',
  };
  const configSectionOrder = [
    'task', 'accounts', 'network', 'sign', 'playids', 'mixPlay', 'musician',
    'note', 'dailySongShare', 'fansgroup', 'vipMemberGift', 'notify',
    'updater', 'log', 'database'
  ];
  const configFlowMeta = {
    task: { nav: '批量任务', title: '批量任务总开关与调度', description: '执行 ncmm task 时按勾选状态执行' },
    accounts: { nav: '账号与凭据', title: '账号路径与 Token 凭据', description: '主/辅账号 Cookie 映射与移动端抓包凭证' },
    network: { nav: '网络与 UA', title: '网络请求与多端协议 UA 伪装', description: '超时、步进重试与 Web/iOS/Android User-Agent' },
    sign: { nav: '日常签到', title: '日常一键签到与云贝任务', description: '云贝中心任务、黑胶 VIP 签到与福利领取' },
    playids: { nav: '播放歌曲', title: '播放指定歌曲有效播放量', description: '批量冲刺指定歌曲或歌单有效播放量目标' },
    mixPlay: { nav: '日推混听', title: '日推混听防风控策略', description: '按比例穿插播放官方日推歌曲，模拟真人听歌' },
    musician: { nav: '音乐人任务', title: '音乐人专属任务与 VIP 进阶', description: '日常签到领云豆、VIP 月度进阶与专属播放覆盖' },
    note: { nav: '图文动态', title: '图文动态与笔记素材池', description: '通用随机标题、正文与配图素材池（供推歌与音乐人复用）' },
    dailySongShare: { nav: '每日推歌', title: '每日推歌与抽奖', description: '推歌抽奖、话题标签、封面模式与秒删保护（需 antiCheatToken）' },
    fansgroup: { nav: '乐迷团', title: '乐迷团日常打卡', description: '音乐合伙人与关注歌手乐迷团日常应援打卡' },
    vipMemberGift: { nav: '会员礼品', title: '黑胶会员礼品卡与云端互助', description: '社区共享库互助赠送与领取黑胶会员天数（需 antiCheatToken）' },
    notify: { nav: '通知策略', title: '运行失败通知策略', description: '任务异常或跳过时的汇总推送策略（通道凭证见 notify.yaml）' },
    updater: { nav: '自动更新', title: '自动更新与加速镜像', description: '新版本检测、自动替换与 GitHub 代理镜像加速' },
    log: { nav: '日志管理', title: '日志记录与滚动归档策略', description: '日志格式、控制台输出、切割体积与过期保留天数' },
    database: { nav: '本地数据库', title: '本地持久化缓存数据库', description: '持久化保存每日播放进度、身份与 Token 缓存' },
  };
  const taskFlowMeta = {
    sign: { badge: '日常必备', tone: 'success' },
    playids: { badge: '核心冲量', tone: 'cyan' },
    'musician-sign': { badge: '音乐人专属', tone: 'amber' },
    'musician-vip': { badge: '每月进阶', tone: 'primary' },
    'daily-song-share': { badge: '需 Token', tone: 'cyan' },
    'vip-member-gift': { badge: '需 Token', tone: 'success' },
    fansgroup: { badge: '日常打卡', tone: 'amber' },
    note: { badge: '可选', tone: 'primary' },
  };
  const notifySectionOrder = ['webhook', 'bark', 'serverchan', 'telegram', 'dingtalk', 'coolpush', 'pushplus', 'wecom_key', 'wecom_app'];

  const PRESET_STRATEGIES = {
    novice: {
      name: '小白省心 (推荐)',
      hint: '全自动日常打卡 + 30%防风控刷歌 + 自动领会员',
      config: {
        task: { sign: true, playids: true, 'musician-sign': true, 'musician-vip': false, note: false, 'daily-song-share': false, 'vip-member-gift': true, fansgroup: true, mode: 'by-task-group' },
        sign: { automatic: true, enableMain: true, enableSecondaries: true, enableVipTask: true },
        playids: { enableMain: true, enableSecondaries: true, daily_min: 40, daily_max: 100, run_min: 10, run_max: 30, gap_min: 15, gap_max: 35 },
        mixPlay: { enabled: true, dailyRecommendRatio: 0.3, countTarget: true },
        vipMemberGift: { enableMain: true, enableSecondaries: true, enableGift: true, enableClaim: true },
        fansgroup: { enableMain: true, enableSecondaries: true, autoDeleteNote: true },
        network: { timeout: '60s', retry: 3, debug: false },
        updater: { check: true, auto_update: true },
        log: { level: 'info', stdout: true }
      }
    },
    musician: {
      name: '音乐人冲刺',
      hint: '开启全量刷歌与 VIP 进阶长耗时任务，全面提升音乐人身份数据',
      config: {
        task: { sign: true, playids: true, 'musician-sign': true, 'musician-vip': true, note: true, 'daily-song-share': true, 'vip-member-gift': true, fansgroup: true, mode: 'by-task-group' },
        musician: { enableMain: true, enableSecondaries: true, identityCacheDays: 0, enableVipNote: true, enableVipPlay: true },
        playids: { enableMain: true, enableSecondaries: true, daily_min: 100, daily_max: 300, run_min: 30, run_max: 60, gap_min: 20, gap_max: 40 },
        mixPlay: { enabled: true, dailyRecommendRatio: 0.25, countTarget: true },
        note: { type: 39, autoDelete: false },
        dailySongShare: { enableMain: true, enableSecondaries: true, autoDelete: false }
      }
    },
    stealth: {
      name: '高防风控安全',
      hint: '拉大播放间隔、45%高防风控混听、轻量低频刷歌，全方位规避账号风控',
      config: {
        task: { sign: true, playids: true, 'musician-sign': true, 'musician-vip': false, note: false, 'daily-song-share': false, 'vip-member-gift': false, fansgroup: true, mode: 'by-task-group' },
        playids: { enableMain: true, enableSecondaries: true, daily_min: 30, daily_max: 60, run_min: 5, run_max: 15, gap_min: 35, gap_max: 75 },
        mixPlay: { enabled: true, dailyRecommendRatio: 0.45, countTarget: true },
        network: { timeout: '90s', retry: 2, debug: false }
      }
    },
    sign_only: {
      name: '极速纯签到',
      hint: '仅秒级打卡（签到、乐迷团、领云豆），不刷歌不发动态，3秒极速结束',
      config: {
        task: { sign: true, playids: false, 'musician-sign': true, 'musician-vip': false, note: false, 'daily-song-share': false, 'vip-member-gift': false, fansgroup: true, mode: 'by-task-group' },
        mixPlay: { enabled: false },
        sign: { automatic: true, enableMain: true, enableSecondaries: true, enableVipTask: true }
      }
    }
  };
  const notifyFieldOrder = {
    webhook: ['enabled', 'url', 'method', 'headers', 'body_template'], bark: ['enabled', 'key', 'server'],
    serverchan: ['enabled', 'sckey'], telegram: ['enabled', 'bot_token', 'user_id', 'api_host', 'proxy'],
    dingtalk: ['enabled', 'access_token', 'secret'], coolpush: ['enabled', 'skey', 'mode'],
    pushplus: ['enabled', 'token'], wecom_key: ['enabled', 'key'],
    wecom_app: ['enabled', 'corp_id', 'corp_secret', 'to_user', 'agent_id', 'media_id'],
  };

  function sectionIcon(key) {
    return sectionIcons[key] || '◆';
  }

  const $ = (selector, root = document) => root.querySelector(selector);
  const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];
  const escapeHTML = (value) => String(value ?? '').replace(/[&<>'"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[c]));
  const icon = (name) => `<svg aria-hidden="true"><use href="#i-${name}"></use></svg>`;
  const passwordMaxLength = 64;

  function safeAvatarURL(value) {
    try {
      const url = new URL(String(value || '').replace(/^http:/i, 'https:'));
      return url.protocol === 'https:' && url.hostname.endsWith('.music.126.net') ? url.href : '';
    } catch {
      return '';
    }
  }

  function passwordSettings() {
    return state.auth?.settings || {
      passwordMinLength: 1,
      passwordRequireLetters: false,
      passwordRequireDigits: false,
      passwordRequireSymbols: false,
    };
  }

  function passwordCharAllowed(char) {
    return /^[\x21-\x7E]$/.test(char);
  }

  function normalizePasswordInput(value, settings) {
    return Array.from(value).filter(char => passwordCharAllowed(char, settings)).join('').slice(0, passwordMaxLength);
  }

  function enabledPasswordTypesText(settings) {
    const labels = [];
    if (settings.passwordRequireLetters) labels.push('英文字母');
    if (settings.passwordRequireDigits) labels.push('数字');
    if (settings.passwordRequireSymbols) labels.push('符号');
    if (!labels.length) return '可见 ASCII 字符';
    if (labels.length === 1) return labels[0];
    if (labels.length === 2) return `${labels[0]}和${labels[1]}`;
    return `${labels.slice(0, -1).join('、')}和${labels.at(-1)}`;
  }

  function analyzePassword(password, settings = passwordSettings()) {
    const hasLetters = /[A-Za-z]/.test(password);
    const hasDigits = /\d/.test(password);
    const hasSymbols = /[!"#$%&'()*+,\-./:;<=>?@[\\\]^_`{|}~]/.test(password);
    const categories = [/[a-z]/.test(password), /[A-Z]/.test(password), hasDigits, hasSymbols].filter(Boolean).length;
    const lengthOk = password.length >= settings.passwordMinLength && password.length <= passwordMaxLength;
    const charsOk = password.length > 0 && password === normalizePasswordInput(password, settings);
    const lettersOk = !settings.passwordRequireLetters || hasLetters;
    const digitsOk = !settings.passwordRequireDigits || hasDigits;
    const symbolsOk = !settings.passwordRequireSymbols || hasSymbols;
    const requiredTypesOk = lettersOk && digitsOk && symbolsOk;
    const score = Number(lengthOk) + Number(requiredTypesOk) + Number(password.length >= 12) + Number(categories >= 3);
    return {
      lengthOk, charsOk, lettersOk, digitsOk, symbolsOk,
      valid: lengthOk && charsOk && requiredTypesOk,
      strength: score >= 4 ? 'strong' : score >= 2 ? 'medium' : 'weak',
    };
  }

  async function api(path, options = {}) {
    const method = (options.method || 'GET').toUpperCase();
    const headers = { ...(options.headers || {}) };
    if (['POST', 'PUT', 'PATCH', 'DELETE'].includes(method) && state.csrf) headers['X-NCMM-CSRF'] = state.csrf;
    if (options.body && typeof options.body !== 'string') {
      headers['Content-Type'] = 'application/json';
      options.body = JSON.stringify(options.body);
    }
    const response = await fetch(path, { ...options, headers, credentials: 'same-origin' });
    const type = response.headers.get('content-type') || '';
    const payload = type.includes('application/json') ? await response.json() : await response.text();
    if (!response.ok) {
      if (response.status === 401) clearSession(true);
      throw new Error(payload?.error || payload || `HTTP ${response.status}`);
    }
    return payload;
  }

  async function publicApi(path, options = {}) {
    const headers = { ...(options.headers || {}) };
    if (options.body && typeof options.body !== 'string') {
      headers['Content-Type'] = 'application/json';
      options.body = JSON.stringify(options.body);
    }
    const response = await fetch(path, { ...options, headers, credentials: 'same-origin' });
    const type = response.headers.get('content-type') || '';
    const payload = type.includes('application/json') ? await response.json() : await response.text();
    if (!response.ok) throw new Error(payload?.error || payload || `HTTP ${response.status}`);
    return payload;
  }

  async function apiBlob(path) {
    const response = await fetch(path, { credentials: 'same-origin' });
    if (!response.ok) {
      const type = response.headers.get('content-type') || '';
      const payload = type.includes('application/json') ? await response.json() : await response.text();
      if (response.status === 401) clearSession(true);
      throw new Error(payload?.error || payload || `HTTP ${response.status}`);
    }
    return response.blob();
  }

  function toast(message, error = false) {
    const node = document.createElement('div');
    node.className = `toast${error ? ' error' : ''}`;
    node.textContent = message;
    $('#toast-region').append(node);
    setTimeout(() => node.remove(), 4200);
  }

  function applyTheme(mode, persist = false) {
    const theme = mode === 'light' ? 'light' : 'dark';
    document.documentElement.dataset.theme = theme;
    $('#theme-icon-use')?.setAttribute('href', theme === 'dark' ? '#i-sun' : '#i-moon');
    $('#theme-button')?.setAttribute('title', theme === 'dark' ? '切换到浅色主题' : '切换到深色主题');
    $('.turntable-player-wrap img')?.setAttribute('src', theme === 'dark' ? '/cd-dark.svg' : '/cd-light.svg');
    if (persist) localStorage.setItem('ncmm-theme', theme);
  }

  async function enterApp(auth) {
    state.csrf = auth.csrfToken || state.csrf;
    state.auth = { ...(state.auth || {}), ...auth };
    try {
      await loadCoreState();
    } catch (error) {
      state.authenticated = false;
      showLoginView(state.auth);
      showAuthError(`控制台初始化失败：${error.message}`);
      return;
    }
    state.authenticated = true;
    $('#login-view').classList.add('hidden');
    $('#app-shell').classList.remove('hidden');
    navigate(routePages[location.pathname] || 'dashboard', { history: false });
    setServiceStatus('online');
    startPolling();
    void loadOptionalState();
  }

  function clearAuthError() {
    $('#auth-error-message').textContent = '';
    $('#auth-error').classList.add('hidden');
  }

  function showAuthError(message, revealRecovery = false) {
    $('#auth-error-message').textContent = message;
    $('#auth-error').classList.remove('hidden');
    $('#forgot-password-wrap').classList.toggle('visible', revealRecovery);
  }

  function renderPasswordStrength() {
    const password = $('#password-input').value;
    const container = $('#password-strength');
    if (state.authMode !== 'setup' || !password) {
      container.classList.add('hidden');
      return;
    }
    const settings = passwordSettings();
    const analysis = analyzePassword(password, settings);
    const labels = { weak: '弱', medium: '中', strong: '强' };
    const progress = { weak: 28, medium: 62, strong: 100 };
    const colors = { weak: '#ef4444', medium: '#ed6c02', strong: '#2aae67' };
    const rules = [
      { ok: analysis.lengthOk, label: `${settings.passwordMinLength}-${passwordMaxLength} 个字符` },
      ...(settings.passwordRequireLetters ? [{ ok: analysis.lettersOk, label: '包含英文字母' }] : []),
      ...(settings.passwordRequireDigits ? [{ ok: analysis.digitsOk, label: '包含数字' }] : []),
      ...(settings.passwordRequireSymbols ? [{ ok: analysis.symbolsOk, label: '包含符号' }] : []),
    ];
    $('#strength-label').className = `strength-chip ${analysis.strength}`;
    $('#strength-label').textContent = labels[analysis.strength];
    $('#strength-progress').style.width = `${progress[analysis.strength]}%`;
    $('#strength-progress').style.backgroundColor = colors[analysis.strength];
    $('#strength-rules').innerHTML = rules.map(rule => `<span class="${rule.ok ? 'ok' : ''}">${rule.ok ? '✓' : '—'} ${escapeHTML(rule.label)}</span>`).join('');
    container.classList.remove('hidden');
  }

  function setAuthMode(mode, status = state.auth || {}) {
    state.authMode = mode;
    state.auth = status;
    const setup = mode === 'setup';
    $('#auth-title').textContent = setup ? '设置管理员密码' : 'NCMM';
    $('#auth-subtitle').textContent = setup ? '此密码用于保护本机的管理后台' : '网易云音乐任务管理后台';
    $('#password-input').placeholder = setup ? '设置管理员密码' : '管理员密码';
    $('#password-input').autocomplete = setup ? 'new-password' : 'current-password';
    $('#login-submit').classList.toggle('hidden', setup);
    $('#setup-confirm-field').classList.toggle('hidden', !setup);
    $('#setup-confirm-password').required = setup;
    $('#setup-submit').classList.toggle('hidden', !setup);
    $('#forgot-password-wrap').classList.remove('visible', 'open');
    clearAuthError();
    renderPasswordStrength();
    if (status.version) $('#auth-version').textContent = `v${String(status.version).split('\n')[0].replace(/^v/i, '')}`;
  }

  function showLoginView(status = state.auth || {}) {
    setAuthMode('login', status);
    $('#login-view').classList.remove('hidden');
    $('#password-input').focus();
  }

  function showSetupView(setup = {}) {
    state.authenticated = false;
    state.csrf = '';
    setAuthMode('setup', setup);
    $('#login-view').classList.remove('hidden');
    $('#password-input').focus();
  }

  function clearSession(showLogin = true) {
    state.authenticated = false;
    state.csrf = '';
    clearInterval(state.poller);
    clearInterval(state.qrcodePoller);
    state.poller = null;
    state.pollGeneration += 1;
    state.pollInFlight = false;
    state.qrcodePoller = null;
    clearQRCodeImage();
    $('#app-shell').classList.add('hidden');
    if (showLogin) showLoginView();
  }

  async function logout() {
    try { await publicApi('/api/v1/auth/logout', { method: 'POST', headers: { 'X-NCMM-CSRF': state.csrf } }); }
    catch { /* local logout still clears the browser state */ }
    clearSession(true);
  }

  async function loadCoreState() {
    const [status, config, schedules, runs, settings, auth] = await Promise.all([
      api('/api/v1/status'), api('/api/v1/config'), api('/api/v1/schedules'), api('/api/v1/runs'), api('/api/v1/settings'),
      publicApi('/api/v1/auth/status'),
    ]);
    state.status = status;
    state.config = config;
    state.schedules = schedules;
    state.runs = runs;
    state.settings = settings;
    state.auth = auth;
    state.csrf = auth.csrfToken || state.csrf;
    state.configDirty = false;
    state.configDirtyMode = '';
    renderAll();
    setAccountMethod(state.accountMethod);
  }

  async function loadOptionalState() {
    const requests = [
      ['通知配置', '/api/v1/notify', value => { state.notify = value; }],
      ['配置元数据', '/api/v1/config/schema', value => { state.configSchema = value; }],
      ['认证策略', '/api/v1/auth/settings', value => { state.authSettings = value; state.savedAuthSettings = structuredClone(value); }],
      ['版本信息', '/api/v1/update', value => { state.update = value; }],
      ['二维码状态', '/api/v1/accounts/qrcode', value => { state.qrcodeSession = value; }],
	  ['有效播放统计', '/api/v1/play-stats', value => {
		state.playStats = value;
		state.playStatsRunToken = playStatsRunToken(state.runs);
	  }, true],
    ];
    const results = await Promise.allSettled(requests.map(([, path]) => api(path)));
    if (!state.authenticated) return;
    results.forEach((result, index) => {
	  const [label, , apply, quiet] = requests[index];
      if (result.status === 'fulfilled') apply(result.value);
	  else if (!quiet) toast(`${label}加载失败：${result.reason.message}`, true);
    });
    const qrcodeSession = state.qrcodeSession;
    if (qrcodeSession) {
      state.accountMain = !!qrcodeSession.main;
      state.accountMethod = 'qrcode';
      $('#account-filename').value = qrcodeSession.filename;
      $$('.account-type button').forEach(button => button.classList.toggle('active', (button.dataset.main === 'true') === state.accountMain));
    }
    renderAll();
    setAccountMethod(state.accountMethod);
    if (qrcodeSessionActive()) openAccountPanel();
    if (qrcodeSessionActive()) startQRCodePolling();
  }

  function renderAll() {
    renderStatus();
    renderConfig();
    renderAccounts();
    renderSchedules();
    renderRuns();
    renderSettings();
    renderSystem();
    renderDashboard();
    renderQRCodeSession();
  }

  function clearQRCodeImage() {
    if (state.qrcodeImageURL) URL.revokeObjectURL(state.qrcodeImageURL);
    state.qrcodeImageURL = '';
    const image = $('#qrcode-image');
    if (image) {
      image.removeAttribute('src');
      image.classList.add('hidden');
    }
    $('#qrcode-placeholder')?.classList.remove('hidden');
  }

  function qrcodeSessionActive(session = state.qrcodeSession) {
    return !!session && ['starting', 'waiting', 'cancelling'].includes(session.status);
  }

  function renderQRCodeSession() {
    const session = state.qrcodeSession;
    const status = session?.status || 'idle';
    const labels = {
      idle: '未生成', starting: '生成中', waiting: '等待扫码', cancelling: '取消中',
      succeeded: '登录成功', failed: '登录失败', expired: '已超时', canceled: '已取消',
    };
    const badgeClasses = {
      idle: 'disabled', starting: 'running', waiting: 'running', cancelling: 'running',
      succeeded: 'success', failed: 'failed', expired: 'failed', canceled: 'disabled',
    };
    $('#qrcode-badge').className = `badge ${badgeClasses[status] || 'disabled'}`;
    $('#qrcode-badge').textContent = labels[status] || status;
    $('#qrcode-message').textContent = session?.message || '等待生成二维码';
    $('#qrcode-filename').textContent = session?.filename || $('#account-filename').value || 'cookie.json';
    $('#qrcode-cancel').classList.toggle('hidden', !qrcodeSessionActive());
    $('#account-submit').disabled = qrcodeSessionActive();
    $$('#account-login-method button, .account-type button').forEach(button => { button.disabled = qrcodeSessionActive(); });
  }

  async function loadQRCodeImage(id) {
    if (state.qrcodeImageURL) return;
    const blob = await apiBlob(`/api/v1/accounts/qrcode/${encodeURIComponent(id)}/image`);
    state.qrcodeImageURL = URL.createObjectURL(blob);
    $('#qrcode-image').src = state.qrcodeImageURL;
    $('#qrcode-image').classList.remove('hidden');
    $('#qrcode-placeholder').classList.add('hidden');
  }

  async function refreshQRCodeSession() {
    if (!state.qrcodeSession?.id) return;
    const previousStatus = state.qrcodeSession.status;
    try {
      state.qrcodeSession = await api(`/api/v1/accounts/qrcode/${encodeURIComponent(state.qrcodeSession.id)}`);
      renderQRCodeSession();
      if (state.qrcodeSession.imageReady && !state.qrcodeImageURL) {
        try { await loadQRCodeImage(state.qrcodeSession.id); }
        catch (error) { if (!qrcodeSessionActive()) toast(error.message, true); }
      }
      if (!qrcodeSessionActive()) {
        clearInterval(state.qrcodePoller);
        state.qrcodePoller = null;
        if (previousStatus !== state.qrcodeSession.status && state.qrcodeSession.status === 'succeeded') {
          state.config = await api('/api/v1/config');
          renderConfig();
          renderAccounts();
          renderDashboard();
          toast('二维码登录成功');
        } else if (previousStatus !== state.qrcodeSession.status && ['failed', 'expired'].includes(state.qrcodeSession.status)) {
          toast(state.qrcodeSession.message, true);
        }
      }
    } catch (error) {
      clearInterval(state.qrcodePoller);
      state.qrcodePoller = null;
      toast(error.message, true);
    }
  }

  function startQRCodePolling() {
    clearInterval(state.qrcodePoller);
    state.qrcodePoller = setInterval(refreshQRCodeSession, 1000);
    refreshQRCodeSession();
  }

  function setAccountMethod(method) {
    state.accountMethod = method;
    $$('#account-login-method button').forEach(button => button.classList.toggle('active', button.dataset.loginMethod === method));
    $('#cookie-login-fields').classList.toggle('hidden', method !== 'cookie');
    $('#qrcode-login-fields').classList.toggle('hidden', method !== 'qrcode');
    $('#cookie-result').classList.toggle('hidden', method !== 'cookie' || !$('#cookie-result').textContent);
    $('#cookie-format').disabled = method !== 'cookie';
    $('#cookie-content').disabled = method !== 'cookie';
    $('#account-submit-label').textContent = method === 'cookie' ? '验证并导入' : '生成二维码';
    $('#account-submit-icon').setAttribute('href', method === 'cookie' ? '#i-user' : '#i-qr');
    renderQRCodeSession();
  }

  function activeDocument() {
    return state.configTarget === 'notify' ? state.notify : state.config;
  }

  function activeEndpoint() {
    return state.configTarget === 'notify' ? '/api/v1/notify' : '/api/v1/config';
  }

  function renderStatus() {
    const status = state.status || {};
    const version = status.version ? String(status.version).split('\n')[0].replace(/^v/i, '') : '';
    const commit = String(status.commit || '').trim();
    const branch = String(status.branch || '').trim();
    const revision = [branch && branch !== 'unknown' ? branch : '', commit && commit !== 'none' ? commit.slice(0, 7) : ''].filter(Boolean).join('/');
    $('#version-label').textContent = version ? `v${version}${revision ? ` (${revision})` : ''}` : 'NCMM';
    $('#build-label').textContent = status.buildTime && status.buildTime !== 'now' ? status.buildTime : '';
    $('#stat-scheduler').textContent = status.schedulerActive ? '调度正常' : '调度启动中';
    $('#stat-scheduler').className = `badge ${status.schedulerActive ? 'success' : 'running'}`;
    $('#stat-timezone').textContent = status.timezone || '--';
    $('#stat-schedules').textContent = status.schedules ?? '--';
    $('#stat-running').textContent = status.running ?? '--';
    $('#stat-storage').textContent = formatBytes(status.logs?.sizeBytes || 0);
    $('#stat-files').textContent = `${status.logs?.files || 0} 个文件`;
    $('#dashboard-uptime').textContent = status.startedAt ? formatUptime(status.startedAt) : '--';
    $('#system-listen-url').textContent = status.listenUrl || '--';
    $('#system-web-url').textContent = status.webUrl || '--';
    $('#system-scheduler').textContent = status.schedulerActive ? '运行中' : '启动中';
    const enabledSchedules = state.schedules.filter(job => job.enabled).length;
    $('#system-schedules').textContent = `${enabledSchedules} / ${status.schedules ?? state.schedules.length}`;
    $('#system-running').textContent = `${status.running ?? 0} / ${status.queued ?? 0}`;
    $('#system-path-executable').textContent = status.paths?.executable || '--';
    $('#system-path-database').textContent = status.paths?.database || '--';
    $('#system-path-config').textContent = status.paths?.config || '--';
    $('#system-path-notify').textContent = status.paths?.notify || '--';
    $('#system-commit').textContent = commit && commit !== 'none' ? `Commit ${commit.slice(0, 7)}` : 'Commit dev';
    $('#system-build').textContent = formatBuildDate(status.buildTime);
    if (version) $('#update-current').textContent = `v${version}`;
    const maxSize = state.settings?.logs?.maxTotalSizeMB;
    $('#system-log-size').textContent = `${formatBytes(status.logs?.sizeBytes || 0)}${maxSize ? ` / ${maxSize} MB` : ''}`;
    $('#logout-button').classList.toggle('hidden', state.auth?.passwordProtectionEnabled === false);
  }

  function configuredAccounts() {
    const accounts = state.config?.data?.accounts || {};
    const mainPath = typeof accounts.main === 'string' && accounts.main.trim()
      ? accounts.main.trim()
      : typeof accounts.primary === 'string' ? accounts.primary.trim() : '';
    const result = [];
    if (mainPath) {
      const profile = latestAccountProfile(mainPath);
      const configured = configuredAccountProfile(mainPath);
      const preferred = preferredAccountMetadata(configured, profile);
      result.push({
        path: mainPath, main: true, profile,
        avatarURL: preferred?.avatarUrl || '',
        label: preferred?.nickname || fileBase(mainPath),
      });
    }
    const secondary = Array.isArray(accounts.secondary) ? accounts.secondary : [];
    secondary.forEach((path, index) => {
      if (typeof path !== 'string' || !path.trim()) return;
      const accountPath = path.trim();
      const profile = latestAccountProfile(accountPath);
      const configured = configuredAccountProfile(accountPath);
      const preferred = preferredAccountMetadata(configured, profile);
      result.push({
        path: accountPath, main: false, sourceIndex: index, profile,
        avatarURL: preferred?.avatarUrl || '',
        label: preferred?.nickname || fileBase(accountPath),
      });
    });
    return result;
  }

  function fileBase(path) {
    return String(path || '').split(/[\\/]/).filter(Boolean).at(-1) || String(path || '');
  }

  function normalizeAccountPath(path) {
    return String(path || '').trim().replaceAll('\\', '/').toLowerCase();
  }

  function accountRewardFrom(rewards, path) {
    const normalized = normalizeAccountPath(path);
    const basename = normalizeAccountPath(fileBase(path));
    return (rewards || []).find(reward => normalizeAccountPath(reward.account) === normalized)
      || (rewards || []).find(reward => normalizeAccountPath(fileBase(reward.account)) === basename);
  }

  function configuredAccountProfile(path) {
    const profiles = state.config?.accountProfiles || {};
    const normalized = normalizeAccountPath(path);
    const basename = normalizeAccountPath(fileBase(path));
    const entry = Object.entries(profiles).find(([candidate]) => normalizeAccountPath(candidate) === normalized)
      || Object.entries(profiles).find(([candidate]) => normalizeAccountPath(fileBase(candidate)) === basename);
    if (entry) return entry[1];
    const names = state.config?.accountNames || {};
    const named = Object.entries(names).find(([candidate]) => normalizeAccountPath(candidate) === normalized)
      || Object.entries(names).find(([candidate]) => normalizeAccountPath(fileBase(candidate)) === basename);
    return named ? { nickname: named[1] } : null;
  }

  function preferredAccountMetadata(configured, observed) {
    if (!configured) return observed;
    if (!observed) return configured;
    const configuredAt = new Date(state.config?.modifiedAt || 0).getTime();
    const observedAt = new Date(observed.observedAt || 0).getTime();
    const primary = configuredAt >= observedAt ? configured : observed;
    const secondary = primary === configured ? observed : configured;
    return {
      nickname: primary.nickname || secondary.nickname || '',
      avatarUrl: primary.avatarUrl || secondary.avatarUrl || '',
    };
  }

  function latestAccountProfile(path) {
    let profile = null;
    let metadataObservedAt = '';
    for (const run of state.runs) {
      const reward = accountRewardFrom(run.rewards, path);
      if (!reward || !(reward.cookieKnown || reward.vipKnown || reward.musicianKnown || reward.nickname || reward.uid || reward.identity)) continue;
      const observedAt = run.finishedAt || run.startedAt || run.triggeredAt;
      if (!profile) profile = { account: reward.account };
      ['uid', 'nickname', 'avatarUrl', 'identity'].forEach(field => {
        if (!profile[field] && reward[field]) {
          profile[field] = reward[field];
          if ((field === 'nickname' || field === 'avatarUrl') && !metadataObservedAt) metadataObservedAt = observedAt;
        }
      });
      [['cookieKnown', 'cookieValid'], ['vipKnown', 'vip'], ['musicianKnown', 'musician']].forEach(([knownField, valueField]) => {
        if (!profile[knownField] && reward[knownField]) {
          profile[knownField] = true;
          profile[valueField] = !!reward[valueField];
        }
      });
    }
    if (!profile) return null;
    profile.observedAt = metadataObservedAt || '';
    return profile;
  }

  function todayAccountRewards() {
    const today = new Date().toDateString();
    const rewards = new Map();
    [...state.runs].reverse().forEach(run => {
      if (new Date(run.triggeredAt || run.startedAt).toDateString() !== today) return;
      (run.rewards || []).forEach(item => {
        const key = normalizeAccountPath(item.account);
        const current = rewards.get(key) || {
          account: item.account, yunbei: 0, yunbeiKnown: false, yunbeiCumulative: false,
          yunbeiBalance: 0, yunbeiBalanceKnown: false,
          growthToday: 0, growthTotal: 0, growthKnown: false,
          effectivePlays: 0, effectiveTarget: 0, effectiveKnown: false,
          cookieKnown: false, cookieValid: false,
          vipKnown: false, vip: false, musicianKnown: false, musician: false, signKnown: false, signed: false,
        };
        if (item.yunbeiKnown) {
          if (item.yunbeiCumulative) {
            current.yunbei = Number(item.yunbei) || 0;
            current.yunbeiCumulative = true;
          } else {
            current.yunbei += Number(item.yunbei) || 0;
          }
          current.yunbeiKnown = true;
        }
        if (item.yunbeiBalanceKnown) {
          current.yunbeiBalance = Number(item.yunbeiBalance) || 0;
          current.yunbeiBalanceKnown = true;
        }
        if (item.growthKnown) {
          current.growthToday = Number(item.growthToday) || 0;
          current.growthTotal = Number(item.growthTotal) || 0;
          current.growthKnown = true;
        }
        if (item.effectiveKnown) {
          current.effectivePlays = Number(item.effectivePlays) || 0;
          current.effectiveTarget = Number(item.effectiveTarget) || 0;
          current.effectiveKnown = true;
        }
        if (item.cookieKnown) {
          current.cookieKnown = true;
          current.cookieValid = !!item.cookieValid;
        }
        if (item.vipKnown) {
          current.vipKnown = true;
          current.vip = !!item.vip;
        }
        if (item.musicianKnown) {
          current.musicianKnown = true;
          current.musician = !!item.musician;
        }
        if (item.signKnown) {
          current.signKnown = true;
          current.signed = !!item.signed;
        }
        ['uid', 'nickname', 'avatarUrl', 'identity'].forEach(field => {
          if (item[field]) current[field] = item[field];
        });
        rewards.set(key, current);
      });
    });
    return [...rewards.values()];
  }

  function localDateKey(value) {
    const date = value instanceof Date ? value : new Date(value);
    if (Number.isNaN(date.getTime())) return '';
    const pad = number => String(number).padStart(2, '0');
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
  }

  function accountPlayStats(path) {
	const normalized = normalizeAccountPath(path);
	const basename = normalizeAccountPath(fileBase(path));
	return (state.playStats || []).find(item => normalizeAccountPath(item.account) === normalized)
	  || (state.playStats || []).find(item => normalizeAccountPath(fileBase(item.account)) === basename);
  }

  function dashboardSeries(account, metric) {
	if (metric === 'plays') {
	  const official = accountPlayStats(account.path);
	  if (official?.points?.length) {
		return official.points.map(point => ({
		  key: point.date,
		  label: String(point.date || '').slice(5),
		  value: Number(point.count) || 0,
		  target: 0,
		  known: !!point.known,
		}));
	  }
	}
    const days = [];
    const start = new Date();
    start.setHours(0, 0, 0, 0);
	start.setDate(start.getDate() - (metric === 'plays' ? 7 : 6));
    for (let offset = 0; offset < 7; offset += 1) {
      const date = new Date(start);
      date.setDate(start.getDate() + offset);
      days.push({ key: localDateKey(date), label: `${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`, value: 0, target: 0, known: false });
    }
    const byKey = new Map(days.map(day => [day.key, day]));
    [...state.runs].reverse().forEach(run => {
      const day = byKey.get(localDateKey(run.triggeredAt || run.startedAt));
      if (!day) return;
      const reward = accountRewardFrom(run.rewards, account.path);
      if (!reward) return;
      if (metric === 'yunbei' && reward.yunbeiKnown) {
        if (reward.yunbeiCumulative) {
          day.value = Number(reward.yunbei) || 0;
        } else {
          day.value += Number(reward.yunbei) || 0;
        }
        day.known = true;
      } else if (metric === 'growth' && reward.growthKnown) {
        day.value = Number(reward.growthToday) || 0;
        day.known = true;
	  }
    });
    return days;
  }

  function chartCoordinate(value) {
    return Number(value.toFixed(2));
  }

  function monotoneCurvePath(points) {
    if (!points.length) return '';
    if (points.length === 1) return `M ${chartCoordinate(points[0].x)} ${chartCoordinate(points[0].y)}`;

    const slopes = points.slice(0, -1).map((point, index) => {
      const next = points[index + 1];
      return (next.y - point.y) / (next.x - point.x);
    });
    const tangents = points.map((_, index) => {
      if (index === 0) return slopes[0];
      if (index === points.length - 1) return slopes.at(-1);
      if (slopes[index - 1] * slopes[index] <= 0) return 0;
      return (slopes[index - 1] + slopes[index]) / 2;
    });

    slopes.forEach((slope, index) => {
      if (slope === 0) {
        tangents[index] = 0;
        tangents[index + 1] = 0;
        return;
      }
      const left = tangents[index] / slope;
      const right = tangents[index + 1] / slope;
      const magnitude = Math.hypot(left, right);
      if (magnitude <= 3) return;
      const scale = 3 / magnitude;
      tangents[index] = scale * left * slope;
      tangents[index + 1] = scale * right * slope;
    });

    let path = `M ${chartCoordinate(points[0].x)} ${chartCoordinate(points[0].y)}`;
    for (let index = 0; index < points.length - 1; index += 1) {
      const point = points[index];
      const next = points[index + 1];
      const width = next.x - point.x;
      path += ` C ${chartCoordinate(point.x + width / 3)} ${chartCoordinate(point.y + tangents[index] * width / 3)}, ${chartCoordinate(next.x - width / 3)} ${chartCoordinate(next.y - tangents[index + 1] * width / 3)}, ${chartCoordinate(next.x)} ${chartCoordinate(next.y)}`;
    }
    return path;
  }

  function metricAreaPath(points, baseline) {
    if (points.length < 2) return '';
    return `${monotoneCurvePath(points)} L ${chartCoordinate(points.at(-1).x)} ${chartCoordinate(baseline)} L ${chartCoordinate(points[0].x)} ${chartCoordinate(baseline)} Z`;
  }

  function renderDashboardChart(accounts) {
    const fallbackAccount = accounts.find(item => item.main) || accounts[0];
    let account = accounts.find(item => normalizeAccountPath(item.path) === normalizeAccountPath(state.dashboardAccountPath));
    if (!account) account = fallbackAccount;
    state.dashboardAccountPath = account?.path || '';
    const labels = { plays: '有效播放量', growth: 'VIP 成长值', yunbei: '云贝收益' };
    $('#dashboard-metric-title').textContent = `最近 7 日${labels[state.dashboardMetric]}`;
    const selector = $('#dashboard-account-select');
    selector.innerHTML = accounts.map(item => `<option value="${escapeHTML(item.path)}"${item.path === state.dashboardAccountPath ? ' selected' : ''}>${escapeHTML(item.label)}${item.main ? '（主账号）' : ''}</option>`).join('');
    selector.disabled = accounts.length < 2;
    selector.classList.toggle('hidden', !accounts.length);
    if (!account) {
      $('#dashboard-chart').className = 'metric-empty';
      $('#dashboard-chart').innerHTML = `${icon('dashboard')}<strong>暂无已配置账号</strong>`;
      return;
    }
    const series = dashboardSeries(account, state.dashboardMetric);
    if (!series.some(item => item.known)) {
      const copy = { plays: '等待音乐人任务同步', growth: '等待 VIP 任务同步', yunbei: '等待云贝任务同步' }[state.dashboardMetric];
      $('#dashboard-chart').className = 'metric-empty';
      $('#dashboard-chart').innerHTML = `${icon('dashboard')}<strong>${copy}</strong>`;
      return;
    }
    const maximum = Math.max(1, ...series.filter(item => item.known).map(item => item.value));
    const chart = $('#dashboard-chart');
    chart.className = 'metric-chart';
    const chartWidth = Math.max(620, Math.round(chart.clientWidth || 700));
    const viewportHeight = Math.max(190, Math.round(chart.clientHeight || 190));
    const chartLeft = 50;
    const chartRight = chartWidth - 18;
    const chartTop = 24;
    const chartBottom = viewportHeight - 34;
    const plotHeight = chartBottom - chartTop;
    const points = series.map((item, index) => ({
      ...item,
      x: chartLeft + index * ((chartRight - chartLeft) / 6),
      y: chartBottom - (item.value / maximum * plotHeight),
    }));
    const segments = [];
    let segment = [];
    points.forEach(point => {
      if (point.known) segment.push(point);
      else if (segment.length) { segments.push(segment); segment = []; }
    });
    if (segment.length) segments.push(segment);
    const grid = [0, 1, 2, 3].map(index => {
      const y = chartTop + index * (plotHeight / 3);
      const value = Math.round(maximum * (1 - index / 3));
      return `<line class="metric-line-grid" x1="${chartLeft}" y1="${y}" x2="${chartRight}" y2="${y}"></line><text class="metric-line-axis" x="42" y="${y + 4}" text-anchor="end">${value}</text>`;
    }).join('');
    const areas = segments.map(pointsInSegment => {
      const path = metricAreaPath(pointsInSegment, chartBottom);
      return path ? `<path class="metric-area-path" d="${path}"></path>` : '';
    }).join('');
    const paths = segments.map(pointsInSegment => `<path class="metric-line-path" d="${monotoneCurvePath(pointsInSegment)}"></path>`).join('');
    const knownPoints = points.filter(item => item.known);
    const lastKnownIndex = points.findLastIndex(item => item.known);
    const average = knownPoints.reduce((total, item) => total + item.value, 0) / knownPoints.length;
    const averageY = chartBottom - (average / maximum * plotHeight);
    const averageDisplay = Number.isInteger(average) ? String(average) : average.toFixed(1);
    const averageLabelY = Math.min(chartBottom - 8, Math.max(chartTop + 12, averageY - 7));
    const averageLine = `<line class="metric-average-line" x1="${chartLeft}" y1="${chartCoordinate(averageY)}" x2="${chartRight}" y2="${chartCoordinate(averageY)}"></line><text class="metric-average-label" x="${chartRight - 4}" y="${chartCoordinate(averageLabelY)}" text-anchor="end">近 7 日均值 ${averageDisplay}</text>`;
    const markers = points.map((item, index) => {
      const display = item.known ? String(item.value) : '--';
      const detail = state.dashboardMetric === 'plays' && item.known && item.target ? `${item.value} / ${item.target}` : display;
      return `<g class="metric-line-point${index === lastKnownIndex ? ' latest' : ''}" aria-label="${escapeHTML(`${item.key}：${detail}`)}"><text class="metric-line-value" x="${item.x}" y="${item.known ? Math.max(14, item.y - 10) : chartBottom - 8}" text-anchor="middle">${escapeHTML(display)}</text>${item.known ? `<circle cx="${item.x}" cy="${item.y}" r="4"></circle>` : ''}<text class="metric-line-label" x="${item.x}" y="${viewportHeight - 8}" text-anchor="middle">${item.label}</text><title>${escapeHTML(`${item.key}：${detail}`)}</title></g>`;
    }).join('');
    chart.dataset.chartSize = `${chartWidth}x${viewportHeight}`;
    chart.innerHTML = `<svg class="metric-line-svg" viewBox="0 0 ${chartWidth} ${viewportHeight}" role="img" aria-label="${escapeHTML(`${account.label}最近 7 日${labels[state.dashboardMetric]}`)}"><defs><linearGradient id="metric-area-gradient" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" class="metric-area-start"></stop><stop offset="100%" class="metric-area-end"></stop></linearGradient></defs>${grid}${areas}${averageLine}${paths}${markers}</svg>`;
  }

  function renderDashboard() {
    const accounts = configuredAccounts();
    const dailyRewards = todayAccountRewards();
    const mainCount = accounts.filter(account => account.main).length;
    const secondaryCount = accounts.length - mainCount;
    const today = new Date().toDateString();
    const todayRuns = state.runs.filter(run => new Date(run.triggeredAt || run.startedAt).toDateString() === today);
    const successCount = todayRuns.filter(run => run.status === 'success').length;
    const failedCount = todayRuns.filter(run => run.status === 'failed').length;
    $('#stat-today-runs').textContent = `${successCount} 成功 · ${failedCount} 失败`;
    $('#stat-today-total').textContent = `共 ${successCount + failedCount} 次`;
    const accountHealth = accounts.map(account => latestAccountProfile(account.path));
    const validAccounts = accountHealth.filter(profile => profile?.cookieKnown && profile.cookieValid).length;
    const invalidAccounts = accountHealth.filter(profile => profile?.cookieKnown && !profile.cookieValid).length;
    const unknownAccounts = accounts.length - validAccounts - invalidAccounts;
    $('#stat-account-health').textContent = `${validAccounts} 有效 · ${invalidAccounts} 失效${unknownAccounts ? ` · ${unknownAccounts} 未知` : ''}`;
    $('#stat-account-issues').textContent = !accounts.length ? '暂无账号' : invalidAccounts ? `${invalidAccounts} 个账号需处理` : unknownAccounts ? '部分账号等待同步' : '全部账号可用';
    $('#stat-account-issues').className = `badge-pill ${invalidAccounts ? 'primary' : unknownAccounts ? 'blue' : 'success'}`;
    $('#stat-accounts').textContent = `${mainCount} 主 + ${secondaryCount} 辅`;
    $('#stat-account-detail').textContent = `${mainCount} 个主账号，${secondaryCount} 个辅助账号`;
    $('#account-nav-count').textContent = accounts.length;

    const upcoming = state.schedules.filter(x => x.enabled && x.nextRuns?.length).sort((a, b) => new Date(a.nextRuns[0]) - new Date(b.nextRuns[0])).slice(0, 5);
    $('#dashboard-next-run').textContent = upcoming[0]?.nextRuns?.[0] ? formatDate(upcoming[0].nextRuns[0]) : '暂无计划';
    $('#upcoming-list').innerHTML = upcoming.length ? upcoming.map(job => `<tr>
      <td><strong>${escapeHTML(job.name)}</strong></td>
      <td><code class="dashboard-command">${escapeHTML(job.command)}${job.args?.length ? ` ${escapeHTML(job.args.join(' '))}` : ''}</code></td>
      <td><code>${escapeHTML(job.cron)}</code></td>
      <td>${formatDate(job.nextRuns[0])}</td>
      <td><span class="badge-pill ${job.running || job.queued ? 'blue' : 'success'}">${job.running ? '运行中' : job.queued ? '排队中' : '空闲中'}</span></td>
    </tr>`).join('') : '<tr><td colspan="5" class="empty-state">暂无定时任务</td></tr>';
    const recent = state.runs.slice(0, 5);
    $('#recent-runs').className = `compact-list${recent.length ? '' : ' empty-state'}`;
    $('#recent-runs').innerHTML = recent.length ? recent.map(run => `<div class="compact-row"><strong>${escapeHTML(run.jobName)}</strong><span>${escapeHTML(run.command)} ${escapeHTML((run.args || []).join(' '))}</span><span class="badge-pill ${runBadgeClass(run.status)}">${escapeHTML(statusLabel(run.status))}</span></div>`).join('') : '暂无运行记录';

    $('#account-rewards-list').className = `reward-list${accounts.length ? '' : ' empty-state'}`;
    $('#account-rewards-list').innerHTML = accounts.length ? accounts.map(account => {
      const reward = accountRewardFrom(dailyRewards, account.path);
      const yunbei = reward?.yunbeiKnown ? `+${reward.yunbei}` : '--';
      const yunbeiBalance = reward?.yunbeiBalanceKnown ? reward.yunbeiBalance : '--';
      const growth = reward?.growthKnown ? `${reward.growthToday} / ${reward.growthTotal}` : '--';
      const growthPart = reward?.vipKnown && !reward.vip ? '' : `<span class="reward-separator">丨</span><span>成长值 ${growth}</span>`;
      return `<div class="reward-row"><div class="reward-account"><strong>${escapeHTML(account.label)}</strong><small>(${escapeHTML(fileBase(account.path))})</small></div><div class="reward-summary"><span>云贝 ${yunbei} / ${yunbeiBalance}</span>${growthPart}</div></div>`;
    }).join('') : '暂无已配置账号';
    const rewardsKnown = dailyRewards.some(reward => reward.yunbeiKnown || reward.growthKnown);
    $('#reward-sync-badge').textContent = rewardsKnown ? '已同步' : '等待同步';
    $('#reward-sync-badge').className = `badge-pill ${rewardsKnown ? 'success' : 'primary'}`;
    renderDashboardChart(accounts);
  }

  function renderConfig() {
    const document = activeDocument();
    const parseError = $('#config-parse-error');
    updateConfigDirtyUI();
    $('#config-search').value = state.configSearch;
    $$('#config-target button').forEach(button => button.classList.toggle('active', button.dataset.target === state.configTarget));
    $('#config-sidebar-title').textContent = state.configTarget === 'notify' ? '推送通道' : '配置模块';
    $('#notify-test').classList.toggle('hidden', state.configTarget !== 'notify');
    $('#page-config').classList.toggle('notify-config-active', state.configTarget === 'notify');
    $('#config-presets-bar')?.classList.toggle('hidden', state.configTarget !== 'config');
    const saveActions = $('#config-save-actions');
    const saveActionsHost = state.configTarget === 'notify' ? $('.config-view-actions') : $('#config-preset-actions');
    if (saveActions && saveActionsHost && saveActions.parentElement !== saveActionsHost) saveActionsHost.appendChild(saveActions);
    if ($('#badge-count-config')) $('#badge-count-config').textContent = '15';
    if ($('#badge-count-notify')) $('#badge-count-notify').textContent = '9';
    $$('#preset-chips-list button').forEach(button => button.classList.toggle('active', button.dataset.preset === (state.currentPreset || 'novice')));

    if (!document) {
      parseError.textContent = '该配置暂时无法加载，请稍后重试。';
      parseError.classList.remove('hidden');
      return;
    }
    parseError.textContent = document.parseError ? `YAML 解析失败，可在源码模式修复后保存：${document.parseError}` : '';
    parseError.classList.toggle('hidden', !document.parseError);
    if (document.parseError) state.configMode = 'yaml';
    $$('#config-mode button').forEach(button => button.classList.toggle('active', button.dataset.mode === state.configMode));
    $('#config-visual').classList.toggle('hidden', state.configMode !== 'visual');
    $('#config-yaml').classList.toggle('hidden', state.configMode !== 'yaml');
    $('#config-search-wrap').classList.toggle('hidden', state.configMode !== 'visual');
    if (!document.data) {
      $('#config-sections').innerHTML = '';
      $('#config-form').innerHTML = '';
      updateYAMLEditor(document.raw || '', document.parseError || '');
      return;
    }
    const root = document.data;
    const targetSchema = activeConfigSchema();
    const schemaOrder = (targetSchema?.categories || []).map(category => category.id);
    const rootKeys = Object.keys(root).filter(key => state.configTarget !== 'config' || key !== 'version');
    const keys = [...schemaOrder.filter(key => rootKeys.includes(key)), ...rootKeys.filter(key => !schemaOrder.includes(key))];
    if (!state.configSection || !keys.includes(state.configSection)) {
      state.configSection = state.configTarget === 'config' && keys.includes('task') ? 'task' : keys[0] || '';
    }
    $('#config-section-count').textContent = state.configTarget === 'config' ? `${keys.length} 模块` : `${keys.length} 通道`;
    const category = schemaCategory(state.configSection);
    const continuousFlow = state.configTarget === 'config' && state.configMode === 'visual';
    if (continuousFlow) {
      $('#config-current-title').innerHTML = `${icon('settings')}<span><strong>规则配置面板</strong></span>`;
      $('#config-sections').innerHTML = keys.map((key, index) => renderConfigSidebarItem(key, index)).join('');
      $('#config-form').className = 'config-form config-flow-form';
      $('#config-form').innerHTML = keys.map((key, index) => renderConfigFlowSection(key, index)).join('');
      applyConfigSearch(state.configSearch);
      updateConfigFlowActiveSection();
    } else {
      const showKey = state.configTarget !== 'notify' && state.configSection;
      const currentTitle = category?.title || sectionLabels[state.configSection] || state.configSection || '配置详情';
      $('#config-current-title').innerHTML = `${icon('settings')}<span><strong>${escapeHTML(currentTitle)}${showKey ? ` <small>(${escapeHTML(state.configSection)})</small>` : ''}</strong><small id="config-current-description">${escapeHTML(category?.description || '')}</small></span>`;
      $('#config-sections').innerHTML = keys.map((key, index) => renderConfigSidebarItem(key, index)).join('');
      $('#config-form').className = 'config-form';
      $('#config-form').innerHTML = renderConfigSection(state.configSection, root[state.configSection]);
    }
    const yaml = !document.parseError ? document.sections?.[state.configSection] || '' : document.raw || '';
    const yamlFile = state.configTarget === 'notify' ? 'notify.yaml' : 'config.yaml';
    $('#yaml-file-label').textContent = `${yamlFile} · ${sectionLabels[state.configSection] || state.configSection}`;
    updateYAMLEditor(yaml, document.parseError || '');
  }

  function renderConfigSidebarItem(key, index) {
    const category = schemaCategory(key);
    const flow = configFlowMeta[key];
    const title = state.configTarget === 'config' ? flow?.nav || category?.title : category?.title || sectionLabels[key] || key;
    const number = state.configTarget === 'config' ? `${index + 1}. ` : '';
    return `<button type="button" data-section="${escapeHTML(key)}" class="sim-category-item ${key === state.configSection ? 'active' : ''}"><span class="sim-category-item-left"><span>${escapeHTML(number + title)}</span></span><small>${escapeHTML(key)}</small></button>`;
  }

  function updateYAMLEditor(raw, parseError = '') {
    const editor = $('#yaml-editor');
    editor.value = raw;
    renderYAMLEditor(parseError);
  }

  function renderYAMLEditor(parseError = '') {
    const editor = $('#yaml-editor');
    const raw = editor.value;
    const lines = raw.split('\n');
    $('#yaml-line-numbers').textContent = lines.map((_, index) => index + 1).join('\n');
    $('#yaml-highlight').innerHTML = lines.map(highlightYAMLLine).join('\n') + '\n';
    const validation = $('#yaml-validation');
    const tabLine = lines.findIndex(line => line.includes('\t'));
    const invalid = parseError || (tabLine >= 0 ? `第 ${tabLine + 1} 行包含 Tab 缩进` : '');
    validation.textContent = invalid ? `格式错误：${invalid}` : state.configDirty ? '待保存校验' : '格式有效';
    validation.className = invalid ? 'invalid' : state.configDirty ? 'pending' : 'valid';
    updateYAMLCursor();
  }

  function highlightYAMLLine(line) {
    if (!line) return '';
    let inQuote = null;
    let commentIdx = -1;
    for (let i = 0; i < line.length; i++) {
      const ch = line[i];
      if ((ch === '"' || ch === "'") && (i === 0 || line[i - 1] !== '\\')) {
        if (!inQuote) inQuote = ch;
        else if (inQuote === ch) inQuote = null;
      } else if (ch === '#' && !inQuote && (i === 0 || /\s/.test(line[i - 1]))) {
        commentIdx = i;
        break;
      }
    }

    const codePart = commentIdx >= 0 ? line.slice(0, commentIdx) : line;
    const commentPart = commentIdx >= 0 ? line.slice(commentIdx) : '';

    let html = '';
    const match = codePart.match(/^(\s*)(-\s+)?(.*)$/);
    if (match) {
      const indent = match[1];
      const bullet = match[2];
      const rest = match[3];

      html += escapeHTML(indent);
      if (bullet) {
        html += `<span class="yaml-bullet">${escapeHTML(bullet)}</span>`;
      }

      if (rest) {
        const kvMatch = rest.match(/^((?:["'][^"']+["']|[^:#\s][^:#]*?))(\s*:\s*)(.*)$/);
        if (kvMatch) {
          const key = kvMatch[1];
          const colon = kvMatch[2];
          const val = kvMatch[3];
          html += `<span class="yaml-key">${escapeHTML(key)}</span><span class="yaml-punct">${escapeHTML(colon)}</span>`;
          if (val) {
            html += highlightYAMLToken(val);
          }
        } else {
          html += highlightYAMLToken(rest);
        }
      }
    } else {
      html += escapeHTML(codePart);
    }

    if (commentPart) {
      html += `<span class="yaml-comment">${escapeHTML(commentPart)}</span>`;
    }
    return html;
  }

  function highlightYAMLToken(rawVal) {
    const trimmed = rawVal.trim();
    if (!trimmed) return escapeHTML(rawVal);
    const leadingSpaces = rawVal.slice(0, rawVal.indexOf(trimmed));
    const trailingSpaces = rawVal.slice(rawVal.indexOf(trimmed) + trimmed.length);

    let tokenHtml = '';
    if (/^(true|false|yes|no|on|off)$/i.test(trimmed)) {
      tokenHtml = `<span class="yaml-boolean">${escapeHTML(trimmed)}</span>`;
    } else if (/^(null|~)$/i.test(trimmed)) {
      tokenHtml = `<span class="yaml-null">${escapeHTML(trimmed)}</span>`;
    } else if (/^(['"]).*\1$/.test(trimmed)) {
      tokenHtml = `<span class="yaml-string">${escapeHTML(trimmed)}</span>`;
    } else if (/^-?\d+(\.\d+)*$/.test(trimmed)) {
      tokenHtml = `<span class="yaml-number">${escapeHTML(trimmed)}</span>`;
    } else {
      tokenHtml = `<span class="yaml-value">${escapeHTML(trimmed)}</span>`;
    }

    return escapeHTML(leadingSpaces) + tokenHtml + escapeHTML(trailingSpaces);
  }

  function updateYAMLCursor() {
    const editor = $('#yaml-editor');
    const before = editor.value.slice(0, editor.selectionStart);
    const lines = before.split('\n');
    $('#yaml-cursor').textContent = `Ln ${lines.length}, Col ${lines.at(-1).length + 1}`;
  }

  function renderAccounts() {
    const accounts = configuredAccounts();
    const dailyRewards = todayAccountRewards();
    const mainCount = accounts.filter(account => account.main).length;
    const secondaryCount = accounts.length - mainCount;
    $('#account-list-title').textContent = `托管账号列表 (${mainCount} 主 + ${secondaryCount} 辅)`;
    const tbody = $('#accounts-table');
    if (!accounts.length) {
      tbody.innerHTML = '<tr><td colspan="8" class="empty-state">暂无已配置账号，请先添加账号</td></tr>';
      return;
    }
    tbody.innerHTML = accounts.map((account, index) => {
      const filename = fileBase(account.path);
      const profile = account.profile;
      const reward = accountRewardFrom(dailyRewards, account.path);
      const initial = Array.from(account.label || filename)[0] || '账';
      const avatarURL = safeAvatarURL(account.avatarURL);
      const avatar = `<span class="account-avatar${account.main ? ' main' : ''}"><span>${escapeHTML(initial)}</span>${avatarURL ? `<img src="${escapeHTML(avatarURL)}" alt="" loading="lazy" referrerpolicy="no-referrer">` : ''}</span>`;
      const cookieStatus = profile?.cookieKnown
        ? `<span class="badge-pill ${profile.cookieValid ? 'success' : 'primary'}">${profile.cookieValid ? '有效' : '失效'}</span>`
        : '<span class="badge-pill gray">未知</span>';
      const yunbei = reward?.yunbeiKnown || reward?.yunbeiBalanceKnown
        ? `${reward?.yunbeiKnown ? `+${reward.yunbei}` : '--'} / ${reward?.yunbeiBalanceKnown ? reward.yunbeiBalance : '--'}`
        : '-- / --';
      const growth = reward?.growthKnown ? `${reward.growthToday} / ${reward.growthTotal}` : '-- / --';
      const signStatus = reward?.signKnown
        ? `<span class="badge-pill ${reward.signed ? 'success' : 'primary'}">${reward.signed ? '已打卡' : '失败'}</span>`
        : '<span class="badge-pill gray">未同步</span>';
      const identityParts = [];
      if (profile?.identity) identityParts.push(`<span class="muted">${escapeHTML(profile.identity)}</span>`);
      if (profile?.musicianKnown && profile.musician) identityParts.push('<span class="badge-pill primary">音乐人</span>');
      const accountIdentity = identityParts.length ? identityParts.join('') : '<span class="muted">待任务同步</span>';
      return `<tr>
        <td><div class="account-identity">${avatar}<div><strong>${escapeHTML(account.label)}</strong><small>UID: ${escapeHTML(profile?.uid || '待任务同步')}</small></div></div></td>
        <td><div class="cookie-cell"><span>${escapeHTML(filename)}</span>${cookieStatus}</div></td>
        <td><span class="badge-pill ${account.main ? 'primary' : 'cyan'}">${account.main ? '主账号' : '辅助号'}</span></td>
		<td><div class="account-type-tags">${accountIdentity}</div></td>
        <td class="muted">${yunbei}</td>
        <td class="muted">${growth}</td>
        <td>${signStatus}</td>
        <td><div class="row-actions"><button class="icon-button" data-account-action="edit" data-index="${index}" title="编辑账号">${icon('edit')}</button><button class="icon-button danger-icon" data-account-action="delete" data-index="${index}" title="删除账号">${icon('trash')}</button></div></td>
      </tr>`;
    }).join('');
    $$('.account-avatar img', tbody).forEach(image => image.addEventListener('error', () => image.remove(), { once: true }));
  }

  function openAccountPanel(account = null) {
    const activeQRCode = qrcodeSessionActive();
    state.accountEditing = account;
    $('#accounts-layout').classList.add('panel-open');
    $('#account-panel').classList.remove('hidden');
    $('#account-panel').classList.add('open');
    $('#account-panel-title').textContent = account ? `编辑账号 · ${account.label}` : '登录 / 添加账号';
    $('#account-target-fields').classList.toggle('hidden', !!account);
    if (account) {
      state.accountMain = account.main;
      $('#account-filename').value = fileBase(account.path);
    } else if (!activeQRCode) {
      state.accountMain = false;
      $('#account-filename').value = 'fan1.json';
      $('#cookie-content').value = '';
      $('#cookie-result').textContent = '';
      $('#cookie-result').classList.add('hidden');
    }
    $('#account-filename').readOnly = !!account;
    $$('.account-type button').forEach(button => {
      button.classList.toggle('active', (button.dataset.main === 'true') === state.accountMain);
      button.disabled = !!account || activeQRCode;
    });
    setAccountMethod(state.accountMethod);
  }

  function closeAccountPanel() {
    if (qrcodeSessionActive()) {
      toast('请先完成或取消当前二维码登录', true);
      return;
    }
    state.accountEditing = null;
    $('#account-filename').readOnly = false;
    $('#account-target-fields').classList.remove('hidden');
    $('#accounts-layout').classList.remove('panel-open');
    $('#account-panel').classList.add('hidden');
    $('#account-panel').classList.remove('open');
  }

  async function deleteConfiguredAccount(account) {
    if (!confirm(`从任务配置中移除“${account.label}”？Cookie 文件会保留。`)) return;
    const data = structuredClone(state.config.data);
    data.accounts ||= {};
    if (account.main) {
      if (typeof data.accounts.main === 'string' && data.accounts.main.trim()) data.accounts.main = '';
      else data.accounts.primary = '';
    } else if (Array.isArray(data.accounts.secondary)) {
      data.accounts.secondary.splice(account.sourceIndex, 1);
    }
    state.config = await api('/api/v1/config', { method: 'PUT', body: { revision: state.config.revision, data } });
    state.configDirty = false;
    state.configDirtyMode = '';
    renderConfig();
    renderAccounts();
    renderDashboard();
    toast('账号已从任务配置移除，Cookie 文件未删除');
  }

  function activeConfigSchema() {
    return state.configSchema?.targets?.[state.configTarget] || null;
  }

  function schemaCategory(id) {
    return activeConfigSchema()?.categories?.find(category => category.id === id) || null;
  }

  function valueAtPath(root, path) {
    let cursor = root;
    for (const key of path) {
      if (cursor === null || typeof cursor !== 'object' || !Object.prototype.hasOwnProperty.call(cursor, key)) {
        return { exists: false, value: undefined };
      }
      cursor = cursor[key];
    }
    return { exists: true, value: cursor };
  }

  function flowFieldInfo(path) {
    const field = activeConfigSchema()?.fields?.find(item => item.path === path);
    if (!field) return null;
    const keys = path.split('.');
    const located = valueAtPath(activeDocument().data, keys);
    if (!located.exists && !Object.prototype.hasOwnProperty.call(field, 'default')) return null;
    return { field, keys, value: located.exists ? located.value : field.default };
  }

  function renderFlowField(path, options = {}) {
    const info = flowFieldInfo(path);
    if (!info) return '';
    const key = info.keys.at(-1);
    const title = options.title || `${info.field.title} (${key})`;
    const description = options.description === undefined ? info.field.description : options.description;
    const control = options.control || renderSchemaControl(info.field, info.keys, info.value);
    const switchField = info.field.widget === 'switch' && !options.forceStack;
    if (switchField) {
      return `<div class="form-row flow-switch-row"><div class="form-row-label"><span>${escapeHTML(title)}</span>${control}</div>${description ? `<p class="form-row-desc">${escapeHTML(description)}</p>` : ''}</div>`;
    }
    return `<div class="form-row"><label class="form-row-label">${escapeHTML(title)}</label>${control}${description ? `<p class="form-row-desc">${escapeHTML(description)}</p>` : ''}</div>`;
  }

  function renderFlowFields(paths) {
    return paths.map(path => renderFlowField(path)).join('');
  }

  function renderFlowCard(title, content, badge = '', tone = 'gray', className = '') {
    const header = title || badge ? `<header class="config-card-header"><strong class="config-card-title">${escapeHTML(title)}</strong>${badge ? `<span class="badge-pill ${escapeHTML(tone)}">${escapeHTML(badge)}</span>` : ''}</header>` : '';
    return `<div class="config-card-box ${escapeHTML(className)}">${header}${content}</div>`;
  }

  function renderFlowRange(label, minPath, maxPath, unit, description = '') {
    const minInfo = flowFieldInfo(minPath);
    const maxInfo = flowFieldInfo(maxPath);
    if (!minInfo || !maxInfo) return '';
    const minEncoded = escapeHTML(JSON.stringify(minInfo.keys));
    const maxEncoded = escapeHTML(JSON.stringify(maxInfo.keys));
    return `<div class="form-row"><label class="form-row-label">${escapeHTML(label)}</label><div class="range-inputs-row"><input class="config-control range-input-box" data-path='${minEncoded}' data-kind="number" type="number" value="${escapeHTML(minInfo.value)}" min="${minInfo.field.min ?? 0}"><span class="range-divider">至</span><input class="config-control range-input-box" data-path='${maxEncoded}' data-kind="number" type="number" value="${escapeHTML(maxInfo.value)}" min="${maxInfo.field.min ?? 0}"><span class="range-unit-text">${escapeHTML(unit)}</span></div>${description ? `<p class="form-row-desc">${escapeHTML(description)}</p>` : ''}</div>`;
  }

  function renderFlowDuration(path, label) {
    const info = flowFieldInfo(path);
    if (!info) return '';
    const numeric = Number.parseFloat(String(info.value)) || 0;
    const encoded = escapeHTML(JSON.stringify(info.keys));
    const presets = (info.field.presets || []).map(preset => `<button type="button" class="quick-pill" data-schema-path='${encoded}' data-schema-value='${escapeHTML(JSON.stringify(preset))}'>${escapeHTML(preset)}</button>`).join('');
    return `<div class="form-row"><label class="form-row-label">${escapeHTML(label)}</label><div class="input-affix-wrap"><div class="input-with-unit"><input class="config-control form-control-input" data-path='${encoded}' data-kind="duration-seconds" type="number" value="${numeric}" min="0"><span class="input-unit-label">秒</span></div><div class="quick-preset-pills">${presets}</div></div></div>`;
  }

  function renderFlowAccounts() {
    const root = activeDocument().data;
    const accounts = root.accounts || {};
    const paths = [accounts.main, ...(Array.isArray(accounts.secondary) ? accounts.secondary : []), ...Object.keys(accounts.antiCheatTokens || {})].filter(Boolean);
    const uniquePaths = [...new Set(paths)];
    const tokens = accounts.antiCheatTokens && typeof accounts.antiCheatTokens === 'object' ? accounts.antiCheatTokens : {};
    const tokenRows = uniquePaths.map((cookiePath, index) => `<div class="form-row"><label class="form-row-label">${index === 0 ? '主账号' : `辅助号 ${index}`} Token (${escapeHTML(cookiePath)})</label><input type="password" class="config-token-control form-control-input" data-token-cookie="${escapeHTML(cookiePath)}" value="${escapeHTML(tokens[cookiePath] || '')}" placeholder="移动端抓包获取，未配置时自动跳过相关任务" autocomplete="off"></div>`).join('');
    const cookieCard = renderFlowCard('Cookie 路径配置', `${renderFlowField('accounts.main', { title: '主账号 Cookie 路径 (main)', description: '' })}${renderFlowField('accounts.secondary', { title: '辅助账号列表 (secondary)', description: '支持多个小号接力，用于做任务与刷播放量。' })}`, '基础凭据', 'cyan');
    const tokenCard = renderFlowCard('移动端 X-antiCheatToken 凭证', tokenRows || '<p class="form-row-desc">请先配置账号 Cookie 文件。</p>', '推歌 / 领会员必备', 'amber');
    return `<div class="config-cards-grid">${cookieCard}${tokenCard}</div>`;
  }

  function renderFlowNetwork() {
    const networkCard = renderFlowCard('网络通信参数', `${renderFlowField('network.debug', { title: '开启 HTTP 调试日志 (debug)', description: '输出详细网络报文，排查接口故障时开启。' })}${renderFlowDuration('network.timeout', '全局请求超时 (timeout)')}${renderFlowField('network.retry', { title: '网络失败重试 (retry)', description: '' })}`);
    const uaCard = renderFlowCard('多端 User-Agent 伪装', renderFlowFields(['network.user_agent.weapi', 'network.user_agent.eapi', 'network.user_agent.xeapi']));
    return `<div class="config-cards-grid">${networkCard}${uaCard}</div>`;
  }

  const ALL_TASK_SUBTASKS = [
    { id: 'daily-song-share', name: '每日推歌与抽奖', duration: '', badge: 'cyan' },
    { id: 'VipTask', name: '黑胶 VIP 日常任务', duration: '', badge: 'success' },
    { id: 'Reserve', name: '预约新歌领云贝', duration: '', badge: 'success' },
    { id: 'ViewVipCenter', name: '浏览会员中心', duration: '', badge: 'success' },
    { id: 'LikeComment', name: '点赞评论与动态', duration: '', badge: 'success' },
    { id: 'FollowArtist', name: '关注推荐歌手', duration: '', badge: 'success' },
    { id: 'LikeSong', name: '红心歌曲', duration: '', badge: 'success' },
    { id: 'CollectSong', name: '收藏歌单歌曲', duration: '', badge: 'success' },
    { id: 'PublishNote', name: '发布图文动态', duration: '', badge: 'success' },
    { id: 'musician-sign', name: '音乐人每日签到', duration: '', badge: 'amber' },
    { id: 'note', name: '公共图文笔记', duration: '', badge: 'primary' },
    { id: 'vip-member-gift', name: '黑胶会员赠礼/领礼', duration: '', badge: 'cyan' },
    { id: 'fansgroup', name: '乐迷团日常打卡', duration: '', badge: 'amber' },
    { id: 'ListenIndie', name: '探索小众歌曲', duration: '约 7 分钟', badge: 'purple' },
    { id: 'PlayDailyRecommend', name: '日推听歌完成任务', duration: '约 30~45 分钟', badge: 'purple' },
    { id: 'playids', name: '播放指定歌曲有效量', duration: '长耗时 (按目标)', badge: 'cyan' },
    { id: 'musician-vip', name: '音乐人 VIP 进阶', duration: '长耗时 (月度进阶)', badge: 'primary' },
  ];

  const DEFAULT_FAST_TASKS = [
    'daily-song-share', 'VipTask', 'Reserve', 'ViewVipCenter', 'LikeComment',
    'FollowArtist', 'LikeSong', 'CollectSong', 'PublishNote', 'musician-sign',
    'note', 'vip-member-gift', 'fansgroup'
  ];

  const DEFAULT_SLOW_TASKS = [
    'ListenIndie', 'PlayDailyRecommend', 'playids', 'musician-vip'
  ];

  let draggedTaskId = null;
  let draggedTaskGroup = null;
  let dragInsertPosition = null;

  function getTaskSubtaskInfo(id) {
    return ALL_TASK_SUBTASKS.find(s => s.id.toLowerCase() === id.toLowerCase()) || {
      id: id,
      name: id,
      duration: '',
      badge: 'gray'
    };
  }

  function getTaskQueueLists() {
    const root = activeDocument().data;
    const fastTasks = Array.isArray(root?.task?.fast_tasks) ? [...root.task.fast_tasks] : [...DEFAULT_FAST_TASKS];
    const slowTasks = Array.isArray(root?.task?.slow_tasks) ? [...root.task.slow_tasks] : [...DEFAULT_SLOW_TASKS];
    return { fastTasks, slowTasks };
  }

  function setTaskQueueLists(fastTasks, slowTasks) {
    const root = activeDocument().data;
    root.task ||= {};
    root.task.fast_tasks = fastTasks;
    root.task.slow_tasks = slowTasks;
    markConfigDirty();
    const scrollTop = $('#config-visual')?.scrollTop || 0;
    renderConfig();
    if ($('#config-visual')) $('#config-visual').scrollTop = scrollTop;
  }

  function moveTaskToSlow(taskId) {
    const { fastTasks, slowTasks } = getTaskQueueLists();
    const nextFast = fastTasks.filter(id => id !== taskId);
    const nextSlow = slowTasks.filter(id => id !== taskId);
    nextSlow.push(taskId);
    setTaskQueueLists(nextFast, nextSlow);
  }

  function moveTaskToFast(taskId) {
    const { fastTasks, slowTasks } = getTaskQueueLists();
    const nextSlow = slowTasks.filter(id => id !== taskId);
    const nextFast = fastTasks.filter(id => id !== taskId);
    nextFast.push(taskId);
    setTaskQueueLists(nextFast, nextSlow);
  }

  function resetTaskQueueDefault() {
    setTaskQueueLists([...DEFAULT_FAST_TASKS], [...DEFAULT_SLOW_TASKS]);
  }

  function renderFlowTaskKanban(fastTasks, slowTasks) {
    const renderCol = (group, title, list, isFast) => {
      const items = list.map((id, idx) => {
        const info = getTaskSubtaskInfo(id);
        const durationBadge = info.duration ? `<span class="badge-pill ${escapeHTML(info.badge)}" style="font-size: 10px;">${escapeHTML(info.duration)}</span>` : '';
        const transferBtn = isFast
          ? `<button type="button" class="kanban-transfer-btn" data-task-transfer="slow" data-task-id="${escapeHTML(id)}" title="移至慢任务"><span>移至慢任务</span><span>→</span></button>`
          : `<button type="button" class="kanban-transfer-btn" data-task-transfer="fast" data-task-id="${escapeHTML(id)}" title="移至快任务"><span>←</span><span>移至快任务</span></button>`;

        return `
          <div class="kanban-item" draggable="true"
               data-task-item="${escapeHTML(id)}"
               data-task-group="${escapeHTML(group)}">
            <div class="kanban-item-left">
              <span class="kanban-drag-handle" title="按住拖拽排序或跨栏">⋮⋮</span>
              <span class="kanban-idx-num">${String(idx + 1).padStart(2, '0')}</span>
              <div class="kanban-item-info">
                <div class="kanban-item-name">${escapeHTML(info.name)}</div>
                <div class="kanban-item-key">${escapeHTML(info.id)}</div>
              </div>
            </div>
            <div class="kanban-item-right">
              ${durationBadge}
              ${transferBtn}
            </div>
          </div>
        `;
      }).join('');

      const emptyHint = isFast ? '<span>暂无快任务，可将任务从右侧拖入</span>' : '<span>暂无慢任务，可将耗时任务从左侧拖入</span>';

      return `
        <div class="kanban-col" data-kanban-col="${escapeHTML(group)}">
          <div class="kanban-col-head">
            <div class="kanban-col-title">
              <span>${escapeHTML(title)}</span>
              <span class="badge-pill ${isFast ? 'cyan' : 'amber'}" style="font-size: 10.5px;">${list.length} 项</span>
            </div>
            <div class="kanban-col-actions">
              <button type="button" class="kanban-reset-btn" data-task-reset="true" title="重置为默认队列">↺ 恢复默认</button>
            </div>
          </div>
          <div class="kanban-col-body ${!list.length ? 'empty-placeholder' : ''}">
            ${items || emptyHint}
          </div>
        </div>
      `;
    };

    return `
      <div class="kanban-grid">
        ${renderCol('fast', '⚡ 快任务列表 (fast_tasks)', fastTasks, true)}
        ${renderCol('slow', '⏳ 慢任务列表 (slow_tasks)', slowTasks, false)}
      </div>
    `;
  }

  function renderFlowTask() {
    const taskFields = ['task.sign', 'task.playids', 'task.musician-sign', 'task.musician-vip', 'task.daily-song-share', 'task.vip-member-gift', 'task.fansgroup', 'task.note'];
    const cards = taskFields.map(path => {
      const info = flowFieldInfo(path);
      if (!info) return '';
      const key = info.keys.at(-1);
      const meta = taskFlowMeta[key] || { badge: '任务', tone: 'gray' };
      return `<article class="task-card ${info.value ? 'active' : ''}" data-task-card="${escapeHTML(path)}"><div class="task-card-top"><div><div class="task-card-title">${escapeHTML(info.field.title)}</div><div class="task-card-desc">${escapeHTML(info.field.description || '')}</div></div>${renderSchemaControl(info.field, info.keys, info.value)}</div><footer class="task-card-footer"><span class="badge-pill ${escapeHTML(meta.tone)}">${escapeHTML(meta.badge)}</span><code>${escapeHTML(path)}</code></footer></article>`;
    }).join('');
    const queue = renderFlowCard('', renderFlowField('task.mode', { title: '任务队列调度模式 (task.mode)', description: '跨账号分组串行：全部账号先完成快任务，再集中执行慢任务，总耗时更短；单账号完整串行：每个账号依次执行完快任务和慢任务后，再轮到下一个账号。' }));
    
    const root = activeDocument().data;
    const mode = String(root?.task?.mode || 'by-task-group').trim();
    const showQueue = mode === 'by-task-group';
    const { fastTasks, slowTasks } = getTaskQueueLists();
    const kanbanHTML = showQueue ? renderFlowTaskKanban(fastTasks, slowTasks) : '';

    return `
      <div class="task-cards-grid">${cards}</div>
      <div class="config-cards-grid single flow-task-mode">${queue}</div>
      <div class="task-queue-wrapper ${showQueue ? '' : 'hidden'}" id="task-queue-wrapper" style="display: ${showQueue ? 'block' : 'none'} !important;">
        ${kanbanHTML}
      </div>
    `;
  }

  function renderFlowSign() {
    const main = renderFlowCard('签到控制', renderFlowFields(['sign.automatic', 'sign.enableMain', 'sign.enableSecondaries', 'sign.enableVipTask']), '日常必备', 'success');
    const subtaskPaths = ['sign.yunbeiTask.enableViewVipCenter', 'sign.yunbeiTask.enableLikeComment', 'sign.yunbeiTask.enableListenIndie', 'sign.yunbeiTask.enableReserve', 'sign.yunbeiTask.enableFollowArtist', 'sign.yunbeiTask.enableLikeSong', 'sign.yunbeiTask.enableCollectSong', 'sign.yunbeiTask.enablePublishNote', 'sign.yunbeiTask.enableShareSong', 'sign.yunbeiTask.enablePlayDailyRecommend'];
    const subtasks = subtaskPaths.map(path => {
      const info = flowFieldInfo(path);
      if (!info) return '';
      return `<div class="subtask-item"><span>${escapeHTML(info.field.title)}</span>${renderSchemaControl(info.field, info.keys, info.value)}</div>`;
    }).join('');
    const detail = renderFlowCard('云贝精细化子任务', `<div class="subtask-grid">${subtasks}</div>`, '10 项子任务', 'cyan');
    return `<div class="config-cards-grid">${main}${detail}</div>`;
  }

  function renderFlowPlayids() {
    const accountSwitches = `<div class="form-row"><label class="form-row-label">账号分工</label><div class="flow-inline-switches">${renderFlowField('playids.enableMain', { title: '主账号刷歌', description: '' })}${renderFlowField('playids.enableSecondaries', { title: '辅助账号刷歌（推荐开启）', description: '' })}</div></div>`;
    const target = renderFlowCard('刷歌分工与目标', `${accountSwitches}${renderFlowRange('每日播放随机目标 (daily_min ~ daily_max)', 'playids.daily_min', 'playids.daily_max', '首 / 天', '每日首次启动时在此范围内随机生成当日目标。')}${renderFlowRange('单次运行随机目标 (run_min ~ run_max)', 'playids.run_min', 'playids.run_max', '首 / 次', '填 0 表示不限制单次数量，直接刷满今日剩余目标。')}`, '核心冲量', 'cyan');
    const source = renderFlowCard('切歌间隔与歌曲源', `${renderFlowRange('切歌随机等待间隔 (gap_min ~ gap_max)', 'playids.gap_min', 'playids.gap_max', '秒')}${renderFlowField('playids.ids', { title: '歌曲 ID 池 (playids.ids)', description: '' })}${renderFlowField('playids.idsFile', { title: '歌曲 ID 远程/本地文件列表 (playids.idsFile)', description: '' })}`, 'playids', 'primary');
    return `<div class="config-cards-grid">${target}${source}</div>`;
  }

  function renderFlowMixPlay() {
    const ratio = flowFieldInfo('mixPlay.dailyRecommendRatio');
    let ratioRow = '';
    if (ratio) {
      const percent = Math.round(Number(ratio.value || 0) * 100);
      const encoded = escapeHTML(JSON.stringify(ratio.keys));
      const safety = percent >= 25 && percent <= 50 ? '● S 极度安全' : percent >= 15 && percent <= 65 ? '● A 较安全' : '● B 建议调整';
      ratioRow = `<div class="form-row"><div class="form-row-label"><span>官方日推穿插比例 (dailyRecommendRatio)</span><span class="mix-safety-score">${safety}</span></div><div class="single-slider-wrap"><div class="single-slider-head"><span class="form-row-desc">穿插比例:</span><div class="single-slider-input-box"><input class="config-control range-input-box" data-path='${encoded}' data-kind="ratio-percent" type="number" value="${percent}" min="0" max="100" step="1"><span class="range-unit-text">%</span></div></div><input class="config-control single-range-input" data-path='${encoded}' data-kind="ratio-percent" type="range" min="0" max="100" step="1" value="${percent}"><div class="quick-preset-pills flow-preset-right"><button type="button" class="quick-pill" data-schema-path='${encoded}' data-schema-value="0.15">15% (轻微)</button><button type="button" class="quick-pill" data-schema-path='${encoded}' data-schema-value="0.3">30% (黄金推荐)</button><button type="button" class="quick-pill" data-schema-path='${encoded}' data-schema-value="0.5">50% (强力防护)</button></div></div><p class="form-row-desc">模拟真实听歌行为，降低连续播放行为过于单一的风险。</p></div>`;
    }
    const mix = renderFlowCard('混听开关与穿插比例', `${renderFlowField('mixPlay.enabled', { title: '启用日推混听 (enabled)', description: '' })}${ratioRow}`);
    const quota = renderFlowCard('额度计算规则', renderFlowField('mixPlay.countTarget', { title: '混听歌曲计入目标额度 (countTarget)', description: '关闭（推荐）：日推仅作为穿插播放，不占用指定歌曲的目标额度；开启：日推也计入目标总数。' }));
    return `<div class="config-cards-grid">${mix}${quota}</div>`;
  }

  function renderFlowNote() {
    const mode = renderFlowCard('', `${renderFlowField('note.type', { title: '动态类型 (type)', description: '' })}${renderFlowField('note.autoDelete', { title: '发布后立即删除 (autoDelete)', description: '' })}${renderFlowField('note.titles', { title: '随机标题池 (titles，每行一条)', description: '' })}${renderFlowField('note.titlesFile', { title: '标题文件来源 (titlesFile)' })}`);
    const content = renderFlowCard('', `${renderFlowField('note.messages', { title: '随机正文文案池 (messages，每行一条)', description: '' })}${renderFlowField('note.messagesFile', { title: '正文文件来源 (messagesFile)' })}${renderFlowField('note.imageUrls', { title: '图片素材链接池 (imageUrls)', description: '' })}`);
    return `<div class="config-cards-grid">${mode}${content}</div>`;
  }

  function renderFlowMusician() {
    const main = renderFlowCard('音乐人主控参数', renderFlowFields(['musician.enableMain', 'musician.enableSecondaries', 'musician.enableVipNote', 'musician.enableVipPlay', 'musician.identityCacheDays']));
    const play = renderFlowCard('进阶专属覆盖参数 (musician.play)', `${renderFlowField('musician.play.ids')}${renderFlowField('musician.play.idsFile')}${renderFlowRange('进阶每日目标范围 (daily_min ~ daily_max)', 'musician.play.daily_min', 'musician.play.daily_max', '首', '填 0 自动沿用全局播放目标。')}${renderFlowRange('进阶单次目标范围 (run_min ~ run_max)', 'musician.play.run_min', 'musician.play.run_max', '首')}${renderFlowRange('进阶切歌间隔 (gap_min ~ gap_max)', 'musician.play.gap_min', 'musician.play.gap_max', '秒')}`, '填 0 继承 playids', 'amber');
    return `<div class="config-cards-grid">${main}${play}</div>`;
  }

  function renderFlowDailySongShare() {
    const mainPaths = ['dailySongShare.enableMain', 'dailySongShare.enableSecondaries', 'dailySongShare.songId', 'dailySongShare.playlistId', 'dailySongShare.imageMode', 'dailySongShare.titleMode', 'dailySongShare.autoDelete'];
    const contentPaths = ['dailySongShare.imageUrls', 'dailySongShare.titles', 'dailySongShare.titlesFile', 'dailySongShare.messages', 'dailySongShare.messagesFile'];
    const publish = renderFlowCard('推歌发布控制', `${renderFlowFields(mainPaths)}<details class="flow-details"><summary>专属文案与配图</summary><div class="flow-details-body">${renderFlowFields(contentPaths)}</div></details>`, '需 Token', 'amber');
    const community = renderFlowCard('抽奖与社区话题', renderFlowFields(['dailySongShare.lottery.enabled', 'dailySongShare.lottery.activityId', 'dailySongShare.lottery.autoRegister', 'dailySongShare.topics']));
    return `<div class="config-cards-grid">${publish}${community}</div>`;
  }

  function renderFlowGeneric(key) {
    const layouts = {
      fansgroup: [
        { title: '', paths: ['fansgroup.enableMain', 'fansgroup.enableSecondaries', 'fansgroup.autoDeleteNote'] },
        { title: '', paths: ['fansgroup.groupIds'] },
      ],
      vipMemberGift: [
        { title: '', paths: ['vipMemberGift.enableMain', 'vipMemberGift.enableSecondaries', 'vipMemberGift.enableGift', 'vipMemberGift.enableClaim'] },
        { title: '自建云端服务 (vipMemberGift.cloud)', paths: ['vipMemberGift.cloud.baseUrl', 'vipMemberGift.cloud.token', 'vipMemberGift.refer'] },
      ],
      notify: [
        { title: '', paths: ['notify.enabled', 'notify.on_skip'] },
        { title: '', paths: ['notify.title_prefix', 'notify.timeout', 'notify.file'] },
      ],
      updater: [
        { title: '', paths: ['updater.check', 'updater.auto_update'] },
        { title: '', paths: ['updater.proxy_mirrors'] },
      ],
      log: [
        { title: '', paths: ['log.app', 'log.format', 'log.level', 'log.stdout'] },
        { title: '', paths: ['log.rotate.filename', 'log.rotate.maxsize', 'log.rotate.maxage', 'log.rotate.maxbackups', 'log.rotate.localtime', 'log.rotate.compress'] },
      ],
      database: [
        { title: '', paths: ['database.driver'] },
        { title: '', paths: ['database.path'] },
      ],
    };
    const layout = layouts[key];
    if (!layout) return renderConfigSection(key, activeDocument().data[key]);
    return `<div class="config-cards-grid">${layout.map(card => renderFlowCard(card.title, renderFlowFields(card.paths))).join('')}</div>`;
  }

  function renderConfigFlowSection(key, index) {
    const meta = configFlowMeta[key] || { title: sectionLabels[key] || key, description: schemaCategory(key)?.description || '' };
    let content;
    switch (key) {
      case 'accounts': content = renderFlowAccounts(); break;
      case 'network': content = renderFlowNetwork(); break;
      case 'task': content = renderFlowTask(); break;
      case 'sign': content = renderFlowSign(); break;
      case 'playids': content = renderFlowPlayids(); break;
      case 'mixPlay': content = renderFlowMixPlay(); break;
      case 'note': content = renderFlowNote(); break;
      case 'musician': content = renderFlowMusician(); break;
      case 'dailySongShare': content = renderFlowDailySongShare(); break;
      default: content = renderFlowGeneric(key);
    }
    return `<section class="card-flow-section" id="config-block-${escapeHTML(key)}" data-config-section="${escapeHTML(key)}"><header class="card-flow-heading"><div class="card-flow-heading-left"><span>${index + 1}. ${escapeHTML(meta.title)} <small>(${escapeHTML(key)})</small></span></div><span class="card-flow-heading-desc">${escapeHTML(meta.description)}</span></header>${content}</section>`;
  }

  function applyConfigSearch(rawQuery) {
    if (state.configTarget !== 'config' || state.configMode !== 'visual') return;
    const query = String(rawQuery || '').trim().toLowerCase();
    $$('.card-flow-section', $('#config-form')).forEach(section => {
      const heading = section.querySelector('.card-flow-heading')?.textContent.toLowerCase() || '';
      const headingMatches = !query || heading.includes(query);
      let visible = headingMatches;
      $$('.config-card-box, .task-card, .kanban-col, .kanban-item', section).forEach(card => {
        const match = headingMatches || card.textContent.toLowerCase().includes(query);
        card.classList.toggle('search-hidden', !match);
        visible ||= match;
      });
      section.classList.toggle('search-hidden', !visible);
    });
  }

  function updateConfigFlowActiveSection() {
    if (state.configTarget !== 'config' || state.configMode !== 'visual') return;
    const body = $('#config-visual');
    const sections = $$('.card-flow-section:not(.search-hidden)', $('#config-form'));
    if (!sections.length) return;
    const anchor = body.getBoundingClientRect().top + 28;
    let active = sections[0];
    for (const section of sections) {
      if (section.getBoundingClientRect().top <= anchor) active = section;
      else break;
    }
    const key = active.dataset.configSection;
    if (key) state.configSection = key;
    $$('#config-sections [data-section]').forEach(button => button.classList.toggle('active', button.dataset.section === key));
  }

  function scrollToConfigSection(key) {
    const body = $('#config-visual');
    const section = $(`#config-block-${CSS.escape(key)}`);
    if (!body || !section) return;
    const top = body.scrollTop + section.getBoundingClientRect().top - body.getBoundingClientRect().top - 12;
    body.scrollTo({ top, behavior: 'smooth' });
  }

  function renderConfigSection(key, value) {
    const target = activeConfigSchema();
    const category = schemaCategory(key);
    if (!target || !category) return renderFallbackConfigSection(key, value);

    const query = state.configSearch.trim().toLowerCase();
    const root = activeDocument().data;
    const categoryFields = target.fields.filter(field => field.path.startsWith(`${key}.`));
    const availableFields = categoryFields.filter(field => valueAtPath(root, field.path.split('.')).exists || Object.prototype.hasOwnProperty.call(field, 'default'));
    const matchesSearch = field => !query || `${field.title} ${field.description || ''} ${field.path}`.toLowerCase().includes(query);
    const visibleFields = availableFields.filter(matchesSearch);
    const cards = category.groups.map(group => {
      const fields = visibleFields.filter(field => field.group === group.id);
      if (!fields.length) return '';
      return `<section class="config-card-box"><header class="config-card-header"><h4 class="config-card-title">${escapeHTML(group.title)}</h4>${group.badge ? `<span class="schema-group-badge ${escapeHTML(group.tone || 'gray')}">${escapeHTML(group.badge)}</span>` : ''}</header><div class="schema-card-fields">${fields.map(renderSchemaField).join('')}</div></section>`;
    }).join('');

    const registered = new Set(categoryFields.map(field => field.path));
    const fallbackFields = [];
    collectFallbackConfigFields(value, [key], registered, fallbackFields);
    const visibleFallback = fallbackFields.filter(item => !query || `${item.path.join('.')} ${fieldLabels[item.key] || item.key}`.toLowerCase().includes(query));
    const fallback = visibleFallback.length
      ? `<section class="schema-fallback"><div class="schema-fallback-note"><strong>兼容配置</strong><span>以下字段尚未注册界面元数据，暂以通用控件显示。</span></div><div class="config-fields">${visibleFallback.map(item => renderFallbackConfigField(item.path, item.key, item.value)).join('')}</div></section>`
      : '';
    if (!cards && !fallback) return '<div class="schema-empty">没有匹配的配置项</div>';
    return `<div class="config-card-flow"><div class="config-cards-grid">${cards}</div>${fallback}</div>`;
  }

  function renderFallbackConfigSection(key, value) {
    const title = sectionLabels[key] || key;
    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      return `<fieldset class="config-group"><legend>${escapeHTML(title)}${descriptionMarkup([key])}</legend><div class="config-fields">${orderedEntries(value, [key]).map(([childKey, child]) => renderFallbackConfigField([key, childKey], childKey, child)).join('')}</div></fieldset>`;
    }
    return `<fieldset class="config-group"><legend>${escapeHTML(title)}${descriptionMarkup([key])}</legend><div class="config-fields">${renderFallbackConfigField([key], key, value)}</div></fieldset>`;
  }

  function collectFallbackConfigFields(value, path, registered, output) {
    const joined = path.join('.');
    if (registered.has(joined)) return;
    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      Object.entries(value).forEach(([key, child]) => collectFallbackConfigFields(child, [...path, key], registered, output));
      return;
    }
    output.push({ path, key: path.at(-1), value });
  }

  function renderSchemaField(field) {
    const path = field.path.split('.');
    const located = valueAtPath(activeDocument().data, path);
    const value = located.exists ? located.value : field.default;
    const control = renderSchemaControl(field, path, value);
    const advanced = field.advanced ? '<span class="schema-field-badge">高级</span>' : '';
    const copy = `<span class="schema-field-copy"><span class="schema-field-label">${escapeHTML(field.title)}${advanced}<code>${escapeHTML(path.at(-1))}</code></span>${field.description ? `<span class="schema-field-description">${escapeHTML(field.description)}</span>` : ''}</span>`;
    if (field.widget === 'switch') {
      return `<div class="schema-form-row switch-row"><div class="schema-field-heading">${copy}${control}</div></div>`;
    }
    return `<div class="schema-form-row"><div class="schema-field-heading">${copy}</div>${control}</div>`;
  }

  function renderSchemaControl(field, path, value) {
    const encodedPath = escapeHTML(JSON.stringify(path));
    const disabled = field.readOnly ? ' disabled' : '';
    const numberAttrs = `${field.min !== undefined ? ` min="${field.min}"` : ''}${field.max !== undefined ? ` max="${field.max}"` : ''}${field.step !== undefined ? ` step="${field.step}"` : ''}`;
    const unit = field.unit ? `<span class="schema-input-unit">${escapeHTML(field.unit)}</span>` : '';
    const stringValue = value ?? '';

    switch (field.widget) {
      case 'switch':
        return `<label class="schema-switch"><input class="config-control" data-path='${encodedPath}' type="checkbox" ${value ? 'checked' : ''}${disabled}><span></span></label>`;
      case 'segmented':
        return `<div class="schema-segmented">${(field.options || []).map(option => `<button type="button" class="schema-segment ${Object.is(option.value, value) ? 'active' : ''}" data-schema-path='${encodedPath}' data-schema-value='${escapeHTML(JSON.stringify(option.value))}'${disabled}>${escapeHTML(option.label)}</button>`).join('')}</div>`;
      case 'stepper':
        return `<div class="schema-stepper"><button type="button" data-schema-step="-1" title="减少"${disabled}>−</button><input class="config-control" data-path='${encodedPath}' data-kind="number" type="number" value="${escapeHTML(value)}"${numberAttrs}${disabled}><button type="button" data-schema-step="1" title="增加"${disabled}>＋</button>${unit}</div>`;
      case 'duration':
        return `<div class="schema-affix-line"><div class="schema-input-affix"><input class="config-control" data-path='${encodedPath}' data-kind="string" type="text" value="${escapeHTML(stringValue)}"${disabled}></div>${renderSchemaPresets(field, encodedPath, value, disabled)}</div>`;
      case 'slider': {
        const numeric = Number(value) || 0;
        const shown = field.unit === '%' ? `${Math.round(numeric * 100)}%` : `${numeric}${field.unit || ''}`;
        return `<div class="schema-slider"><div class="schema-slider-head"><output>${escapeHTML(shown)}</output><div class="schema-input-affix"><input class="config-control" data-path='${encodedPath}' data-kind="number" type="number" value="${numeric}"${numberAttrs}${disabled}>${unit}</div></div><input class="config-control schema-range" data-path='${encodedPath}' data-kind="number" type="range" value="${numeric}"${numberAttrs}${disabled}>${renderSchemaPresets(field, encodedPath, value, disabled)}</div>`;
      }
      case 'tags':
        return renderSchemaTags(field, encodedPath, value, disabled);
      case 'map':
        return renderSchemaMap(field, encodedPath, value, disabled);
      case 'json':
        return `<textarea class="config-control schema-textarea schema-json" data-path='${encodedPath}' data-kind="json" spellcheck="false"${disabled}>${escapeHTML(JSON.stringify(value ?? [], null, 2))}</textarea>`;
      case 'textarea-list':
        return `<textarea class="config-control schema-textarea" data-path='${encodedPath}' data-kind="array"${disabled}>${escapeHTML(Array.isArray(value) ? value.join('\n') : stringValue)}</textarea>`;
      case 'textarea':
        return `<textarea class="config-control schema-textarea" data-path='${encodedPath}' data-kind="string"${disabled}>${escapeHTML(stringValue)}</textarea>`;
      default:
        return `<div class="schema-input-affix"><input class="config-control" data-path='${encodedPath}' data-kind="${field.type === 'integer' || field.type === 'number' ? 'number' : 'string'}" type="${field.sensitive ? 'password' : field.type === 'integer' || field.type === 'number' ? 'number' : 'text'}" value="${escapeHTML(stringValue)}"${numberAttrs}${disabled}>${unit}</div>`;
    }
  }

  function renderSchemaPresets(field, encodedPath, value, disabled) {
    if (!field.presets?.length) return '';
    return `<div class="schema-presets">${field.presets.map(preset => `<button type="button" class="${Object.is(preset, value) ? 'active' : ''}" data-schema-path='${encodedPath}' data-schema-value='${escapeHTML(JSON.stringify(preset))}'${disabled}>${escapeHTML(field.unit === '%' ? `${Math.round(Number(preset) * 100)}%` : `${preset}${field.unit || ''}`)}</button>`).join('')}</div>`;
  }

  function renderSchemaTags(field, encodedPath, value, disabled) {
    const values = field.type === 'csv'
      ? String(value || '').split(',').map(item => item.trim()).filter(Boolean)
      : Array.isArray(value) ? value : value ? [String(value)] : [];
    return `<div class="schema-tags" data-schema-path='${encodedPath}' data-schema-kind="${escapeHTML(field.type)}">${values.map((item, index) => `<span class="schema-tag"><span>${escapeHTML(item)}</span><button type="button" data-schema-tag-remove="${index}" title="删除"${disabled}>×</button></span>`).join('')}<input class="schema-tag-input" type="text" placeholder="输入后按 Enter"${disabled}></div>`;
  }

  function renderSchemaMap(field, encodedPath, value, disabled) {
    const entries = value && typeof value === 'object' && !Array.isArray(value) ? Object.entries(value) : [];
    return `<div class="schema-map" data-schema-path='${encodedPath}' data-schema-sensitive="${field.sensitive ? 'true' : 'false'}"><div class="schema-map-rows">${entries.map(([key, item]) => renderSchemaMapRow(key, item, field.sensitive, disabled)).join('')}</div><button class="schema-map-add" type="button"${disabled}>＋ 添加一项</button></div>`;
  }

  function renderSchemaMapRow(key = '', value = '', sensitive = false, disabled = '') {
    return `<div class="schema-map-row"><input class="schema-map-key" type="text" value="${escapeHTML(key)}" placeholder="键"${disabled}><input class="schema-map-value" type="${sensitive ? 'password' : 'text'}" value="${escapeHTML(value ?? '')}" placeholder="值"${disabled}><button type="button" class="schema-map-remove" title="删除"${disabled}>×</button></div>`;
  }

  function renderFallbackConfigField(path, key, value) {
    const label = fieldLabels[key] || key;
    const encodedPath = escapeHTML(JSON.stringify(path));
    const full = value !== null && typeof value === 'object';
    if (typeof value === 'boolean') {
      return `<label class="config-field toggle-label"><span class="config-label">${escapeHTML(label)}</span>${descriptionMarkup(path)}<input class="config-control" data-path='${encodedPath}' type="checkbox" ${value ? 'checked' : ''}><span class="toggle"></span></label>`;
    }
    if (typeof value === 'number') {
      return `<label class="config-field"><span class="config-label">${escapeHTML(label)}</span>${descriptionMarkup(path)}<input class="config-control" data-path='${encodedPath}' data-kind="number" type="number" value="${value}"></label>`;
    }
    if (Array.isArray(value)) {
      if (value.every(item => typeof item !== 'object' || item === null)) {
        return `<label class="config-field full"><span class="config-label">${escapeHTML(label)}</span>${descriptionMarkup(path)}<textarea class="config-control array-input" data-path='${encodedPath}' data-kind="array">${escapeHTML(value.join('\n'))}</textarea></label>`;
      }
      return `<label class="config-field full"><span class="config-label">${escapeHTML(label)}</span>${descriptionMarkup(path)}<textarea class="config-control json-input" data-path='${encodedPath}' data-kind="json">${escapeHTML(JSON.stringify(value, null, 2))}</textarea></label>`;
    }
    if (value !== null && typeof value === 'object' && key === 'headers') {
      return `<label class="config-field full"><span class="config-label">${escapeHTML(label)}</span>${descriptionMarkup(path)}<textarea class="config-control json-input" data-path='${encodedPath}' data-kind="json">${escapeHTML(JSON.stringify(value, null, 2))}</textarea></label>`;
    }
    if (value !== null && typeof value === 'object') {
      return `<fieldset class="config-group config-field full"><legend>${escapeHTML(label)}${descriptionMarkup(path)}</legend><div class="config-fields">${orderedEntries(value, path).map(([childKey, child]) => renderFallbackConfigField([...path, childKey], childKey, child)).join('')}</div></fieldset>`;
    }
    if (key === 'body_template') {
      return `<label class="config-field full"><span class="config-label">${escapeHTML(label)}</span>${descriptionMarkup(path)}<textarea class="config-control template-input" data-path='${encodedPath}' data-kind="string">${escapeHTML(value ?? '')}</textarea></label>`;
    }
    const sensitive = /token|password|secret|skey/i.test(key);
    return `<label class="config-field ${full ? 'full' : ''}"><span class="config-label">${escapeHTML(label)}</span>${descriptionMarkup(path)}<input class="config-control" data-path='${encodedPath}' data-kind="string" type="${sensitive ? 'password' : 'text'}" value="${escapeHTML(value ?? '')}" autocomplete="off"></label>`;
  }

  const notifyDescriptions = {
    webhook: '自定义 Webhook 通道，按 JSON 请求推送通知。', 'webhook.enabled': '是否启用此通知通道。',
    'webhook.url': '接收通知的 Webhook 地址。', 'webhook.method': '发送通知时使用的 HTTP 方法。',
    'webhook.headers': '发送请求时附加的 HTTP 请求头。', 'webhook.body_template': '可选的 JSON 请求体模板。',
    bark: 'Bark 通知通道。', 'bark.enabled': '是否启用此通知通道。', 'bark.key': 'Bark 设备 Key。',
    'bark.server': 'Bark 服务地址，留空使用官方服务。', serverchan: 'Server 酱通知通道。',
    'serverchan.enabled': '是否启用此通知通道。', 'serverchan.sckey': 'Server 酱发送 Key。',
    telegram: 'Telegram Bot 通知通道。', 'telegram.enabled': '是否启用此通知通道。',
    'telegram.bot_token': 'Telegram Bot Token。', 'telegram.user_id': '接收通知的用户或群组 ID。',
    'telegram.api_host': '可选的 Telegram API 反向代理地址。', 'telegram.proxy': '可选的 HTTP 代理地址。',
    dingtalk: '钉钉机器人通知通道。', 'dingtalk.enabled': '是否启用此通知通道。',
    'dingtalk.access_token': '钉钉机器人 Access Token。', 'dingtalk.secret': '钉钉机器人加签 Secret。',
    coolpush: 'QQ CoolPush / Qmsg 通知通道。', 'coolpush.enabled': '是否启用此通知通道。',
    'coolpush.skey': 'CoolPush 服务 Key。', 'coolpush.mode': 'CoolPush 发送模式。',
    pushplus: 'PushPlus 通知通道。', 'pushplus.enabled': '是否启用此通知通道。', 'pushplus.token': 'PushPlus Token。',
    wecom_key: '企业微信群机器人通知通道。', 'wecom_key.enabled': '是否启用此通知通道。', 'wecom_key.key': '群机器人 Key。',
    wecom_app: '企业微信应用消息通知通道。', 'wecom_app.enabled': '是否启用此通知通道。',
    'wecom_app.corp_id': '企业微信 Corp ID。', 'wecom_app.corp_secret': '企业微信应用 Secret。',
    'wecom_app.to_user': '接收用户，默认 @all。', 'wecom_app.agent_id': '企业微信应用 Agent ID。',
    'wecom_app.media_id': '可选媒体 ID，填写后发送图文消息。',
  };

  function pathKey(path) {
    return path.join('.');
  }

  function descriptionMarkup(path) {
    const document = activeDocument();
    const description = document?.descriptions?.[pathKey(path)] || (state.configTarget === 'notify' ? notifyDescriptions[pathKey(path)] : '');
    return description ? `<small class="config-description">${escapeHTML(description)}</small>` : '';
  }

  function orderedEntries(value, path) {
    const keys = Object.keys(value);
    let preferred = [];
    if (state.configTarget === 'notify') preferred = path.length ? (notifyFieldOrder[path[0]] || []) : notifySectionOrder;
    else if (!path.length) preferred = configSectionOrder;
    else if (path[0] === 'accounts') preferred = ['main', 'secondary', 'antiCheatTokens'];
    return [...preferred.filter(key => keys.includes(key)), ...keys.filter(key => !preferred.includes(key))]
      .map(key => [key, value[key]]);
  }

  function updateConfigControl(target) {
    const path = JSON.parse(target.dataset.path);
    let value;
    if (target.type === 'checkbox') value = target.checked;
    else if (target.dataset.kind === 'duration-seconds') value = `${Math.max(0, Number(target.value) || 0)}s`;
    else if (target.dataset.kind === 'ratio-percent') value = Math.max(0, Math.min(100, Number(target.value) || 0)) / 100;
    else if (target.dataset.kind === 'number') value = Number(target.value);
    else if (target.dataset.kind === 'array') value = target.value.split('\n').map(x => x.trim()).filter(Boolean);
    else if (target.dataset.kind === 'json') {
      try { value = JSON.parse(target.value); target.setCustomValidity(''); }
      catch { target.setCustomValidity('JSON 格式无效'); return; }
    } else value = target.value;
    setByPath(activeDocument().data, path, value);
    $$('.config-control', $('#config-form')).forEach(peer => {
      if (peer === target || peer.dataset.path !== target.dataset.path || peer.type === 'checkbox') return;
      if (peer.dataset.kind === 'ratio-percent') peer.value = Math.round(Number(value) * 100);
      else if (peer.dataset.kind === 'duration-seconds') peer.value = Number.parseFloat(String(value)) || 0;
      else peer.value = value;
    });
    const slider = target.closest('.schema-slider');
    if (slider) {
      const output = slider.querySelector('output');
      const schema = activeConfigSchema()?.fields?.find(field => field.path === path.join('.'));
      if (output) output.textContent = schema?.unit === '%' ? `${Math.round(Number(value) * 100)}%` : `${value}${schema?.unit || ''}`;
    }
    const taskCard = target.closest('.task-card');
    if (taskCard && target.type === 'checkbox') taskCard.classList.toggle('active', target.checked);
    const mixCard = target.closest('#config-block-mixPlay');
    if (mixCard && target.dataset.kind === 'ratio-percent') {
      const percent = Math.round(Number(value) * 100);
      const score = mixCard.querySelector('.mix-safety-score');
      if (score) score.textContent = percent >= 25 && percent <= 50 ? '● S 极度安全' : percent >= 15 && percent <= 65 ? '● A 较安全' : '● B 建议调整';
    }
    markConfigDirty();
  }

  function markConfigDirty() {
    state.configDirty = true;
    state.configDirtyMode = 'visual';
    updateConfigDirtyUI();
  }

  function updateConfigDirtyUI() {
    const dirtyBadge = $('#config-dirty');
    const dirtyText = $('#config-dirty-text');
    const reloadText = $('#config-reload-text');
    const reloadBtn = $('#config-reload');
    const saveBtn = $('#config-save');
    const saveText = $('#config-save-text');

    if (state.configDirty) {
      dirtyBadge?.classList.remove('hidden');
      if (dirtyText) dirtyText.textContent = '有未保存修改';
      if (reloadText) reloadText.textContent = '放弃修改';
      if (reloadBtn) reloadBtn.title = '放弃本次所有未保存修改并回滚';
      if (saveBtn) saveBtn.classList.add('dirty-active');
      if (saveText) saveText.textContent = '保存配置';
    } else {
      dirtyBadge?.classList.add('hidden');
      if (reloadText) reloadText.textContent = '重新加载';
      if (reloadBtn) reloadBtn.title = '从磁盘重新读取最新配置文件';
      if (saveBtn) saveBtn.classList.remove('dirty-active');
      if (saveText) saveText.textContent = '保存配置';
    }
  }

  function applyPreset(presetKey) {
    const preset = PRESET_STRATEGIES[presetKey];
    if (!preset || state.configTarget !== 'config') return;
    const document = activeDocument();
    if (!document || !document.data) return;

    function deepMerge(target, source) {
      for (const [k, v] of Object.entries(source)) {
        if (v && typeof v === 'object' && !Array.isArray(v)) {
          if (!target[k] || typeof target[k] !== 'object' || Array.isArray(target[k])) target[k] = {};
          deepMerge(target[k], v);
        } else {
          target[k] = structuredClone(v);
        }
      }
    }
    deepMerge(document.data, preset.config);
    state.currentPreset = presetKey;
    markConfigDirty();
    renderConfig();
    toast(`已套用【${preset.name}】预设策略，点击右上角保存即可生效`);
  }

  async function handleReloadUndo() {
    if (state.configDirty) {
      try {
        const updated = await api(activeEndpoint());
        if (state.configTarget === 'notify') state.notify = updated;
        else {
          state.config = updated;
          renderAccounts();
          renderDashboard();
        }
        state.configDirty = false;
        state.configDirtyMode = '';
        renderConfig();
        toast('已放弃未保存修改并还原');
      } catch (error) {
        toast(error.message, true);
      }
      return;
    }

    try {
      const updated = await api(activeEndpoint());
      if (state.configTarget === 'notify') state.notify = updated;
      else {
        state.config = updated;
        renderAccounts();
        renderDashboard();
      }
      state.configDirty = false;
      state.configDirtyMode = '';
      renderConfig();
      toast('配置已重新加载');
    } catch (error) {
      toast(error.message, true);
    }
  }

  function setByPath(root, path, value) {
    let cursor = root;
    path.slice(0, -1).forEach(key => {
      if (cursor[key] === null || typeof cursor[key] !== 'object') cursor[key] = {};
      cursor = cursor[key];
    });
    cursor[path[path.length - 1]] = value;
  }

  function setSchemaValue(path, value, rerender = true) {
    setByPath(activeDocument().data, path, value);
    markConfigDirty();
    if (path.join('.') === 'task.mode') {
      const queueWrapper = $('#task-queue-wrapper');
      if (queueWrapper) queueWrapper.style.display = value === 'by-task-group' ? 'block' : 'none';
    }
    if (rerender) {
      const scrollTop = $('#config-visual')?.scrollTop || 0;
      renderConfig();
      if ($('#config-visual')) $('#config-visual').scrollTop = scrollTop;
    }
  }

  function addSchemaTag(container, raw) {
    const item = String(raw || '').trim().replace(/,$/, '').trim();
    if (!item) return;
    const path = JSON.parse(container.dataset.schemaPath);
    const current = valueAtPath(activeDocument().data, path).value;
    const values = container.dataset.schemaKind === 'csv'
      ? String(current || '').split(',').map(value => value.trim()).filter(Boolean)
      : Array.isArray(current) ? [...current] : [];
    if (!values.includes(item)) values.push(item);
    setSchemaValue(path, container.dataset.schemaKind === 'csv' ? values.join(',') : values);
  }

  function updateSchemaMap(container) {
    const value = {};
    $$('.schema-map-row', container).forEach(row => {
      const key = row.querySelector('.schema-map-key').value.trim();
      if (key) value[key] = row.querySelector('.schema-map-value').value;
    });
    setSchemaValue(JSON.parse(container.dataset.schemaPath), value, false);
  }

  async function saveConfig() {
    const button = $('#config-save');
    button.disabled = true;
    try {
      const document = activeDocument();
      const body = state.configMode === 'yaml'
        ? {
          revision: document.revision,
          raw: $('#yaml-editor').value,
          ...(!document.parseError ? { section: state.configSection } : {}),
        }
        : { revision: document.revision, data: document.data };
      const updated = await api(activeEndpoint(), { method: 'PUT', body });
      if (state.configTarget === 'notify') state.notify = updated;
      else {
        state.config = updated;
        renderAccounts();
        renderDashboard();
      }
      state.configDirty = false;
      state.configDirtyMode = '';
      renderConfig();
      toast('配置已保存，原文件已备份');
    } catch (error) { toast(error.message, true); }
    finally { button.disabled = false; }
  }

  function renderSchedules() {
    const tbody = $('#schedule-table');
    const query = state.scheduleSearch.trim().toLowerCase();
    const schedules = state.schedules
      .filter(job => !query || `${job.name} ${job.command} ${(job.args || []).join(' ')}`.toLowerCase().includes(query))
      .slice()
      .sort((a, b) => {
        if (Boolean(a.enabled) !== Boolean(b.enabled)) {
          return a.enabled ? -1 : 1;
        }
        if (Boolean(a.pinned) !== Boolean(b.pinned)) {
          return a.pinned ? -1 : 1;
        }
        return 0;
      });
    if (!schedules.length) {
      tbody.innerHTML = '<tr><td colspan="9" class="empty-state">暂无定时任务</td></tr>';
      return;
    }
    tbody.innerHTML = schedules.map(job => {
      const lastRun = state.runs.find(run => run.jobId === job.id)
        || state.runs.find(run => run.command === job.command && JSON.stringify(run.args || []) === JSON.stringify(job.args || []));
      return `<tr>
      <td><span class="task-name-link">${escapeHTML(job.name)}</span>${job.pinned ? ` <span class="task-pinned-badge">${icon('pin')}置顶</span>` : ''}${job.readOnly ? ' <span class="task-type-badge">环境托管</span>' : ''}</td>
      <td><span class="task-cmd-tag">${escapeHTML(job.command)} ${escapeHTML((job.args || []).join(' '))}</span></td>
      <td><code class="cron-capsule-input">${escapeHTML(job.cron)}</code></td>
      <td><span class="badge-pill ${job.running || job.queued ? 'blue' : job.enabled ? 'success' : 'gray'}">${job.running ? '运行中' : job.queued ? `排队 ${job.queued}` : job.enabled ? '空闲中' : '已禁用'}</span></td>
      <td>${lastRun ? formatScheduleDate(lastRun.startedAt || lastRun.triggeredAt) : '<span class="schedule-date">--</span>'}</td>
      <td>${job.nextRuns?.[0] ? formatScheduleDate(job.nextRuns[0]) : '<span class="schedule-date">--</span>'}</td>
      <td>${lastRun ? `<span class="badge-pill ${runBadgeClass(lastRun.status)}">${escapeHTML(statusLabel(lastRun.status))}</span>` : '<span class="badge-pill gray">未运行</span>'}</td>
      <td><span class="schedule-date">${lastRun ? duration(lastRun.startedAt, lastRun.finishedAt) : '--'}</span></td>
      <td class="actions-column"><div class="schedule-actions-row">
        ${job.running ? `<button class="schedule-act-link warning" data-action="stop" data-id="${escapeHTML(job.id)}">停止</button>` : `<button class="schedule-act-link action-run" data-action="run" data-id="${escapeHTML(job.id)}">运行</button>`}
        <button class="schedule-act-link ${job.enabled ? 'danger' : 'success'}" data-action="toggle" data-id="${escapeHTML(job.id)}" ${job.readOnly ? 'disabled' : ''}>${job.enabled ? '禁用' : '启用'}</button>
        <button class="schedule-act-link" data-action="logs" data-id="${escapeHTML(job.id)}">日志</button>
        <button class="schedule-act-link" data-action="edit" data-id="${escapeHTML(job.id)}" ${job.readOnly ? 'disabled' : ''}>编辑</button>
        <button class="schedule-act-link more schedule-more-button" data-action="more" data-id="${escapeHTML(job.id)}" aria-label="更多操作" title="更多操作" ${job.readOnly ? 'disabled' : ''}>${icon('more-vertical')}</button>
        <div class="schedule-pop-menu"><button class="schedule-pop-item" data-action="pin" data-id="${escapeHTML(job.id)}">${icon('pin')}${job.pinned ? '取消置顶' : '置顶'}</button><button class="schedule-pop-item danger" data-action="delete" data-id="${escapeHTML(job.id)}">${icon('trash')}删除</button></div>
      </div></td>
    </tr>`;
    }).join('');
  }

  function openScheduleDialog(job = null) {
    $('#schedule-dialog-title').textContent = job ? '编辑任务' : '新建任务';
    $('#schedule-id').value = job?.id || '';
    $('#schedule-name').value = job?.name || '';
    $('#schedule-enabled').checked = job?.enabled ?? true;
    $('#schedule-cron').value = job?.cron || '30 8 * * *';
    $('#schedule-command').value = job?.command || 'task';
    $('#schedule-args').value = (job?.args || []).join(' ');
    $('#schedule-overlap').value = job?.overlapPolicy || 'skip';
    const preset = [...$('#cron-preset').options].some(x => x.value === $('#schedule-cron').value) ? $('#schedule-cron').value : 'custom';
    $('#cron-preset').value = preset;
    renderCronNext(job?.nextRuns || []);
    $('#schedule-dialog').showModal();
  }

  async function saveSchedule(event) {
    event.preventDefault();
    const id = $('#schedule-id').value;
    const body = {
      name: $('#schedule-name').value.trim(), enabled: $('#schedule-enabled').checked,
      cron: $('#schedule-cron').value.trim(), command: $('#schedule-command').value,
      args: parseArgs($('#schedule-args').value), overlapPolicy: $('#schedule-overlap').value,
    };
    try {
      await api(id ? `/api/v1/schedules/${encodeURIComponent(id)}` : '/api/v1/schedules', { method: id ? 'PUT' : 'POST', body });
      $('#schedule-dialog').close();
      await refreshSchedules();
      toast(id ? '定时任务已更新' : '定时任务已创建');
    } catch (error) { toast(error.message, true); }
  }

  async function refreshSchedules() {
    state.schedules = await api('/api/v1/schedules');
    renderSchedules(); renderDashboard();
  }

  function renderRuns() {
    const tbody = $('#runs-table');
    const query = state.runSearch.trim().toLowerCase();
    const runs = state.runs.filter(run => {
      const statusMatches = state.runFilter === 'all'
        || state.runFilter === 'running' && ['running', 'queued'].includes(run.status)
        || run.status === state.runFilter;
      const searchMatches = !query || `${run.jobName} ${run.command} ${(run.args || []).join(' ')}`.toLowerCase().includes(query);
      return statusMatches && searchMatches;
    });
    if (!runs.length) {
      tbody.innerHTML = '<tr><td colspan="5" class="empty-state">暂无运行记录</td></tr>';
      return;
    }
    tbody.innerHTML = runs.map(run => `<tr>
      <td><div class="run-identity"><strong>${escapeHTML(run.jobName || run.command)}</strong><small>${escapeHTML([`${run.command} ${(run.args || []).join(' ')}`.trim(), shortRunID(run.id), runSourceLabel(run.source)].filter(Boolean).join(' · '))}</small></div></td>
      <td><span class="status-capsule ${['success'].includes(run.status) ? 'idle' : ['running', 'queued'].includes(run.status) ? 'running' : 'disabled'}">${statusLabel(run.status)}</span></td>
      <td><code>${duration(run.startedAt, run.finishedAt)}</code></td>
      <td><code>${formatRunDate(run.startedAt)}</code></td>
      <td class="actions-column"><div class="row-actions run-row-actions"><button class="table-action" data-run-action="log" data-id="${escapeHTML(run.id)}">查看</button>${['running', 'queued'].includes(run.status) ? `<button class="table-action danger" data-run-action="stop" data-id="${escapeHTML(run.id)}">停止</button>` : `<button class="table-action danger" data-run-action="delete" data-id="${escapeHTML(run.id)}">删除</button>`}</div></td>
    </tr>`).join('');
  }

  function renderSettings() {
    if (!state.settings) return;
    $('#retention-enabled').checked = state.settings.logs.retentionEnabled !== false;
    $('#retention-days').value = state.settings.logs.retentionDays;
    $('#max-size-enabled').checked = state.settings.logs.maxSizeEnabled !== false;
    $('#max-log-size').value = state.settings.logs.maxTotalSizeMB;
    $('#timezone-input').value = state.settings.timezone;
    $('#max-parallel').value = state.settings.concurrency?.maxParallel || 1;
    $('#logs-summary').textContent = `${state.settings.stats.files} 个文件 · ${formatBytes(state.settings.stats.sizeBytes)}`;
    const automatic = state.settings.logs.retentionEnabled || state.settings.logs.maxSizeEnabled;
    $('#retention-label').textContent = `自动清理:${automatic ? '开启' : '关闭'}`;
    $('#retention-days').disabled = !state.settings.logs.retentionEnabled;
    $('#max-log-size').disabled = !state.settings.logs.maxSizeEnabled;
    $('#system-log-size').textContent = `${formatBytes(state.settings.stats.sizeBytes)} / ${state.settings.logs.maxTotalSizeMB} MB`;
  }

  function renderSystem() {
    if (state.authSettings) {
      $('#password-min-length').value = state.authSettings.passwordMinLength;
      $('#new-password').minLength = state.authSettings.passwordMinLength;
      $('#confirm-password').minLength = state.authSettings.passwordMinLength;
      $('#require-letters').checked = !!state.authSettings.passwordRequireLetters;
      $('#require-digits').checked = !!state.authSettings.passwordRequireDigits;
      $('#require-symbols').checked = !!state.authSettings.passwordRequireSymbols;
      $('#session-ttl').value = String(state.authSettings.sessionTTLSeconds);
      $('#idle-timeout').value = String(state.authSettings.idleTimeoutSeconds);
      const enabled = state.authSettings.passwordProtectionEnabled !== false;
      $('#password-protection').checked = enabled;
      $('#password-protection-warning').classList.toggle('hidden', enabled);
      $('#auth-mode').textContent = enabled ? '已启用' : '已关闭';
      $('#auth-mode').className = `badge-pill ${enabled ? 'success' : 'gray'}`;
      const requirements = [state.authSettings.passwordRequireLetters && '英文字母', state.authSettings.passwordRequireDigits && '数字', state.authSettings.passwordRequireSymbols && '符号'].filter(Boolean);
      $('#password-policy-help').textContent = `密码长度至少 ${state.authSettings.passwordMinLength} 个字符${requirements.length ? `，并包含${requirements.join('、')}` : ''}`;
      $('#password-min-decrease').disabled = state.authSettings.passwordMinLength <= 1;
      $('#password-min-increase').disabled = state.authSettings.passwordMinLength >= 64;
    }
    updateSecurityDirtyState();
    $('#logout-button').classList.toggle('hidden', state.authSettings?.passwordProtectionEnabled === false);
    const update = state.update;
    if (!update) return;
    const currentVersion = update.current_version ? `v${String(update.current_version).replace(/^v/i, '')}` : '--';
    const latestVersion = update.latest_version ? `v${String(update.latest_version).replace(/^v/i, '')}` : '--';
    const checked = !!update.last_check_time && !String(update.last_check_time).startsWith('0001-');
    const failed = !!update.last_error;
    $('#update-panel').classList.toggle('hidden', !state.updateChecked);
    const statusTitle = failed ? '检查更新失败' : update.available ? '发现可用更新' : checked ? '当前已是最新版本' : '尚未检查更新';
    $('#update-current').textContent = currentVersion;
    $('#update-current-detail').textContent = currentVersion;
    $('#update-latest').textContent = latestVersion;
    $('#update-platform').textContent = `${update.os || '--'} / ${update.arch || '--'}`;
    $('#update-status-title').textContent = statusTitle;
    $('#update-status-badge').textContent = failed ? '失败' : update.available ? '可更新' : checked ? '最新' : '未检查';
    $('#update-status-badge').className = `badge-pill ${failed || update.available ? 'primary' : checked ? 'success' : 'gray'}`;
    $('#update-message').textContent = update.last_error || (update.available
      ? update.message || (update.docker ? 'Docker 部署请通过容器编排工具更新镜像。' : '检测到可用的新版本。')
      : checked ? `检查时间：${formatDate(update.last_check_time)}` : '');
    const notes = $('#release-notes');
    notes.textContent = update.release_notes || '';
    notes.classList.toggle('hidden', !update.available || !update.release_notes);
  }

  function securityFormSettings() {
    return {
      passwordProtectionEnabled: $('#password-protection').checked,
      passwordMinLength: Number($('#password-min-length').value),
      passwordRequireLetters: $('#require-letters').checked,
      passwordRequireDigits: $('#require-digits').checked,
      passwordRequireSymbols: $('#require-symbols').checked,
      sessionTTLSeconds: Number($('#session-ttl').value),
      idleTimeoutSeconds: Number($('#idle-timeout').value),
    };
  }

  function updateSecurityDirtyState() {
    const bar = $('.security-action-bar');
    if (!bar || !state.savedAuthSettings) {
      bar?.classList.add('hidden');
      return 0;
    }
    const current = securityFormSettings();
    const keys = Object.keys(current);
    const count = keys.filter(key => current[key] !== state.savedAuthSettings[key]).length;
    $('#security-dirty-note').textContent = `有未保存的设置项：${count}`;
    bar.classList.toggle('hidden', count === 0);
    return count;
  }

  function setServiceStatus(status) {
    const node = $('#service-status');
    const labels = { online: '在线', reconnecting: '重连中', offline: '离线' };
    node.dataset.state = status;
    node.lastChild.textContent = labels[status] || status;
    if (status === 'online') state.lastOnlineAt = new Date();
    node.title = state.lastOnlineAt ? `最近在线：${state.lastOnlineAt.toLocaleTimeString('zh-CN', { hour12: false })}` : '';
  }

  function playStatsRunToken(runs) {
	const latest = (runs || []).find(run => playStatsCommands.has(run.command) && run.finishedAt);
	return latest ? `${latest.id}:${latest.finishedAt}:${latest.status}` : '';
  }

  async function refreshPlayStatsAfterRun(runs) {
	if ((runs || []).some(run => databaseRunCommands.has(run.command) && ['running', 'queued'].includes(run.status))) return;
	const token = playStatsRunToken(runs);
	if (token === state.playStatsRunToken) return;
	try {
	  state.playStats = await api('/api/v1/play-stats');
	  state.playStatsRunToken = token;
	} catch {
	  // CLI 子进程持有 Badger 锁时保留旧数据，下次轮询继续尝试。
	}
  }

  async function refreshOperationalState() {
    if (!state.authenticated || state.pollInFlight) return;
    state.pollInFlight = true;
    const generation = ++state.pollGeneration;
    try {
      const [runs, status, schedules] = await Promise.all([
        api('/api/v1/runs'), api('/api/v1/status'), api('/api/v1/schedules'),
      ]);
      if (!state.authenticated || generation !== state.pollGeneration) return;
      state.runs = runs; state.status = status; state.schedules = schedules;
	  await refreshPlayStatsAfterRun(runs);
	  if (!state.authenticated || generation !== state.pollGeneration) return;
      renderRuns(); renderStatus(); renderSchedules(); renderDashboard();
      if ($('#log-dialog').open && $('#log-dialog').dataset.runId) await refreshOpenLog();
      state.consecutivePollFailures = 0;
      setServiceStatus('online');
    } catch {
      if (state.authenticated && generation === state.pollGeneration) {
        state.consecutivePollFailures += 1;
        setServiceStatus(state.consecutivePollFailures >= 3 ? 'offline' : 'reconnecting');
      }
    } finally {
      if (generation === state.pollGeneration) state.pollInFlight = false;
      scheduleNextPoll();
    }
  }

  function scheduleNextPoll() {
    clearTimeout(state.poller);
    if (!state.authenticated) return;
    state.poller = setTimeout(refreshOperationalState, document.hidden ? 30000 : 5000);
  }

  function startPolling() {
    state.pollGeneration += 1;
    state.pollInFlight = false;
    scheduleNextPoll();
  }

  function navigate(page, options = {}) {
    if (!pageRoutes[page]) page = 'dashboard';
    state.currentPage = page;
    $$('.nav-item').forEach(x => x.classList.toggle('active', x.dataset.page === page));
    $$('.page').forEach(x => x.classList.toggle('active', x.id === `page-${page}`));
    updatePageTitle();
    $('#sidebar').classList.remove('open');
    const route = pageRoutes[page];
    if (options.history !== false && location.pathname !== route) {
      history[options.replace ? 'replaceState' : 'pushState']({ page }, '', route);
    }
    if (page === 'config') requestAnimationFrame(updateConfigFlowActiveSection);
  }

  function updatePageTitle() {
    const title = pageTitles[state.currentPage];
    $('#page-title').textContent = $('#app-shell').classList.contains('sidebar-collapsed') ? `NCMM - ${title}` : title;
  }

  function parseArgs(input) {
    return [...input.matchAll(/(?:"([^"]*)"|'([^']*)'|(\S+))/g)].map(match => match[1] ?? match[2] ?? match[3]);
  }

  function renderCronNext(times) {
    $('#cron-next').textContent = times.length ? `下次：${times.slice(0, 3).map(formatDate).join('、')}` : '';
  }

  function statusLabel(status) {
    return ({ success: '成功', failed: '失败', running: '运行中', queued: '排队中', skipped: '已跳过', stopped: '已停止', interrupted: '已中断' })[status] || status;
  }

  function runBadgeClass(status) {
    if (status === 'success') return 'success';
    if (status === 'running' || status === 'queued') return 'blue';
    if (status === 'failed') return 'primary';
    return 'gray';
  }

  function shortRunID(value) {
    const id = String(value || '').replace(/^run-/, '');
    return id ? `run-${id.slice(0, 4)}` : '';
  }

  function runSourceLabel(source) {
    return ({ manual: '手动运行', scheduler: '自动调度', schedule: '自动调度' })[source] || source || '';
  }

  function formatDate(value) {
    if (!value) return '--';
    return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }).format(new Date(value));
  }

  function dateParts(value) {
    const date = new Date(value);
    if (!value || Number.isNaN(date.getTime())) return null;
    const pad = number => String(number).padStart(2, '0');
    return {
      year: date.getFullYear(), month: pad(date.getMonth() + 1), day: pad(date.getDate()),
      hour: pad(date.getHours()), minute: pad(date.getMinutes()), second: pad(date.getSeconds()),
    };
  }

  function formatScheduleDate(value) {
    const item = dateParts(value);
    return item ? `<span class="schedule-date">${item.month}-${item.day} ${item.hour}:${item.minute}:${item.second}</span>` : '<span class="schedule-date">--</span>';
  }

  function formatRunDate(value) {
    const item = dateParts(value);
    return item ? `${item.year}/${Number(item.month)}/${Number(item.day)} ${item.hour}:${item.minute}:${item.second}` : '--';
  }

  function duration(start, end) {
    const seconds = Math.max(0, Math.floor((new Date(end || Date.now()) - new Date(start)) / 1000));
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
    return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`;
  }

  function formatBuildDate(value) {
    if (!value || value === 'now') return '开发构建';
    const item = dateParts(value);
    return item ? `${item.year}-${item.month}-${item.day} 构建` : String(value);
  }

  function shortRevision(value) {
    const revision = String(value || '');
    if (revision.length <= 12) return revision || '--';
    return `${revision.slice(0, 4)}...${revision.slice(-4)}`;
  }

  function formatUptime(startedAt) {
    const totalMinutes = Math.max(0, Math.floor((Date.now() - new Date(startedAt).getTime()) / 60000));
    const days = Math.floor(totalMinutes / 1440);
    const hours = Math.floor((totalMinutes % 1440) / 60);
    const minutes = totalMinutes % 60;
    return `${days ? `${days}天 ` : ''}${hours}小时 ${minutes}分`;
  }

  function formatBytes(bytes) {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 ** 3) return `${(bytes / 1024 ** 2).toFixed(1)} MB`;
    return `${(bytes / 1024 ** 3).toFixed(1)} GB`;
  }

  async function refreshOpenLog() {
    const dialog = $('#log-dialog');
    const runID = dialog.dataset.runId;
    const content = await api(`/api/v1/runs/${encodeURIComponent(runID)}/log`);
    if (!dialog.open || dialog.dataset.runId !== runID || content === state.openLogContent) return;
    const pre = $('#log-content');
    const nearBottom = state.openLogContent === '' || pre.scrollHeight - pre.scrollTop - pre.clientHeight < 80;
    state.openLogContent = content;
    await new Promise(resolve => requestAnimationFrame(() => {
      pre.textContent = content;
      if (nearBottom) pre.scrollTop = pre.scrollHeight;
      resolve();
    }));
  }

  async function openRunLog(run) {
    const dialog = $('#log-dialog');
    dialog.dataset.runId = run.id;
    state.openLogContent = '';
    $('#log-title').textContent = run.jobName;
    $('#log-content').textContent = '正在加载日志...';
    if (!dialog.open) dialog.showModal();
    await refreshOpenLog();
  }

  function bindEvents() {
    document.addEventListener('visibilitychange', () => {
      if (!state.authenticated) return;
      clearTimeout(state.poller);
      if (document.hidden) scheduleNextPoll();
      else void refreshOperationalState();
    });
    $('#auth-form').addEventListener('submit', async event => {
      event.preventDefault();
      clearAuthError();
      $('#forgot-password-wrap').classList.remove('visible', 'open');
      const password = $('#password-input').value;
      if (!password) {
        showAuthError(state.authMode === 'setup' ? '请设置管理员密码。' : '请输入管理员密码。');
        return;
      }
      if (state.authMode === 'setup') {
        const settings = passwordSettings();
        const analysis = analyzePassword(password, settings);
        if (!analysis.charsOk) {
          showAuthError(`密码只能包含${enabledPasswordTypesText(settings)}，不能包含空格或中文。`);
          return;
        }
        if (!analysis.lengthOk) {
          showAuthError(`密码长度需为 ${settings.passwordMinLength}-${passwordMaxLength} 个字符。`);
          return;
        }
        if (!analysis.valid) {
          showAuthError('密码不符合安全要求，请根据上方规则调整。');
          return;
        }
        if (password !== $('#setup-confirm-password').value) {
          showAuthError('两次输入的密码不一致');
          return;
        }
      }
      const form = $('#auth-form');
      form.classList.add('loading');
      $('#login-submit').disabled = true;
      $('#setup-submit').disabled = true;
      try {
        const path = state.authMode === 'setup' ? '/api/v1/auth/setup' : '/api/v1/auth/login';
        const auth = await publicApi(path, { method: 'POST', body: { password } });
        $('#password-input').value = '';
        $('#setup-confirm-password').value = '';
        await enterApp(auth);
      } catch (error) {
        const revealRecovery = state.authMode === 'login' && error.message.includes('密码不正确');
        showAuthError(error.message, revealRecovery);
      } finally {
        form.classList.remove('loading');
        $('#login-submit').disabled = false;
        $('#setup-submit').disabled = false;
      }
    });
    $('#password-input').addEventListener('input', event => {
      const settings = state.authMode === 'setup' ? passwordSettings() : undefined;
      const normalized = normalizePasswordInput(event.target.value, settings);
      if (event.target.value !== normalized) {
        event.target.value = normalized;
        showAuthError(`密码只能包含${state.authMode === 'setup' ? enabledPasswordTypesText(settings) : '英文字母、数字和符号'}，不能包含空格或中文。`);
      } else {
        clearAuthError();
      }
      $('#forgot-password-wrap').classList.remove('visible', 'open');
      renderPasswordStrength();
    });
    $('#setup-confirm-password').addEventListener('input', event => {
      const settings = passwordSettings();
      const normalized = normalizePasswordInput(event.target.value, settings);
      if (event.target.value !== normalized) {
        event.target.value = normalized;
        showAuthError(`密码只能包含${enabledPasswordTypesText(settings)}，不能包含空格或中文。`);
      } else {
        clearAuthError();
      }
    });
    $('#password-input').addEventListener('focus', () => $('#login-logo').classList.add('active'));
    $('#password-input').addEventListener('blur', () => $('#login-logo').classList.remove('active'));
    $('#forgot-password').addEventListener('click', () => $('#forgot-password-wrap').classList.toggle('open'));
    $('#logout-button').addEventListener('click', logout);
    $('#theme-button').addEventListener('click', () => {
      const dark = document.documentElement.dataset.theme === 'dark';
      applyTheme(dark ? 'light' : 'dark', true);
    });
    $('#refresh-button').addEventListener('click', async () => {
      const button = $('#refresh-button');
      button.disabled = true;
      try { await refreshOperationalState(); toast('状态已刷新'); }
      finally { button.disabled = false; }
    });
    $('#menu-button').addEventListener('click', () => {
      if (matchMedia('(max-width: 760px)').matches) $('#sidebar').classList.toggle('open');
      else {
        $('#app-shell').classList.toggle('sidebar-collapsed');
        updatePageTitle();
      }
    });
    $$('.nav-item').forEach(button => button.addEventListener('click', () => navigate(button.dataset.page)));
    $$('[data-go]').forEach(button => button.addEventListener('click', () => navigate(button.dataset.go)));
    $('#dashboard-metric').addEventListener('click', event => {
      const button = event.target.closest('[data-metric]');
      if (!button) return;
      state.dashboardMetric = button.dataset.metric;
      $$('#dashboard-metric button').forEach(item => item.classList.toggle('active', item === button));
      renderDashboard();
    });
    $('#dashboard-account-select').addEventListener('change', event => {
      state.dashboardAccountPath = event.target.value;
      renderDashboard();
    });

    $('#config-sections').addEventListener('click', async event => {
      const button = event.target.closest('[data-section]');
      if (!button) return;
      const nextSection = button.dataset.section;
      if (state.configTarget === 'config' && state.configMode === 'visual') {
        state.configSection = nextSection;
        $$('#config-sections [data-section]').forEach(item => item.classList.toggle('active', item === button));
        scrollToConfigSection(nextSection);
        return;
      }
      if (state.configDirty && nextSection !== state.configSection) {
        if (!confirm('切换配置项会放弃当前未保存修改，是否继续？')) return;
        try {
          const updated = await api(activeEndpoint());
          if (state.configTarget === 'notify') state.notify = updated;
          else state.config = updated;
          state.configDirty = false;
          state.configDirtyMode = '';
        } catch (error) { toast(error.message, true); return; }
      }
      state.configSection = nextSection;
      state.configSearch = '';
      renderConfig();
    });
    $('#config-search').addEventListener('input', event => {
      state.configSearch = event.target.value;
      if (state.configTarget === 'config' && state.configMode === 'visual') {
        applyConfigSearch(state.configSearch);
        updateConfigFlowActiveSection();
      } else {
        renderConfig();
        $('#config-search').focus();
      }
    });
    $('#config-form').addEventListener('input', event => {
      const target = event.target;
      if (target.matches('.config-control')) updateConfigControl(target);
      if (target.matches('.config-token-control')) {
        const root = activeDocument().data;
        root.accounts ||= {};
        root.accounts.antiCheatTokens ||= {};
        root.accounts.antiCheatTokens[target.dataset.tokenCookie] = target.value;
        markConfigDirty();
      }
      if (target.matches('.schema-map-key, .schema-map-value')) updateSchemaMap(target.closest('.schema-map'));
    });
    $('#config-form').addEventListener('keydown', event => {
      if (!event.target.matches('.schema-tag-input') || !['Enter', ','].includes(event.key)) return;
      event.preventDefault();
      addSchemaTag(event.target.closest('.schema-tags'), event.target.value);
    });
    $('#config-form').addEventListener('click', event => {
      const valueButton = event.target.closest('[data-schema-value]');
      if (valueButton) {
        setSchemaValue(JSON.parse(valueButton.dataset.schemaPath), JSON.parse(valueButton.dataset.schemaValue));
        return;
      }
      const stepButton = event.target.closest('[data-schema-step]');
      if (stepButton) {
        const input = stepButton.closest('.schema-stepper')?.querySelector('.config-control');
        if (!input || input.disabled) return;
        const step = Number(input.step) || 1;
        const min = input.min === '' ? -Infinity : Number(input.min);
        const max = input.max === '' ? Infinity : Number(input.max);
        input.value = Math.min(max, Math.max(min, Number(input.value || 0) + Number(stepButton.dataset.schemaStep) * step));
        input.dispatchEvent(new Event('input', { bubbles: true }));
        return;
      }
      const taskCard = event.target.closest('.task-card');
      if (taskCard && !event.target.closest('input, label, button')) {
        const checkbox = taskCard.querySelector('input[type="checkbox"]');
        if (checkbox && !checkbox.disabled) {
          checkbox.checked = !checkbox.checked;
          checkbox.dispatchEvent(new Event('input', { bubbles: true }));
        }
        return;
      }
      const tagRemove = event.target.closest('[data-schema-tag-remove]');
      if (tagRemove) {
        const container = tagRemove.closest('.schema-tags');
        const path = JSON.parse(container.dataset.schemaPath);
        const current = valueAtPath(activeDocument().data, path).value;
        const values = container.dataset.schemaKind === 'csv'
          ? String(current || '').split(',').map(value => value.trim()).filter(Boolean)
          : Array.isArray(current) ? [...current] : [];
        values.splice(Number(tagRemove.dataset.schemaTagRemove), 1);
        setSchemaValue(path, container.dataset.schemaKind === 'csv' ? values.join(',') : values);
        return;
      }
      const mapAdd = event.target.closest('.schema-map-add');
      if (mapAdd) {
        const container = mapAdd.closest('.schema-map');
        container.querySelector('.schema-map-rows').insertAdjacentHTML('beforeend', renderSchemaMapRow('', '', container.dataset.schemaSensitive === 'true'));
        container.querySelector('.schema-map-row:last-child .schema-map-key').focus();
        return;
      }
      const transferBtn = event.target.closest('[data-task-transfer]');
      if (transferBtn) {
        const taskId = transferBtn.dataset.taskId;
        const targetGroup = transferBtn.dataset.taskTransfer;
        if (targetGroup === 'slow') moveTaskToSlow(taskId);
        else moveTaskToFast(taskId);
        return;
      }
      const resetBtn = event.target.closest('[data-task-reset]');
      if (resetBtn) {
        resetTaskQueueDefault();
        return;
      }
      const mapRemove = event.target.closest('.schema-map-remove');
      if (mapRemove) {
        const container = mapRemove.closest('.schema-map');
        mapRemove.closest('.schema-map-row').remove();
        updateSchemaMap(container);
      }
    });
    $('#config-form').addEventListener('dragstart', event => {
      const item = event.target.closest('.kanban-item');
      if (!item) return;
      draggedTaskId = item.dataset.taskItem;
      draggedTaskGroup = item.dataset.taskGroup;
      item.classList.add('dragging');
      event.dataTransfer.effectAllowed = 'move';
      event.dataTransfer.setData('text/plain', draggedTaskId);
    });
    $('#config-form').addEventListener('dragend', event => {
      const item = event.target.closest('.kanban-item');
      if (item) item.classList.remove('dragging');
      $$('.kanban-col', $('#config-form')).forEach(col => col.classList.remove('drag-over'));
      $$('.kanban-item', $('#config-form')).forEach(el => el.classList.remove('drag-insert-top', 'drag-insert-bottom'));
      draggedTaskId = null;
      draggedTaskGroup = null;
      dragInsertPosition = null;
    });
    $('#config-form').addEventListener('dragover', event => {
      const col = event.target.closest('.kanban-col');
      if (!col || !draggedTaskId) return;
      event.preventDefault();
      event.dataTransfer.dropEffect = 'move';
      col.classList.add('drag-over');

      const item = event.target.closest('.kanban-item');
      if (item && item.dataset.taskItem !== draggedTaskId) {
        const rect = item.getBoundingClientRect();
        const isTop = event.clientY < rect.top + rect.height / 2;
        dragInsertPosition = isTop ? 'top' : 'bottom';
        item.classList.toggle('drag-insert-top', isTop);
        item.classList.toggle('drag-insert-bottom', !isTop);
      }
    });
    $('#config-form').addEventListener('dragleave', event => {
      const item = event.target.closest('.kanban-item');
      if (item && !item.contains(event.relatedTarget)) {
        item.classList.remove('drag-insert-top', 'drag-insert-bottom');
      }
      const col = event.target.closest('.kanban-col');
      if (col && !col.contains(event.relatedTarget)) {
        col.classList.remove('drag-over');
      }
    });
    $('#config-form').addEventListener('drop', event => {
      const col = event.target.closest('.kanban-col');
      if (!col || !draggedTaskId) return;
      event.preventDefault();

      const targetGroup = col.dataset.kanbanCol;
      const targetItem = event.target.closest('.kanban-item');
      const targetTaskId = targetItem ? targetItem.dataset.taskItem : null;

      $$('.kanban-col', $('#config-form')).forEach(c => c.classList.remove('drag-over'));
      $$('.kanban-item', $('#config-form')).forEach(el => el.classList.remove('drag-insert-top', 'drag-insert-bottom'));

      const { fastTasks, slowTasks } = getTaskQueueLists();
      let sourceList = draggedTaskGroup === 'fast' ? fastTasks : slowTasks;
      let targetList = targetGroup === 'fast' ? fastTasks : slowTasks;

      // Remove from source
      const sIdx = sourceList.indexOf(draggedTaskId);
      if (sIdx >= 0) sourceList.splice(sIdx, 1);

      // Insert into target
      if (targetTaskId && targetTaskId !== draggedTaskId) {
        let tIdx = targetList.indexOf(targetTaskId);
        if (tIdx < 0) {
          targetList.push(draggedTaskId);
        } else {
          if (dragInsertPosition === 'bottom') tIdx += 1;
          targetList.splice(tIdx, 0, draggedTaskId);
        }
      } else {
        targetList.push(draggedTaskId);
      }

      setTaskQueueLists(fastTasks, slowTasks);
    });
    $('#yaml-editor').addEventListener('input', () => {
      state.configDirty = true;
      state.configDirtyMode = 'yaml';
      $('#config-dirty').classList.remove('hidden');
      renderYAMLEditor();
    });
    $('#yaml-editor').addEventListener('scroll', event => {
      $('#yaml-highlight').scrollTop = event.target.scrollTop;
      $('#yaml-highlight').scrollLeft = event.target.scrollLeft;
      $('#yaml-line-numbers').style.transform = `translateY(${-event.target.scrollTop}px)`;
    });
    ['click', 'keyup', 'select'].forEach(name => $('#yaml-editor').addEventListener(name, updateYAMLCursor));
    $('#config-target').addEventListener('click', async event => {
      const button = event.target.closest('[data-target]'); if (!button || button.dataset.target === state.configTarget) return;
      if (state.configDirty && !confirm('切换配置文件会放弃当前未保存修改，是否继续？')) return;
      if (state.configDirty) {
        try {
          const updated = await api(activeEndpoint());
          if (state.configTarget === 'notify') state.notify = updated;
          else {
            state.config = updated;
            renderAccounts();
            renderDashboard();
          }
        } catch (error) { toast(error.message, true); return; }
      }
      state.configTarget = button.dataset.target;
      state.configSection = '';
      state.configSearch = '';
      state.configDirty = false;
      state.configDirtyMode = '';
      renderConfig();
    });
    $('#config-mode').addEventListener('click', async event => {
      const button = event.target.closest('[data-mode]'); if (!button || button.dataset.mode === state.configMode) return;
      if (state.configDirty && state.configDirtyMode && state.configDirtyMode !== button.dataset.mode) {
        if (!confirm('切换编辑模式会放弃当前未保存修改，是否继续？')) return;
        try {
          const updated = await api(activeEndpoint());
          if (state.configTarget === 'notify') state.notify = updated;
          else {
            state.config = updated;
            renderAccounts();
            renderDashboard();
          }
          state.configDirty = false;
          state.configDirtyMode = '';
        }
        catch (error) { toast(error.message, true); return; }
      }
      state.configMode = button.dataset.mode;
      renderConfig();
    });
    $('#preset-chips-list')?.addEventListener('click', event => {
      const button = event.target.closest('[data-preset]');
      if (button) applyPreset(button.dataset.preset);
    });
    $('#config-reload')?.addEventListener('click', handleReloadUndo);
    $('#config-save')?.addEventListener('click', saveConfig);
    let configScrollFrame = 0;
    $('#config-visual').addEventListener('scroll', () => {
      if (configScrollFrame) cancelAnimationFrame(configScrollFrame);
      configScrollFrame = requestAnimationFrame(() => {
        configScrollFrame = 0;
        updateConfigFlowActiveSection();
      });
    }, { passive: true });

    $('#notify-test')?.addEventListener('click', async () => {
      if (state.configTarget !== 'notify' || !state.configSection) return;
      if (state.configDirty) { toast('请先保存当前通道配置', true); return; }
      const button = $('#notify-test');
      button.disabled = true;
      try {
        await api(`/api/v1/notify/${encodeURIComponent(state.configSection)}/test`, { method: 'POST' });
        toast(`${sectionLabels[state.configSection] || state.configSection} 测试消息已发送`);
      } catch (error) { toast(`发送失败：${error.message}`, true); }
      finally { button.disabled = false; }
    });

    $('#account-add').addEventListener('click', () => openAccountPanel());
    $('#account-close').addEventListener('click', closeAccountPanel);
    $('#accounts-table').addEventListener('click', async event => {
      const button = event.target.closest('[data-account-action]');
      if (!button) return;
      const account = configuredAccounts()[Number(button.dataset.index)];
      if (!account) return;
      try {
        if (button.dataset.accountAction === 'edit') openAccountPanel(account);
        if (button.dataset.accountAction === 'delete') await deleteConfiguredAccount(account);
      } catch (error) { toast(error.message, true); }
    });
    $$('#account-login-method button').forEach(button => button.addEventListener('click', () => {
      if (qrcodeSessionActive()) {
        toast('请先完成或取消当前二维码登录', true);
        return;
      }
      setAccountMethod(button.dataset.loginMethod);
    }));
    $$('.account-type button').forEach(button => button.addEventListener('click', () => {
      if (qrcodeSessionActive() || state.accountEditing) return;
      const previousDefault = state.accountMain ? 'cookie.json' : 'fan1.json';
      state.accountMain = button.dataset.main === 'true';
      $$('.account-type button').forEach(x => x.classList.toggle('active', x === button));
      const nextDefault = state.accountMain ? 'cookie.json' : 'fan1.json';
      if ($('#account-filename').value === previousDefault || $('#account-filename').value === 'account.json') $('#account-filename').value = nextDefault;
      renderQRCodeSession();
    }));
    $('#account-filename').addEventListener('input', renderQRCodeSession);
    $('#account-form').addEventListener('submit', async event => {
      event.preventDefault();
      const submit = $('#account-submit'); submit.disabled = true;
      if (state.accountMethod === 'qrcode') {
        clearQRCodeImage();
        state.qrcodeSession = null;
        renderQRCodeSession();
        try {
          state.qrcodeSession = await api('/api/v1/accounts/qrcode', {
            method: 'POST', body: {
              filename: $('#account-filename').value, main: state.accountMain,
            }
          });
          renderQRCodeSession();
          startQRCodePolling();
        } catch (error) {
          toast(error.message, true);
          submit.disabled = false;
        }
        return;
      }
      try {
        const result = await api('/api/v1/accounts/cookie', {
          method: 'POST', body: {
            content: $('#cookie-content').value, format: $('#cookie-format').value,
            filename: $('#account-filename').value, main: state.accountMain,
          }
        });
        $('#cookie-content').value = '';
        $('#cookie-result').textContent = result.message || `已导入 ${result.filename}`;
        $('#cookie-result').classList.remove('hidden');
        state.config = await api('/api/v1/config'); renderConfig(); renderAccounts(); renderDashboard();
        toast('Cookie 验证并导入成功');
        closeAccountPanel();
      } catch (error) { toast(error.message, true); }
      finally { submit.disabled = false; }
    });
    $('#qrcode-cancel').addEventListener('click', async () => {
      if (!state.qrcodeSession?.id || !qrcodeSessionActive()) return;
      $('#qrcode-cancel').disabled = true;
      try {
        state.qrcodeSession = await api(`/api/v1/accounts/qrcode/${encodeURIComponent(state.qrcodeSession.id)}`, { method: 'DELETE' });
        renderQRCodeSession();
      } catch (error) { toast(error.message, true); }
      finally { $('#qrcode-cancel').disabled = false; }
    });

    $('#schedule-add').addEventListener('click', () => openScheduleDialog());
    $('#schedule-search').addEventListener('input', event => { state.scheduleSearch = event.target.value; renderSchedules(); });
    $$('.dialog-close').forEach(x => x.addEventListener('click', () => $('#schedule-dialog').close()));
    $('#schedule-form').addEventListener('submit', saveSchedule);
    $('#cron-preset').addEventListener('change', event => { if (event.target.value !== 'custom') $('#schedule-cron').value = event.target.value; });
    $('#schedule-table').addEventListener('click', async event => {
      const button = event.target.closest('[data-action]'); if (!button) return;
      const job = state.schedules.find(x => x.id === button.dataset.id); if (!job) return;
      try {
        if (button.dataset.action === 'more') {
          const menu = button.nextElementSibling;
          const shouldOpen = !!menu && !menu.classList.contains('open');
          $$('.schedule-pop-menu.open').forEach(item => item.classList.remove('open'));
          if (shouldOpen) {
            menu.classList.add('open');
            const rect = button.getBoundingClientRect();
            const cardBottom = button.closest('.schedule-table-card')?.getBoundingClientRect().bottom || window.innerHeight;
            menu.classList.toggle('open-up', rect.bottom + menu.offsetHeight + 5 > cardBottom);
          }
          return;
        }
        if (button.dataset.action === 'pin') {
          button.closest('.schedule-pop-menu')?.classList.remove('open');
          await api(`/api/v1/schedules/${encodeURIComponent(job.id)}/pin`, { method: 'POST' });
          await refreshSchedules();
          toast(job.pinned ? '已取消置顶' : '定时任务已置顶');
          return;
        }
        if (button.dataset.action === 'edit') openScheduleDialog(job);
        if (button.dataset.action === 'stop') {
          const activeRun = state.runs.find(r => (r.jobId === job.id || (r.command === job.command && JSON.stringify(r.args || []) === JSON.stringify(job.args || []))) && ['running', 'queued'].includes(r.status));
          if (activeRun) {
            await api(`/api/v1/runs/${encodeURIComponent(activeRun.id)}/stop`, { method: 'POST' });
            toast('正在停止任务');
            await refreshOperationalState();
          }
          return;
        }
        if (button.dataset.action === 'run') { await api(`/api/v1/schedules/${encodeURIComponent(job.id)}/run`, { method: 'POST' }); toast('任务已启动'); await refreshOperationalState(); }
        if (button.dataset.action === 'toggle') {
          await api(`/api/v1/schedules/${encodeURIComponent(job.id)}`, {
            method: 'PUT', body: {
              name: job.name, enabled: !job.enabled, cron: job.cron, command: job.command,
              args: job.args || [], overlapPolicy: job.overlapPolicy || 'skip',
            }
          });
          await refreshSchedules();
          toast(job.enabled ? '定时任务已禁用' : '定时任务已启用');
        }
        if (button.dataset.action === 'logs') {
          const latestRun = state.runs.find(run => run.jobId === job.id)
            || state.runs.find(run => run.command === job.command && JSON.stringify(run.args || []) === JSON.stringify(job.args || []));
          if (!latestRun) {
            toast('该任务暂无运行日志');
            return;
          }
          await openRunLog(latestRun);
        }
        if (button.dataset.action === 'delete' && confirm(`删除定时任务“${job.name}”？`)) { await api(`/api/v1/schedules/${encodeURIComponent(job.id)}`, { method: 'DELETE' }); await refreshSchedules(); toast('定时任务已删除'); }
      } catch (error) { toast(error.message, true); }
    });
    document.addEventListener('click', event => {
      if (!event.target.closest('.schedule-actions-row')) $$('.schedule-pop-menu.open').forEach(item => item.classList.remove('open'));
    });

    $('#runs-table').addEventListener('click', async event => {
      const button = event.target.closest('[data-run-action]'); if (!button) return;
      const run = state.runs.find(x => x.id === button.dataset.id); if (!run) return;
      try {
        if (button.dataset.runAction === 'log') await openRunLog(run);
        if (button.dataset.runAction === 'stop') { await api(`/api/v1/runs/${encodeURIComponent(run.id)}/stop`, { method: 'POST' }); toast('正在停止任务'); }
        if (button.dataset.runAction === 'delete' && confirm(`删除“${run.jobName}”的这条运行日志？`)) {
          await api(`/api/v1/runs/${encodeURIComponent(run.id)}`, { method: 'DELETE' });
          state.runs = state.runs.filter(item => item.id !== run.id);
          renderRuns(); renderDashboard();
          toast('运行日志已删除');
        }
      } catch (error) { toast(error.message, true); }
    });
    $('#run-filter').addEventListener('click', event => {
      const button = event.target.closest('[data-run-filter]');
      if (!button) return;
      state.runFilter = button.dataset.runFilter;
      $$('#run-filter button').forEach(item => item.classList.toggle('active', item === button));
      renderRuns();
    });
    $('#run-search').addEventListener('input', event => { state.runSearch = event.target.value; renderRuns(); });
    $('#logs-auto-open').addEventListener('click', () => { renderSettings(); $('#logs-auto-dialog').showModal(); });
    $$('.logs-auto-close').forEach(button => button.addEventListener('click', () => $('#logs-auto-dialog').close()));
    $('#retention-enabled').addEventListener('change', event => { $('#retention-days').disabled = !event.target.checked; });
    $('#max-size-enabled').addEventListener('change', event => { $('#max-log-size').disabled = !event.target.checked; });
    $('#logs-auto-form').addEventListener('submit', async event => {
      event.preventDefault();
      try {
        state.settings = await api('/api/v1/settings', {
          method: 'PUT', body: {
            timezone: state.settings.timezone,
            logs: {
              retentionEnabled: $('#retention-enabled').checked,
              retentionDays: Number($('#retention-days').value),
              maxSizeEnabled: $('#max-size-enabled').checked,
              maxTotalSizeMB: Number($('#max-log-size').value),
            },
            concurrency: state.settings.concurrency,
          },
        });
        $('#logs-auto-dialog').close();
        renderSettings();
        toast('自动清理设置已保存');
      } catch (error) { toast(error.message, true); }
    });
    $('#logs-advanced-open').addEventListener('click', () => $('#logs-advanced-dialog').showModal());
    $$('.logs-advanced-close').forEach(button => button.addEventListener('click', () => $('#logs-advanced-dialog').close()));
    $('#logs-advanced-form').addEventListener('submit', async event => {
      event.preventDefault();
      const start = $('#cleanup-start-date').value;
      const end = $('#cleanup-end-date').value;
      const status = $('#cleanup-status').value;
      if (!confirm('确认按当前条件永久清理运行日志？')) return;
      try {
        const result = await api('/api/v1/logs/cleanup/advanced', {
          method: 'POST', body: {
            jobName: $('#cleanup-job-name').value.trim(),
            statuses: status ? [status] : [],
            startedAfter: start ? new Date(`${start}T00:00:00`).toISOString() : null,
            startedBefore: end ? new Date(`${end}T23:59:59.999`).toISOString() : null,
          },
        });
        $('#logs-advanced-dialog').close();
        [state.runs, state.settings] = await Promise.all([api('/api/v1/runs'), api('/api/v1/settings')]);
        renderRuns(); renderDashboard(); renderSettings();
        toast(`已清理 ${result.deleted} 条日志，释放 ${formatBytes(result.freedBytes)}`);
      } catch (error) { toast(error.message, true); }
    });
    $$('.log-close').forEach(x => x.addEventListener('click', () => {
      $('#log-dialog').close();
      state.openLogContent = '';
    }));
    $('#system-tabs').addEventListener('click', event => {
      const button = event.target.closest('[data-system-tab]');
      if (!button) return;
      const tab = button.dataset.systemTab;
      $$('#system-tabs button').forEach(item => item.classList.toggle('active', item === button));
      $('#system-tab-overview').classList.toggle('hidden', tab !== 'overview');
      $('#system-tab-security').classList.toggle('hidden', tab !== 'security');
    });
    $('#password-form').addEventListener('submit', async event => {
      event.preventDefault();
      const newPassword = $('#new-password').value;
      const confirmation = $('#confirm-password').value;
      if (newPassword !== confirmation) {
        $('#confirm-password').setCustomValidity('两次输入的密码不一致');
        $('#confirm-password').reportValidity();
        return;
      }
      $('#confirm-password').setCustomValidity('');
      const button = $('#password-save'); button.disabled = true;
      try {
        const result = await api('/api/v1/auth/password', { method: 'PUT', body: { newPassword } });
        state.csrf = result.csrfToken;
        $('#new-password').value = ''; $('#confirm-password').value = '';
        renderSystem(); toast('管理员密码已更新');
      } catch (error) { toast(error.message, true); }
      finally { button.disabled = false; }
    });
    $('#confirm-password').addEventListener('input', () => $('#confirm-password').setCustomValidity(''));
    $('#password-protection').addEventListener('change', event => {
      $('#password-protection-warning').classList.toggle('hidden', event.target.checked);
      updateSecurityDirtyState();
    });
    $('#password-min-decrease').addEventListener('click', () => {
      $('#password-min-length').value = Math.max(1, Number($('#password-min-length').value) - 1);
      $('#password-min-length').dispatchEvent(new Event('input', { bubbles: true }));
    });
    $('#password-min-increase').addEventListener('click', () => {
      $('#password-min-length').value = Math.min(64, Number($('#password-min-length').value) + 1);
      $('#password-min-length').dispatchEvent(new Event('input', { bubbles: true }));
    });
    $('#password-min-length').addEventListener('input', event => {
      const value = Number(event.target.value);
      $('#password-min-decrease').disabled = value <= 1;
      $('#password-min-increase').disabled = value >= 64;
      updateSecurityDirtyState();
    });
    $('#auth-settings-form').addEventListener('change', updateSecurityDirtyState);
    $('#auth-settings-reset').addEventListener('click', () => {
      if (!state.savedAuthSettings) return;
      state.authSettings = structuredClone(state.savedAuthSettings);
      renderSystem();
      toast('安全设置已还原');
    });
    $('#auth-settings-form').addEventListener('submit', async event => {
      event.preventDefault();
      try {
        const wasEnabled = state.authSettings?.passwordProtectionEnabled !== false;
        const settings = securityFormSettings();
        state.authSettings = await api('/api/v1/auth/settings', {
          method: 'PUT', body: settings,
        });
        state.savedAuthSettings = structuredClone(state.authSettings);
        state.auth = { ...(state.auth || {}), passwordProtectionEnabled: state.authSettings.passwordProtectionEnabled };
        renderSystem(); toast('安全设置已保存');
        if (!wasEnabled && state.authSettings.passwordProtectionEnabled) {
          clearSession(true);
        }
      } catch (error) { toast(error.message, true); }
    });
    $('#update-check').addEventListener('click', async () => {
      const button = $('#update-check'); button.disabled = true;
      state.updateChecked = true;
      $('#update-panel').classList.remove('hidden');
      $('#update-check-label').textContent = '检查中...';
      $('#update-status-title').textContent = '正在检查更新';
      $('#update-status-badge').textContent = '检查中';
      $('#update-status-badge').className = 'badge-pill blue';
      $('#update-message').textContent = '正在连接更新服务，请稍候。';
      try {
        state.update = await api('/api/v1/update/check', { method: 'POST' });
        renderSystem();
        toast(state.update.available ? '发现可用更新' : '当前已是最新版本');
      } catch (error) {
        $('#update-status-title').textContent = '检查更新失败';
        $('#update-status-badge').textContent = '失败';
        $('#update-status-badge').className = 'badge-pill primary';
        $('#update-message').textContent = error.message;
        toast(error.message, true);
      }
      finally { button.disabled = false; $('#update-check-label').textContent = '检查更新'; }
    });
    $('#copy-web-url').addEventListener('click', async () => {
      const value = $('#system-web-url').textContent;
      try { await navigator.clipboard.writeText(value); toast('WebUI 地址已复制'); }
      catch { toast('复制失败，请手动复制地址', true); }
    });
    $('#system-restart').addEventListener('click', async () => {
      if (!confirm('确认重启 NCMM 服务？运行中的任务会被停止。')) return;
      const button = $('#system-restart'); button.disabled = true;
      try {
        await api('/api/v1/system/restart', { method: 'POST' });
        toast('NCMM 正在重启');
        setTimeout(() => location.reload(), 1800);
      } catch (error) { button.disabled = false; toast(error.message, true); }
    });
    window.addEventListener('popstate', () => navigate(routePages[location.pathname] || 'dashboard', { history: false }));
    if ('ResizeObserver' in window) {
      dashboardChartObserver = new ResizeObserver(entries => {
        const { width, height } = entries[0]?.contentRect || {};
        if (!state.authenticated || state.currentPage !== 'dashboard' || width < 1 || height < 1) return;
        const size = `${Math.max(620, Math.round(width))}x${Math.max(190, Math.round(height))}`;
        if ($('#dashboard-chart').dataset.chartSize === size) return;
        clearTimeout(dashboardChartResizeTimer);
        dashboardChartResizeTimer = setTimeout(() => renderDashboardChart(configuredAccounts()), 80);
      });
      dashboardChartObserver.observe($('#dashboard-chart'));
    }
  }

  async function boot() {
    applyTheme(localStorage.getItem('ncmm-theme') || 'dark');
    bindEvents();
    try {
      const auth = await publicApi('/api/v1/auth/status');
      if (auth.setupRequired) {
        showSetupView(auth);
        return;
      }
      if (auth.authenticated) {
        await enterApp(auth);
        return;
      }
    } catch (error) {
      showLoginView();
      showAuthError(error.message);
      return;
    } finally {
      document.documentElement.classList.remove('auth-pending');
    }
    showLoginView();
  }

  boot();
})();
