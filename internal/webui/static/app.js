(() => {
  'use strict';

  const state = {
    authenticated: false,
    csrf: '',
    auth: null,
    authMode: 'login',
    authSettings: null,
    authSessions: [],
    status: null,
    config: null,
    notify: null,
    configTarget: 'config',
    configMode: 'visual',
    configSection: '',
    configDirty: false,
    configDirtyMode: '',
    schedules: [],
    runs: [],
    settings: null,
    update: null,
    accountMain: true,
    accountMethod: 'cookie',
    qrcodeSession: null,
    qrcodePoller: null,
    qrcodeImageURL: '',
    poller: null,
    pollInFlight: false,
    pollGeneration: 0,
    lastOnlineAt: null,
    consecutivePollFailures: 0,
  };

  const pageTitles = { dashboard: '概览', config: '配置', accounts: '账号', schedules: '定时任务', runs: '运行记录', system: '系统' };
  const sectionLabels = {
    version: '配置版本', accounts: '账号', task: '批量任务', network: '网络', playids: '指定歌曲播放',
    sign: '签到', mixPlay: '混合播放', note: '图文笔记', dailySongShare: '每日推歌',
    vipMemberGift: '会员赠礼', musician: '音乐人', fansgroup: '乐迷团', database: '数据库',
    log: '应用日志', updater: '更新', notify: '通知',
    webhook: 'Webhook', bark: 'Bark', serverchan: 'Server 酱', telegram: 'Telegram', dingtalk: '钉钉',
    coolpush: 'CoolPush', pushplus: 'PushPlus', wecom_key: '企业微信群', wecom_app: '企业微信应用',
  };
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
  const configSectionOrder = ['accounts', 'task', 'network', 'playids', 'sign', 'mixPlay', 'note', 'dailySongShare', 'vipMemberGift', 'musician', 'fansgroup', 'database', 'log', 'updater', 'notify', 'version'];
  const notifySectionOrder = ['webhook', 'bark', 'serverchan', 'telegram', 'dingtalk', 'coolpush', 'pushplus', 'wecom_key', 'wecom_app'];
  const notifyFieldOrder = {
    webhook: ['enabled', 'url', 'method', 'headers', 'body_template'], bark: ['enabled', 'key', 'server'],
    serverchan: ['enabled', 'sckey'], telegram: ['enabled', 'bot_token', 'user_id', 'api_host', 'proxy'],
    dingtalk: ['enabled', 'access_token', 'secret'], coolpush: ['enabled', 'skey', 'mode'],
    pushplus: ['enabled', 'token'], wecom_key: ['enabled', 'key'],
    wecom_app: ['enabled', 'corp_id', 'corp_secret', 'to_user', 'agent_id', 'media_id'],
  };

  const $ = (selector, root = document) => root.querySelector(selector);
  const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];
  const escapeHTML = (value) => String(value ?? '').replace(/[&<>'"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[c]));
  const icon = (name) => `<svg aria-hidden="true"><use href="#i-${name}"></use></svg>`;
  const passwordMaxLength = 64;

  function passwordSettings() {
    return state.auth?.settings || {
      passwordMinLength: 8,
      passwordRequireLetters: true,
      passwordRequireDigits: true,
      passwordRequireSymbols: true,
    };
  }

  function passwordCharAllowed(char, settings) {
    if (!/^[\x21-\x7E]$/.test(char)) return false;
    if (!settings) return true;
    return (settings.passwordRequireLetters && /^[A-Za-z]$/.test(char))
      || (settings.passwordRequireDigits && /^\d$/.test(char))
      || (settings.passwordRequireSymbols && /^[!"#$%&'()*+,\-./:;<=>?@[\\\]^_`{|}~]$/.test(char));
  }

  function normalizePasswordInput(value, settings) {
    return Array.from(value).filter(char => passwordCharAllowed(char, settings)).join('').slice(0, passwordMaxLength);
  }

  function enabledPasswordTypesText(settings) {
    const labels = [];
    if (settings.passwordRequireLetters) labels.push('英文字母');
    if (settings.passwordRequireDigits) labels.push('数字');
    if (settings.passwordRequireSymbols) labels.push('符号');
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
      ['认证策略', '/api/v1/auth/settings', value => { state.authSettings = value; }],
      ['登录会话', '/api/v1/auth/sessions', value => { state.authSessions = value; }],
      ['版本信息', '/api/v1/update', value => { state.update = value; }],
      ['二维码状态', '/api/v1/accounts/qrcode', value => { state.qrcodeSession = value; }],
    ];
    const results = await Promise.allSettled(requests.map(([, path]) => api(path)));
    if (!state.authenticated) return;
    results.forEach((result, index) => {
      const [label, , apply] = requests[index];
      if (result.status === 'fulfilled') apply(result.value);
      else toast(`${label}加载失败：${result.reason.message}`, true);
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
    if (qrcodeSessionActive()) startQRCodePolling();
  }

  function renderAll() {
    renderStatus();
    renderConfig();
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
    $('#version-label').textContent = status.version ? String(status.version).split('\n')[0] : 'NCMM';
    $('#stat-scheduler').textContent = status.schedulerActive ? '运行中' : '启动中';
    $('#stat-timezone').textContent = status.timezone || '--';
    $('#stat-schedules').textContent = status.schedules ?? '--';
    $('#stat-running').textContent = status.running ?? '--';
    $('#stat-storage').textContent = formatBytes(status.logs?.sizeBytes || 0);
    $('#stat-files').textContent = `${status.logs?.files || 0} 个文件`;
  }

  function renderDashboard() {
    const upcoming = state.schedules.filter(x => x.enabled && x.nextRuns?.length).sort((a, b) => new Date(a.nextRuns[0]) - new Date(b.nextRuns[0])).slice(0, 5);
    $('#upcoming-list').className = `compact-list${upcoming.length ? '' : ' empty-state'}`;
    $('#upcoming-list').innerHTML = upcoming.length ? upcoming.map(job => `<div class="compact-row"><strong>${escapeHTML(job.name)}</strong><span><code>${escapeHTML(job.cron)}</code> · ${escapeHTML(job.command)}</span><time>${formatDate(job.nextRuns[0])}</time></div>`).join('') : '暂无定时任务';
    const recent = state.runs.slice(0, 5);
    $('#recent-runs').className = `compact-list${recent.length ? '' : ' empty-state'}`;
    $('#recent-runs').innerHTML = recent.length ? recent.map(run => `<div class="compact-row"><strong>${escapeHTML(run.jobName)}</strong><span>${escapeHTML(run.command)} ${escapeHTML((run.args || []).join(' '))}</span><span class="badge ${escapeHTML(run.status)}">${statusLabel(run.status)}</span></div>`).join('') : '暂无运行记录';
  }

  function renderConfig() {
    const document = activeDocument();
    const parseError = $('#config-parse-error');
    $('#config-dirty').classList.toggle('hidden', !state.configDirty);
    $$('#config-target button').forEach(button => button.classList.toggle('active', button.dataset.target === state.configTarget));
    if (!document) {
      parseError.textContent = '该配置暂时无法加载，请稍后重试。';
      parseError.classList.remove('hidden');
      return;
    }
    $('#yaml-editor').value = document.raw || '';
    parseError.textContent = document.parseError ? `YAML 解析失败，可在源码模式修复后保存：${document.parseError}` : '';
    parseError.classList.toggle('hidden', !document.parseError);
    if (document.parseError) state.configMode = 'yaml';
    $$('#config-mode button').forEach(button => button.classList.toggle('active', button.dataset.mode === state.configMode));
    $('#config-visual').classList.toggle('hidden', state.configMode !== 'visual');
    $('#config-yaml').classList.toggle('hidden', state.configMode !== 'yaml');
    if (!document.data) {
      $('#config-sections').innerHTML = '';
      $('#config-form').innerHTML = '';
      return;
    }
    const root = document.data;
    const keys = orderedEntries(root, []).map(([key]) => key);
    if (!state.configSection || !keys.includes(state.configSection)) state.configSection = keys[0] || '';
    $('#config-sections').innerHTML = keys.map(key => `<button data-section="${escapeHTML(key)}" class="${key === state.configSection ? 'active' : ''}">${escapeHTML(sectionLabels[key] || key)}</button>`).join('');
    const value = root[state.configSection];
    $('#config-form').innerHTML = renderConfigSection(state.configSection, value);
  }

  function renderConfigSection(key, value) {
    const title = sectionLabels[key] || key;
    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      return `<fieldset class="config-group"><legend>${escapeHTML(title)}${descriptionMarkup([key])}</legend><div class="config-fields">${orderedEntries(value, [key]).map(([childKey, child]) => renderConfigField([key, childKey], childKey, child)).join('')}</div></fieldset>`;
    }
    return `<fieldset class="config-group"><legend>${escapeHTML(title)}${descriptionMarkup([key])}</legend><div class="config-fields">${renderConfigField([key], key, value)}</div></fieldset>`;
  }

  function renderConfigField(path, key, value) {
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
      return `<fieldset class="config-group config-field full"><legend>${escapeHTML(label)}${descriptionMarkup(path)}</legend><div class="config-fields">${orderedEntries(value, path).map(([childKey, child]) => renderConfigField([...path, childKey], childKey, child)).join('')}</div></fieldset>`;
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
    else if (target.dataset.kind === 'number') value = Number(target.value);
    else if (target.dataset.kind === 'array') value = target.value.split('\n').map(x => x.trim()).filter(Boolean);
    else if (target.dataset.kind === 'json') {
      try { value = JSON.parse(target.value); target.setCustomValidity(''); }
      catch { target.setCustomValidity('JSON 格式无效'); return; }
    } else value = target.value;
    setByPath(activeDocument().data, path, value);
    state.configDirty = true;
    state.configDirtyMode = 'visual';
    $('#config-dirty').classList.remove('hidden');
  }

  function setByPath(root, path, value) {
    let cursor = root;
    path.slice(0, -1).forEach(key => { cursor = cursor[key]; });
    cursor[path[path.length - 1]] = value;
  }

  async function saveConfig() {
    const button = $('#config-save');
    button.disabled = true;
    try {
      const document = activeDocument();
      const body = state.configMode === 'yaml'
        ? { revision: document.revision, raw: $('#yaml-editor').value }
        : { revision: document.revision, data: document.data };
      const updated = await api(activeEndpoint(), { method: 'PUT', body });
      if (state.configTarget === 'notify') state.notify = updated;
      else state.config = updated;
      state.configDirty = false;
      state.configDirtyMode = '';
      renderConfig();
      toast('配置已保存，原文件已备份');
    } catch (error) { toast(error.message, true); }
    finally { button.disabled = false; }
  }

  function renderSchedules() {
    const tbody = $('#schedule-table');
    if (!state.schedules.length) {
      tbody.innerHTML = '<tr><td colspan="6" class="empty-state">暂无定时任务</td></tr>';
      return;
    }
    tbody.innerHTML = state.schedules.map(job => `<tr>
      <td><strong>${escapeHTML(job.name)}</strong>${job.readOnly ? '<br><small class="muted">环境变量托管</small>' : ''}</td>
      <td><code>${escapeHTML(job.cron)}</code></td>
      <td><code>${escapeHTML(job.command)} ${escapeHTML((job.args || []).join(' '))}</code></td>
      <td>${job.nextRuns?.[0] ? formatDate(job.nextRuns[0]) : '--'}</td>
      <td><span class="badge ${job.running ? 'running' : job.queued ? 'queued' : job.enabled ? 'enabled' : 'disabled'}">${job.running ? `运行中${job.queued ? ` · 排队 ${job.queued}` : ''}` : job.queued ? `排队 ${job.queued}` : job.enabled ? '已启用' : '已停用'}</span></td>
      <td><div class="row-actions">
        <button class="icon-button" data-action="run" data-id="${escapeHTML(job.id)}" title="立即运行">${icon('play')}</button>
        <button class="icon-button" data-action="edit" data-id="${escapeHTML(job.id)}" title="编辑" ${job.readOnly ? 'disabled' : ''}>${icon('edit')}</button>
        <button class="icon-button" data-action="delete" data-id="${escapeHTML(job.id)}" title="删除" ${job.readOnly ? 'disabled' : ''}>${icon('trash')}</button>
      </div></td>
    </tr>`).join('');
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
    if (!state.runs.length) {
      tbody.innerHTML = '<tr><td colspan="6" class="empty-state">暂无运行记录</td></tr>';
      return;
    }
    tbody.innerHTML = state.runs.map(run => `<tr>
      <td><strong>${escapeHTML(run.jobName)}</strong></td>
      <td><code>${escapeHTML(run.command)} ${escapeHTML((run.args || []).join(' '))}</code></td>
      <td>${formatDate(run.startedAt)}</td><td>${duration(run.startedAt, run.finishedAt)}</td>
      <td><span class="badge ${escapeHTML(run.status)}">${statusLabel(run.status)}</span></td>
      <td><div class="row-actions"><button class="icon-button" data-run-action="log" data-id="${escapeHTML(run.id)}" title="查看日志">${icon('eye')}</button>${['running', 'queued'].includes(run.status) ? `<button class="icon-button" data-run-action="stop" data-id="${escapeHTML(run.id)}" title="停止">${icon('stop')}</button>` : ''}</div></td>
    </tr>`).join('');
  }

  function renderSettings() {
    if (!state.settings) return;
    $('#retention-days').value = state.settings.logs.retentionDays;
    $('#max-log-size').value = state.settings.logs.maxTotalSizeMB;
    $('#timezone-input').value = state.settings.timezone;
    $('#max-parallel').value = state.settings.concurrency?.maxParallel || 1;
    $('#logs-summary').textContent = `${state.settings.stats.files} 个文件 · ${formatBytes(state.settings.stats.sizeBytes)}`;
  }

  function renderSystem() {
    if (state.authSettings) {
      $('#password-min-length').value = state.authSettings.passwordMinLength;
      $('#new-password').minLength = state.authSettings.passwordMinLength;
      $('#require-letters').checked = !!state.authSettings.passwordRequireLetters;
      $('#require-digits').checked = !!state.authSettings.passwordRequireDigits;
      $('#require-symbols').checked = !!state.authSettings.passwordRequireSymbols;
      $('#session-ttl').value = String(state.authSettings.sessionTTLSeconds);
      $('#idle-timeout').value = String(state.authSettings.idleTimeoutSeconds);
    }
    if (state.auth) {
      $('#auth-mode').textContent = state.auth.secureCookie ? 'Secure Session' : 'Session';
      $('#session-auth-note').textContent = '浏览器仅使用 HttpOnly Session Cookie，认证信息不会写入浏览器存储。';
    }
    const sessions = state.authSessions || [];
    $('#sessions-list').classList.toggle('empty-state', !sessions.length);
    $('#sessions-list').innerHTML = sessions.length ? sessions.map(session => `
      <div class="compact-row">
        <div><strong>${session.current ? '当前会话' : '其他会话'}</strong><span>${escapeHTML(session.ip || '未知来源')}</span></div>
        <span class="session-client">${escapeHTML(session.userAgent || '未知客户端')}</span>
        <span>最近活动 ${formatDate(session.lastSeen)}<br>到期 ${formatDate(session.expiresAt)}</span>
        <button class="button ${session.current ? 'subtle' : 'danger'}" data-session-id="${escapeHTML(session.id)}" ${session.current ? 'disabled' : ''}>${session.current ? '当前' : '撤销'}</button>
      </div>`).join('') : '暂无登录会话';
    $('#sessions-revoke-others').disabled = sessions.filter(session => !session.current).length === 0;
    const update = state.update;
    if (!update) return;
    $('#update-current').textContent = update.current_version || '--';
    $('#update-latest').textContent = update.latest_version || '--';
    const checked = update.last_check_time ? new Date(update.last_check_time) : null;
    const hasChecked = checked && checked.getFullYear() > 1971;
    $('#update-checked').textContent = hasChecked ? formatDate(checked) : '--';
    $('#update-platform').textContent = `${update.docker ? 'Docker · ' : ''}${update.os || '--'} / ${update.arch || '--'}`;
    const badge = $('#update-badge');
    badge.className = `badge ${update.restartRequired ? 'running' : update.available ? 'enabled' : update.update_status === 'failed' ? 'failed' : 'disabled'}`;
    badge.textContent = update.restartRequired ? '等待重启' : update.available ? '发现新版本' : update.update_status === 'failed' ? '更新失败' : hasChecked ? '已是最新' : '未检查';
    $('#update-apply').disabled = !update.canApply;
    $('#update-message').textContent = update.message || update.last_error || (update.docker ? 'Docker 镜像通过容器编排工具更新。' : '');
    const notes = $('#release-notes');
    notes.textContent = update.release_notes || '';
    notes.classList.toggle('hidden', !update.release_notes);
  }

  function setServiceStatus(status) {
    const node = $('#service-status');
    const labels = { online: '在线', reconnecting: '重连中', offline: '离线' };
    node.dataset.state = status;
    node.lastChild.textContent = labels[status] || status;
    if (status === 'online') state.lastOnlineAt = new Date();
    node.title = state.lastOnlineAt ? `最近在线：${state.lastOnlineAt.toLocaleTimeString('zh-CN', { hour12: false })}` : '';
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

  function navigate(page) {
    $$('.nav-item').forEach(x => x.classList.toggle('active', x.dataset.page === page));
    $$('.page').forEach(x => x.classList.toggle('active', x.id === `page-${page}`));
    $('#page-title').textContent = pageTitles[page];
    $('#sidebar').classList.remove('open');
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

  function formatDate(value) {
    if (!value) return '--';
    return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }).format(new Date(value));
  }

  function duration(start, end) {
    const seconds = Math.max(0, Math.floor((new Date(end || Date.now()) - new Date(start)) / 1000));
    if (seconds < 60) return `${seconds}秒`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}分${seconds % 60}秒`;
    return `${Math.floor(seconds / 3600)}时${Math.floor((seconds % 3600) / 60)}分`;
  }

  function formatBytes(bytes) {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 ** 3) return `${(bytes / 1024 ** 2).toFixed(1)} MB`;
    return `${(bytes / 1024 ** 3).toFixed(1)} GB`;
  }

  async function refreshOpenLog() {
    const dialog = $('#log-dialog');
    const content = await api(`/api/v1/runs/${encodeURIComponent(dialog.dataset.runId)}/log`);
    const pre = $('#log-content');
    const nearBottom = pre.scrollHeight - pre.scrollTop - pre.clientHeight < 80;
    pre.textContent = content;
    if (nearBottom) pre.scrollTop = pre.scrollHeight;
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
      document.documentElement.dataset.theme = dark ? 'light' : 'dark';
      localStorage.setItem('ncmm-theme', dark ? 'light' : 'dark');
    });
    $('#menu-button').addEventListener('click', () => $('#sidebar').classList.toggle('open'));
    $$('.nav-item').forEach(button => button.addEventListener('click', () => navigate(button.dataset.page)));
    $$('[data-go]').forEach(button => button.addEventListener('click', () => navigate(button.dataset.go)));

    $('#config-sections').addEventListener('click', event => {
      const button = event.target.closest('[data-section]');
      if (!button) return;
      state.configSection = button.dataset.section; renderConfig();
    });
    $('#config-form').addEventListener('input', event => { if (event.target.matches('.config-control')) updateConfigControl(event.target); });
    $('#config-form').addEventListener('change', event => { if (event.target.matches('.config-control')) updateConfigControl(event.target); });
    $('#yaml-editor').addEventListener('input', () => { state.configDirty = true; state.configDirtyMode = 'yaml'; $('#config-dirty').classList.remove('hidden'); });
    $('#config-target').addEventListener('click', async event => {
      const button = event.target.closest('[data-target]'); if (!button || button.dataset.target === state.configTarget) return;
      if (state.configDirty && !confirm('切换配置文件会放弃当前未保存修改，是否继续？')) return;
      if (state.configDirty) {
        try {
          const updated = await api(activeEndpoint());
          if (state.configTarget === 'notify') state.notify = updated; else state.config = updated;
        } catch (error) { toast(error.message, true); return; }
      }
      state.configTarget = button.dataset.target;
      state.configSection = '';
      state.configDirty = false;
      state.configDirtyMode = '';
      renderConfig();
    });
    $('#config-mode').addEventListener('click', async event => {
      const button = event.target.closest('[data-mode]'); if (!button) return;
      if (state.configDirty && state.configDirtyMode && state.configDirtyMode !== button.dataset.mode) {
        if (!confirm('切换编辑模式会放弃当前未保存修改，是否继续？')) return;
        try {
          const updated = await api(activeEndpoint());
          if (state.configTarget === 'notify') state.notify = updated; else state.config = updated;
          state.configDirty = false; state.configDirtyMode = ''; renderConfig();
        }
        catch (error) { toast(error.message, true); return; }
      }
      state.configMode = button.dataset.mode;
      $$('#config-mode button').forEach(x => x.classList.toggle('active', x === button));
      $('#config-visual').classList.toggle('hidden', state.configMode !== 'visual');
      $('#config-yaml').classList.toggle('hidden', state.configMode !== 'yaml');
    });
    $('#config-save').addEventListener('click', saveConfig);
    $('#config-reload').addEventListener('click', async () => {
      try {
        const updated = await api(activeEndpoint());
        if (state.configTarget === 'notify') state.notify = updated; else state.config = updated;
        state.configDirty = false; state.configDirtyMode = ''; renderConfig(); toast('配置已重新加载');
      }
      catch (error) { toast(error.message, true); }
    });

    $$('#account-login-method button').forEach(button => button.addEventListener('click', () => {
      if (qrcodeSessionActive()) {
        toast('请先完成或取消当前二维码登录', true);
        return;
      }
      setAccountMethod(button.dataset.loginMethod);
    }));
    $$('.account-type button').forEach(button => button.addEventListener('click', () => {
      if (qrcodeSessionActive()) return;
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
          state.qrcodeSession = await api('/api/v1/accounts/qrcode', { method: 'POST', body: {
            filename: $('#account-filename').value, main: state.accountMain,
          }});
          renderQRCodeSession();
          startQRCodePolling();
        } catch (error) {
          toast(error.message, true);
          submit.disabled = false;
        }
        return;
      }
      try {
        const result = await api('/api/v1/accounts/cookie', { method: 'POST', body: {
          content: $('#cookie-content').value, format: $('#cookie-format').value,
          filename: $('#account-filename').value, main: state.accountMain,
        }});
        $('#cookie-content').value = '';
        $('#cookie-result').textContent = result.message || `已导入 ${result.filename}`;
        $('#cookie-result').classList.remove('hidden');
        state.config = await api('/api/v1/config'); renderConfig();
        toast('Cookie 验证并导入成功');
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
    $$('.dialog-close').forEach(x => x.addEventListener('click', () => $('#schedule-dialog').close()));
    $('#schedule-form').addEventListener('submit', saveSchedule);
    $('#cron-preset').addEventListener('change', event => { if (event.target.value !== 'custom') $('#schedule-cron').value = event.target.value; });
    $('#schedule-table').addEventListener('click', async event => {
      const button = event.target.closest('[data-action]'); if (!button) return;
      const job = state.schedules.find(x => x.id === button.dataset.id); if (!job) return;
      try {
        if (button.dataset.action === 'edit') openScheduleDialog(job);
        if (button.dataset.action === 'run') { await api(`/api/v1/schedules/${encodeURIComponent(job.id)}/run`, { method: 'POST' }); toast('任务已启动'); navigate('runs'); await refreshOperationalState(); }
        if (button.dataset.action === 'delete' && confirm(`删除定时任务“${job.name}”？`)) { await api(`/api/v1/schedules/${encodeURIComponent(job.id)}`, { method: 'DELETE' }); await refreshSchedules(); toast('定时任务已删除'); }
      } catch (error) { toast(error.message, true); }
    });

    $('#runs-table').addEventListener('click', async event => {
      const button = event.target.closest('[data-run-action]'); if (!button) return;
      const run = state.runs.find(x => x.id === button.dataset.id); if (!run) return;
      try {
        if (button.dataset.runAction === 'log') { $('#log-dialog').dataset.runId = run.id; $('#log-title').textContent = run.jobName; await refreshOpenLog(); $('#log-dialog').showModal(); }
        if (button.dataset.runAction === 'stop') { await api(`/api/v1/runs/${encodeURIComponent(run.id)}/stop`, { method: 'POST' }); toast('正在停止任务'); }
      } catch (error) { toast(error.message, true); }
    });
    $$('.log-close').forEach(x => x.addEventListener('click', () => $('#log-dialog').close()));
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
        state.authSessions = await api('/api/v1/auth/sessions');
        renderSystem(); toast('管理员密码已修改，旧会话已全部撤销');
      } catch (error) { toast(error.message, true); }
      finally { button.disabled = false; }
    });
    $('#confirm-password').addEventListener('input', () => $('#confirm-password').setCustomValidity(''));
    $('#auth-settings-form').addEventListener('submit', async event => {
      event.preventDefault();
      try {
        state.authSettings = await api('/api/v1/auth/settings', { method: 'PUT', body: {
          passwordMinLength: Number($('#password-min-length').value),
          passwordRequireLetters: $('#require-letters').checked,
          passwordRequireDigits: $('#require-digits').checked,
          passwordRequireSymbols: $('#require-symbols').checked,
          sessionTTLSeconds: Number($('#session-ttl').value),
          idleTimeoutSeconds: Number($('#idle-timeout').value),
        }});
        renderSystem(); toast('密码与会话策略已保存');
      } catch (error) { toast(error.message, true); }
    });
    $('#sessions-list').addEventListener('click', async event => {
      const button = event.target.closest('[data-session-id]');
      if (!button || button.disabled) return;
      try {
        await api(`/api/v1/auth/sessions/${encodeURIComponent(button.dataset.sessionId)}`, { method: 'DELETE' });
        state.authSessions = await api('/api/v1/auth/sessions'); renderSystem(); toast('会话已撤销');
      } catch (error) { toast(error.message, true); }
    });
    $('#sessions-revoke-others').addEventListener('click', async () => {
      try {
        await api('/api/v1/auth/sessions/revoke-others', { method: 'POST' });
        state.authSessions = await api('/api/v1/auth/sessions'); renderSystem(); toast('其他会话已全部撤销');
      } catch (error) { toast(error.message, true); }
    });
    $('#update-check').addEventListener('click', async () => {
      const button = $('#update-check'); button.disabled = true; $('#update-apply').disabled = true;
      try {
        state.update = await api('/api/v1/update/check', { method: 'POST' });
        renderSystem();
        toast(state.update.available ? '发现可用更新' : '当前已是最新版本');
      } catch (error) { toast(error.message, true); }
      finally { button.disabled = false; if (state.update) $('#update-apply').disabled = !state.update.canApply; }
    });
    $('#update-apply').addEventListener('click', async () => {
      if (!state.update?.canApply || !confirm(`更新到 ${state.update.latest_version}？`)) return;
      const button = $('#update-apply'); button.disabled = true; $('#update-check').disabled = true;
      try {
        state.update = await api('/api/v1/update/apply', { method: 'POST' });
        renderSystem();
        toast('更新已安装，重启后生效');
      } catch (error) { toast(error.message, true); }
      finally { $('#update-check').disabled = false; button.disabled = !state.update?.canApply; }
    });
    $('#settings-save').addEventListener('click', async () => {
      try {
        state.settings = await api('/api/v1/settings', { method: 'PUT', body: {
          timezone: $('#timezone-input').value.trim(), logs: { retentionDays: Number($('#retention-days').value), maxTotalSizeMB: Number($('#max-log-size').value) },
          concurrency: { maxParallel: Number($('#max-parallel').value) },
        }});
        renderSettings(); await refreshSchedules(); toast('日志保留设置已保存');
      } catch (error) { toast(error.message, true); }
    });
    $('#logs-cleanup').addEventListener('click', async () => {
      try { await api('/api/v1/logs/cleanup', { method: 'POST' }); state.settings = await api('/api/v1/settings'); renderSettings(); toast('日志清理完成'); }
      catch (error) { toast(error.message, true); }
    });
  }

  async function boot() {
    document.documentElement.dataset.theme = localStorage.getItem('ncmm-theme') || (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
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
    }
    showLoginView();
  }

  boot();
})();
