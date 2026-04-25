// Config drawer — opens a Monaco editor against /api/config so you
// can edit the YAML catalogue from the dashboard. Server validates +
// atomic-writes; the existing fswatch picks up the rename and
// triggers the normal hot-reload path.
//
// Monaco is loaded lazily on first open so the dashboard's first
// paint stays cheap.

(function () {
  const MONACO_VERSION = '0.52.0';
  const MONACO_BASE = `https://cdn.jsdelivr.net/npm/monaco-editor@${MONACO_VERSION}/min/vs`;

  const drawer = document.getElementById('config-drawer');
  const backdrop = document.getElementById('config-backdrop');
  const openBtn = document.getElementById('config-open');
  const closeBtn = document.getElementById('config-close');
  const saveBtn = document.getElementById('config-save');
  const statusEl = document.getElementById('config-status');
  const editorEl = document.getElementById('config-editor');
  if (!drawer || !openBtn) return;

  let editor = null;
  let etag = '';
  let initialValue = '';
  let monacoLoading = null;

  // Hot-reload triggers a full page refresh on every config save (and
  // on any static/* change while you're editing the editor itself).
  // Persist "drawer was open" in sessionStorage so the drawer pops
  // back open on the very next paint, and we re-fetch fresh content.
  const DRAWER_KEY = 'config-drawer-open';

  function setStatus(text, kind) {
    statusEl.textContent = text || '';
    statusEl.className = 'status-pill' + (kind ? ' ' + kind : '');
  }

  function isDirty() {
    return editor && editor.getValue() !== initialValue;
  }

  function loadMonaco() {
    if (monacoLoading) return monacoLoading;
    monacoLoading = new Promise(function (resolve, reject) {
      const s = document.createElement('script');
      s.src = MONACO_BASE + '/loader.js';
      s.onload = function () {
        window.require.config({ paths: { vs: MONACO_BASE } });
        window.require(['vs/editor/editor.main'], function () {
          resolve(window.monaco);
        }, reject);
      };
      s.onerror = reject;
      document.head.appendChild(s);
    });
    return monacoLoading;
  }

  async function ensureEditor() {
    if (editor) return editor;
    const monaco = await loadMonaco();
    const dark = matchMedia('(prefers-color-scheme: dark)').matches;
    editor = monaco.editor.create(editorEl, {
      value: '',
      language: 'yaml',
      theme: dark ? 'vs-dark' : 'vs',
      automaticLayout: true,
      fontSize: 13,
      tabSize: 2,
      minimap: { enabled: false },
      scrollBeyondLastLine: false,
      renderWhitespace: 'selection',
    });
    editor.addCommand(
      monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS,
      save
    );
    editor.onDidChangeModelContent(function () {
      setStatus(isDirty() ? 'unsaved changes' : '', isDirty() ? 'dirty' : '');
    });
    return editor;
  }

  async function open() {
    sessionStorage.setItem(DRAWER_KEY, '1');
    drawer.classList.add('open');
    backdrop.classList.add('open');
    document.body.classList.add('drawer-open');
    setStatus('loading…');
    try {
      const ed = await ensureEditor();
      const res = await fetch('/api/config', { cache: 'no-store' });
      if (!res.ok) throw new Error('GET ' + res.status);
      etag = res.headers.get('ETag') || '';
      const text = await res.text();
      initialValue = text;
      ed.setValue(text);
      setStatus('');
      ed.focus();
    } catch (err) {
      setStatus('load failed: ' + err.message, 'error');
    }
  }

  function close() {
    if (isDirty() && !confirm('Discard unsaved changes?')) return;
    sessionStorage.removeItem(DRAWER_KEY);
    drawer.classList.remove('open');
    backdrop.classList.remove('open');
    document.body.classList.remove('drawer-open');
  }

  async function save() {
    if (!editor) return;
    const body = editor.getValue();
    setStatus('saving…');
    saveBtn.disabled = true;
    try {
      const res = await fetch('/api/config', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/x-yaml',
          'If-Match': etag,
        },
        body: body,
      });
      if (!res.ok) {
        const msg = await res.text();
        throw new Error(msg.trim() || res.statusText);
      }
      etag = res.headers.get('ETag') || etag;
      initialValue = body;
      setStatus('saved — reloading…', 'ok');
      // The fswatch on config.yaml will broadcast a hot-reload tick
      // that refreshes the page. Nothing else to do here.
    } catch (err) {
      setStatus('save failed: ' + err.message, 'error');
    } finally {
      saveBtn.disabled = false;
    }
  }

  openBtn.addEventListener('click', open);
  closeBtn.addEventListener('click', close);
  backdrop.addEventListener('click', close);
  saveBtn.addEventListener('click', save);
  document.addEventListener('keydown', function (ev) {
    if (ev.key === 'Escape' && drawer.classList.contains('open')) close();
  });

  // If a hot-reload (or a save-triggered reload) just refreshed the
  // page while the drawer was open, re-open it immediately so the
  // user's flow isn't broken.
  if (sessionStorage.getItem(DRAWER_KEY) === '1') open();
})();
