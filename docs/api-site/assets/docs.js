(() => {
  'use strict';

  const store = {
    get(key) { try { return localStorage.getItem(key); } catch { return null; } },
    set(key, value) { try { localStorage.setItem(key, value); } catch { /* 无痕模式等 */ } },
  };
  const root = document.documentElement;
  const body = document.body;

  /* ---- 复制与提示 ---- */
  const copyText = async text => {
    if (navigator.clipboard && window.isSecureContext) {
      try { await navigator.clipboard.writeText(text); return true; } catch { /* 走降级 */ }
    }
    const area = document.createElement('textarea');
    area.value = text;
    area.setAttribute('readonly', '');
    area.style.cssText = 'position:fixed;left:-100000px;top:0;opacity:0';
    document.body.appendChild(area);
    area.select();
    let ok = false;
    try { ok = document.execCommand('copy'); } catch { ok = false; }
    area.remove();
    return ok;
  };
  const toastEl = document.querySelector('[data-toast]');
  let toastTimer;
  const toast = message => {
    if (!toastEl) return;
    toastEl.querySelector('[data-toast-text]').textContent = message;
    toastEl.hidden = false;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => { toastEl.hidden = true; }, 1800);
  };
  const copyWithToast = async (text, message) => {
    toast(await copyText(text) ? message : '复制失败，请手动选择内容');
  };

  const pageUrl = () => location.origin + location.pathname;
  const mdUrl = () => new URL(body.dataset.md || 'index.md', location.href).href;
  const llmsUrl = () => new URL('llms.txt', location.href).href;

  /* ---- 分享按钮 ---- */
  document.querySelectorAll('[data-share]').forEach(btn => {
    btn.addEventListener('click', async () => {
      const kind = btn.dataset.share;
      if (kind === 'link') return copyWithToast(pageUrl(), '已复制页面链接');
      if (kind === 'prompt') {
        return copyWithToast(`请阅读 ${mdUrl()}，按其中的「Agent 调用规则」帮我对接这个接口：提交任务，并轮询直到完成。`, '已复制提示词，粘贴到 AI 助手即可');
      }
      if (kind === 'llms') {
        return copyWithToast(`请阅读 ${llmsUrl()}，这是 O1Key 图像与视频接口的文档索引。按需打开对应页面的 Markdown，帮我完成对接。`, '已复制链接和提示词');
      }
      if (kind === 'md') {
        try {
          const resp = await fetch(mdUrl(), { cache: 'no-cache' });
          if (!resp.ok) throw new Error(String(resp.status));
          return copyWithToast(await resp.text(), '已复制本页 Markdown');
        } catch {
          return copyWithToast(mdUrl(), '已复制 Markdown 链接');
        }
      }
    });
  });
  document.querySelectorAll('[data-llms-url]').forEach(el => { el.textContent = llmsUrl(); });

  /* ---- 阅读模式 ---- */
  const modes = document.querySelector('[data-modes]');
  const hint = document.querySelector('[data-mode-hint]');
  const HINTS = { simple: '用日常语言解释，适合第一次对接', full: '完整参数、示例、状态与错误格式' };
  const setMode = (mode, persist) => {
    if (!modes) return;
    root.dataset.mode = mode;
    modes.querySelectorAll('[data-mode]').forEach(b => {
      const on = b.dataset.mode === mode;
      b.setAttribute('aria-checked', String(on));
      b.tabIndex = on ? 0 : -1;
    });
    if (hint) hint.textContent = HINTS[mode];
    if (persist) store.set('docs.mode', mode);
  };
  if (modes) {
    const fromUrl = new URLSearchParams(location.search).get('mode');
    setMode(fromUrl === 'simple' || fromUrl === 'full' ? fromUrl : (root.dataset.mode === 'simple' ? 'simple' : 'full'), false);
    const buttons = Array.from(modes.querySelectorAll('[data-mode]'));
    buttons.forEach((b, i) => {
      b.addEventListener('click', () => setMode(b.dataset.mode, true));
      b.addEventListener('keydown', e => {
        if (e.key !== 'ArrowRight' && e.key !== 'ArrowLeft') return;
        e.preventDefault();
        const next = buttons[(i + (e.key === 'ArrowRight' ? 1 : buttons.length - 1)) % buttons.length];
        setMode(next.dataset.mode, true);
        next.focus();
      });
    });
    document.querySelectorAll('[data-mode-switch]').forEach(b => b.addEventListener('click', () => {
      setMode(b.dataset.modeSwitch, true);
      document.querySelector('[data-panel="full"]')?.scrollIntoView({ block: 'start' });
    }));
  }
  const revealHash = () => {
    if (!location.hash) return;
    let target = null;
    try { target = document.getElementById(decodeURIComponent(location.hash.slice(1))); } catch { return; }
    if (target && target.closest('[data-panel="full"]') && root.dataset.mode === 'simple') {
      setMode('full', false);
      target.scrollIntoView();
    }
  };
  window.addEventListener('hashchange', revealHash);

  /* ---- 通用选项卡 ---- */
  const wireTabs = (tabs, onSelect) => {
    tabs.forEach((tab, i) => {
      tab.addEventListener('click', () => onSelect(tab, false));
      tab.addEventListener('keydown', e => {
        let next = i;
        if (e.key === 'ArrowRight') next = (i + 1) % tabs.length;
        else if (e.key === 'ArrowLeft') next = (i - 1 + tabs.length) % tabs.length;
        else if (e.key === 'Home') next = 0;
        else if (e.key === 'End') next = tabs.length - 1;
        else return;
        e.preventDefault();
        onSelect(tabs[next], true);
      });
    });
  };

  /* 示例场景 */
  document.querySelectorAll('[data-example-switcher]').forEach(sw => {
    const tabs = Array.from(sw.querySelectorAll(':scope > .example-tabs [role="tab"]'));
    const panels = Array.from(sw.querySelectorAll(':scope > .example-panels > [role="tabpanel"]'));
    const activate = (next, focus) => {
      tabs.forEach(t => {
        const on = t === next;
        t.setAttribute('aria-selected', String(on));
        t.tabIndex = on ? 0 : -1;
      });
      panels.forEach(p => {
        const on = p.id === next.getAttribute('aria-controls');
        p.classList.toggle('is-active', on);
        p.setAttribute('aria-hidden', String(!on));
      });
      if (focus) next.focus();
    };
    wireTabs(tabs, activate);
    const selected = tabs.find(t => t.getAttribute('aria-selected') === 'true') || tabs[0];
    if (selected) activate(selected, false);
  });

  /* 代码语言，全站同步 */
  const langBlocks = Array.from(document.querySelectorAll('[data-langs]'));
  const setLang = (lang, persist) => {
    langBlocks.forEach(block => {
      block.querySelectorAll('[role="tab"]').forEach(t => {
        const on = t.dataset.lang === lang;
        t.setAttribute('aria-selected', String(on));
        t.tabIndex = on ? 0 : -1;
      });
      block.querySelectorAll('pre[data-lang]').forEach(p => { p.hidden = p.dataset.lang !== lang; });
    });
    if (persist) store.set('docs.lang', lang);
  };
  langBlocks.forEach(block => {
    wireTabs(Array.from(block.querySelectorAll('[role="tab"]')), (tab, focus) => {
      const top = tab.getBoundingClientRect().top;
      setLang(tab.dataset.lang, true);
      window.scrollBy(0, tab.getBoundingClientRect().top - top);
      if (focus) tab.focus();
    });
  });
  const savedLang = store.get('docs.lang');
  if (savedLang && savedLang !== 'curl') setLang(savedLang, false);

  document.querySelectorAll('[data-code]').forEach(block => {
    const btn = block.querySelector('.code-copy');
    if (!btn) return;
    btn.addEventListener('click', () => {
      const pre = Array.from(block.querySelectorAll('pre')).find(p => !p.hidden);
      if (pre) copyWithToast(pre.innerText.replace(/\n$/, ''), '已复制代码');
    });
  });

  document.querySelectorAll('[data-anchor]').forEach(a => {
    a.addEventListener('click', () => {
      setTimeout(() => copyWithToast(pageUrl() + a.getAttribute('href'), '已复制本节链接'), 0);
    });
  });

  /* ---- 模拟调用 ---- */
  const simData = (() => {
    const el = document.getElementById('sim-data');
    if (!el) return [];
    try { return JSON.parse(el.textContent); } catch { return []; }
  })();
  document.querySelectorAll('[data-sim]').forEach(sim => {
    if (simData.length < 2) { sim.hidden = true; return; }
    const stepsEl = sim.querySelector('[data-sim-steps]');
    const respEl = sim.querySelector('[data-sim-resp]');
    const runBtn = sim.querySelector('[data-sim-run]');
    let timers = [];
    const render = index => {
      stepsEl.replaceChildren(...simData.map((s, i) => {
        const chip = document.createElement('span');
        chip.className = 'sim-step' + (i < index || (i === index && i === simData.length - 1) ? ' is-done' : i === index ? ' is-current' : '');
        chip.textContent = (i < index || (i === index && i === simData.length - 1) ? '✓ ' : i === index ? '● ' : '') + s.label;
        return chip;
      }));
      respEl.textContent = index < 0 ? '点击「提交任务」查看完整流程' : simData[index].body;
    };
    render(-1);
    runBtn.addEventListener('click', () => {
      timers.forEach(clearTimeout);
      timers = simData.map((_, i) => setTimeout(() => render(i), i * 1400));
      runBtn.textContent = '重新提交';
    });
  });

  /* ---- 侧栏 ---- */
  const subLinks = Array.from(document.querySelectorAll('.nav-sub a[data-target]'));
  if (subLinks.length && 'IntersectionObserver' in window) {
    const setCurrent = id => subLinks.forEach(a => {
      if (a.dataset.target === id) a.setAttribute('aria-current', 'true');
      else a.removeAttribute('aria-current');
    });
    const observer = new IntersectionObserver(entries => {
      const visible = entries.filter(e => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)[0];
      if (visible) setCurrent(visible.target.id);
    }, { rootMargin: '-80px 0px -65% 0px' });
    subLinks.forEach(a => {
      const target = document.getElementById(a.dataset.target);
      if (target) observer.observe(target);
      a.addEventListener('click', () => {
        if (root.dataset.mode === 'simple') setMode('full', false);
        setCurrent(a.dataset.target);
        body.classList.remove('nav-open');
      });
    });
  }
  const navToggle = document.querySelector('[data-nav-toggle]');
  if (navToggle) {
    navToggle.addEventListener('click', () => {
      const open = body.classList.toggle('nav-open');
      navToggle.setAttribute('aria-expanded', String(open));
    });
    document.addEventListener('keydown', e => {
      if (e.key === 'Escape' && body.classList.contains('nav-open')) {
        body.classList.remove('nav-open');
        navToggle.setAttribute('aria-expanded', 'false');
        navToggle.focus();
      }
    });
  }

  /* ---- 搜索 ---- */
  const search = document.querySelector('[data-search]');
  if (search) {
    const input = search.querySelector('[data-search-input]');
    const results = search.querySelector('[data-search-results]');
    const kbd = search.querySelector('[data-kbd]');
    if (kbd && !/Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent)) kbd.textContent = 'Ctrl K';
    let index = null;
    let loading = null;
    let active = -1;
    const load = () => {
      if (index || loading) return loading;
      loading = fetch(new URL('assets/search-index.json', location.href), { cache: 'no-cache' })
        .then(r => r.json()).then(data => { index = data; }).catch(() => { index = []; });
      return loading;
    };
    const close = () => { results.hidden = true; input.setAttribute('aria-expanded', 'false'); active = -1; };
    const highlight = i => {
      const items = Array.from(results.querySelectorAll('a'));
      active = items.length ? (i + items.length) % items.length : -1;
      items.forEach((a, j) => a.setAttribute('aria-selected', String(j === active)));
      if (active >= 0) items[active].scrollIntoView({ block: 'nearest' });
    };
    const run = async () => {
      const q = input.value.trim().toLowerCase();
      if (!q) { close(); return; }
      await load();
      const terms = q.split(/\s+/);
      const scored = [];
      for (const e of index) {
        const title = e.t.toLowerCase();
        const hay = `${title} ${(e.g || '').toLowerCase()} ${(e.k || '').toLowerCase()}`;
        if (!terms.every(t => hay.includes(t))) continue;
        scored.push([terms.every(t => title.includes(t)) ? 0 : 1, e]);
      }
      scored.sort((a, b) => a[0] - b[0]);
      results.replaceChildren();
      const top = scored.slice(0, 30);
      if (!top.length) {
        const empty = document.createElement('div');
        empty.className = 'search-empty';
        empty.textContent = '没有找到相关接口或参数';
        results.append(empty);
      }
      top.forEach(([, e]) => {
        const a = document.createElement('a');
        a.href = e.u;
        a.setAttribute('role', 'option');
        const g = document.createElement('span');
        g.className = 'sr-group';
        g.textContent = e.g;
        const t = document.createElement('span');
        t.textContent = e.t;
        a.append(g, t);
        const hit = (e.k || '').split(' ').find(w => terms.some(term => w.toLowerCase().includes(term)));
        if (hit) {
          const k = document.createElement('span');
          k.className = 'sr-key';
          k.textContent = hit;
          a.append(k);
        }
        a.addEventListener('click', () => {
          close();
          if (e.u.includes('#') && root.dataset.mode === 'simple') setMode('full', false);
        });
        results.append(a);
      });
      results.hidden = false;
      input.setAttribute('aria-expanded', 'true');
      highlight(0);
    };
    input.addEventListener('focus', load);
    input.addEventListener('input', run);
    input.addEventListener('keydown', e => {
      if (results.hidden) return;
      if (e.key === 'ArrowDown') { e.preventDefault(); highlight(active + 1); }
      else if (e.key === 'ArrowUp') { e.preventDefault(); highlight(active - 1); }
      else if (e.key === 'Enter') {
        const a = results.querySelectorAll('a')[active];
        if (a) { e.preventDefault(); a.click(); location.href = a.href; }
      } else if (e.key === 'Escape') { close(); }
    });
    document.addEventListener('click', e => { if (!search.contains(e.target)) close(); });
    document.addEventListener('keydown', e => {
      if ((e.key === 'k' && (e.metaKey || e.ctrlKey)) || (e.key === '/' && !/input|textarea|select/i.test(document.activeElement.tagName))) {
        e.preventDefault();
        input.focus();
        input.select();
      }
    });
  }

  /* ---- 概览：清单与旧链接 ---- */
  document.querySelectorAll('[data-checklist] input[data-check]').forEach(box => {
    const key = 'docs.check.' + box.dataset.check;
    box.checked = store.get(key) === '1';
    box.addEventListener('change', () => store.set(key, box.checked ? '1' : '0'));
  });
  const legacy = document.getElementById('legacy-anchors');
  if (legacy && location.hash) {
    try {
      const map = JSON.parse(legacy.textContent);
      const id = decodeURIComponent(location.hash.slice(1));
      if (map[id] && !document.getElementById(id)) {
        location.replace(new URL(map[id] + (map[id].includes('#') ? '' : location.hash), location.href).href);
      }
    } catch { /* 忽略无效锚点 */ }
  }

  revealHash();
})();
