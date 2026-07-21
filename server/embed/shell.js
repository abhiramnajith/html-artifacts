// html-artifacts editor shell.
//
// Injected by the server into every /view/<id> page. Lets you annotate the
// artifact — pick an element or select text, attach a comment — and send the
// batch back to the agent as <id>.annotations.json.
//
// Design notes:
//   * All UI lives in a Shadow DOM host so the editor's CSS never bleeds into
//     the artifact (and vice-versa).
//   * No alert/confirm/prompt — those block the page; feedback is a DOM toast.
//   * Identity (which artifact) comes from the URL; the server re-stamps it.
(function () {
  "use strict";

  var m = location.pathname.match(/\/view\/([a-z0-9-]+)$/);
  if (!m) return; // only runs on /view/<id>
  var artifactId = m[1];

  var annotations = []; // {selector, selectedText, comment}
  var active = false;

  // ---- Shadow-DOM UI -------------------------------------------------------
  var host = document.createElement("div");
  host.id = "ha-editor-host";
  var root = host.attachShadow({ mode: "open" });
  document.documentElement.appendChild(host);

  root.innerHTML =
    '<style>' +
    ':host{all:initial}' +
    '*{box-sizing:border-box;font-family:ui-sans-serif,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}' +
    '.bar{position:fixed;right:16px;bottom:16px;z-index:2147483647;display:flex;gap:8px;align-items:center;' +
    'background:#14181e;color:#e7ecf2;border:1px solid #2a323d;border-radius:999px;padding:6px 8px 6px 14px;' +
    'box-shadow:0 8px 30px -8px rgba(0,0,0,.5);font-size:13px}' +
    '.bar button{font:inherit;cursor:pointer;border:0;border-radius:999px;padding:7px 12px;color:#e7ecf2;background:#232b36}' +
    '.bar button.primary{background:#2f45c5;color:#fff}' +
    '.bar button.on{background:#2f45c5;color:#fff}' +
    '.bar button:disabled{opacity:.45;cursor:default}' +
    '.count{font-variant-numeric:tabular-nums;color:#a2adba;min-width:1.2em;text-align:center}' +
    '.hint{position:fixed;left:50%;bottom:70px;transform:translateX(-50%);z-index:2147483647;' +
    'background:#2f45c5;color:#fff;padding:6px 12px;border-radius:8px;font-size:12px;opacity:0;transition:opacity .15s;pointer-events:none}' +
    '.hint.show{opacity:1}' +
    '.pop{position:absolute;z-index:2147483647;width:300px;max-width:90vw;background:#14181e;color:#e7ecf2;' +
    'border:1px solid #2a323d;border-radius:12px;box-shadow:0 12px 40px -10px rgba(0,0,0,.6);padding:12px;display:none}' +
    '.pop .tgt{font:12px ui-monospace,Menlo,monospace;color:#8b98ff;background:#191d33;border-radius:6px;' +
    'padding:5px 7px;margin-bottom:8px;word-break:break-all;max-height:64px;overflow:auto}' +
    '.pop textarea{width:100%;min-height:70px;resize:vertical;background:#0d1014;color:#e7ecf2;border:1px solid #2a323d;' +
    'border-radius:8px;padding:8px;font:inherit;font-size:13px}' +
    '.pop .row{display:flex;justify-content:flex-end;gap:8px;margin-top:8px}' +
    '.pop button{font:inherit;font-size:13px;cursor:pointer;border:0;border-radius:8px;padding:7px 12px;color:#e7ecf2;background:#232b36}' +
    '.pop button.primary{background:#2f45c5;color:#fff}' +
    '.toast{position:fixed;left:50%;bottom:70px;transform:translateX(-50%);z-index:2147483647;' +
    'background:#1f7a4d;color:#fff;padding:8px 14px;border-radius:8px;font-size:13px;opacity:0;transition:opacity .2s;pointer-events:none}' +
    '.toast.show{opacity:1}.toast.err{background:#b02a37}' +
    '</style>' +
    '<div class="bar">' +
    '<button id="toggle">✎ Annotate</button>' +
    '<span class="count" id="count">0</span>' +
    '<button id="send" class="primary" disabled>Send to agent</button>' +
    '</div>' +
    '<div class="hint" id="hint">Click an element, or select text, to comment</div>' +
    '<div class="pop" id="pop">' +
    '<div class="tgt" id="tgt"></div>' +
    '<textarea id="comment" placeholder="Describe the change you want…"></textarea>' +
    '<div class="row"><button id="cancel">Cancel</button><button id="add" class="primary">Add comment</button></div>' +
    '</div>';

  var $ = function (id) { return root.getElementById(id); };
  var toggleBtn = $("toggle"), sendBtn = $("send"), countEl = $("count");
  var hintEl = $("hint"), popEl = $("pop"), tgtEl = $("tgt"), commentEl = $("comment");

  // ---- Highlight overlay (light DOM, non-interactive) ----------------------
  var hi = document.createElement("div");
  hi.style.cssText =
    "position:fixed;z-index:2147483646;pointer-events:none;border:2px solid #2f45c5;" +
    "background:rgba(47,69,197,.12);border-radius:3px;display:none;transition:all .04s linear";
  document.documentElement.appendChild(hi);

  var pending = null; // element being commented on

  function isOurs(node) { return node === host || node === hi || (node && node.getRootNode && node.getRootNode() === root); }

  // ---- CSS selector generation ---------------------------------------------
  function cssPath(el) {
    if (!el || el.nodeType !== 1) return "";
    if (el.id) return "#" + CSS.escape(el.id);
    var parts = [];
    var node = el;
    while (node && node.nodeType === 1 && node !== document.body) {
      if (node.id) { parts.unshift("#" + CSS.escape(node.id)); break; }
      var nth = 1, sib = node;
      while ((sib = sib.previousElementSibling)) {
        if (sib.nodeName === node.nodeName) nth++;
      }
      parts.unshift(node.nodeName.toLowerCase() + ":nth-of-type(" + nth + ")");
      node = node.parentElement;
    }
    return parts.join(" > ");
  }

  // ---- Interaction ---------------------------------------------------------
  function setActive(on) {
    active = on;
    toggleBtn.classList.toggle("on", on);
    toggleBtn.textContent = on ? "✕ Stop" : "✎ Annotate";
    hintEl.classList.toggle("show", on);
    if (!on) { hi.style.display = "none"; }
  }

  function onMove(e) {
    if (!active) return;
    var el = e.target;
    if (isOurs(el)) { hi.style.display = "none"; return; }
    var r = el.getBoundingClientRect();
    hi.style.display = "block";
    hi.style.left = r.left + "px";
    hi.style.top = r.top + "px";
    hi.style.width = r.width + "px";
    hi.style.height = r.height + "px";
  }

  function onClick(e) {
    if (!active) return;
    if (isOurs(e.target)) return; // let our own UI work normally
    e.preventDefault();
    e.stopPropagation();

    var sel = window.getSelection();
    var text = sel && !sel.isCollapsed ? sel.toString().trim() : "";
    var el = e.target;
    if (text && sel.rangeCount) {
      var node = sel.getRangeAt(0).commonAncestorContainer;
      el = node.nodeType === 1 ? node : node.parentElement;
    }
    openPopover(el, text, e.clientX, e.clientY);
  }

  function openPopover(el, text, x, y) {
    pending = { selector: cssPath(el), selectedText: text || "" };
    tgtEl.textContent = pending.selector + (text ? "  —  “" + text.slice(0, 80) + "”" : "");
    commentEl.value = "";
    popEl.style.display = "block";
    var pw = 300, ph = popEl.offsetHeight || 160;
    var px = Math.min(x + window.scrollX, window.scrollX + window.innerWidth - pw - 12);
    var py = Math.min(y + window.scrollY + 12, window.scrollY + window.innerHeight - ph - 12);
    popEl.style.left = Math.max(window.scrollX + 8, px) + "px";
    popEl.style.top = Math.max(window.scrollY + 8, py) + "px";
    commentEl.focus();
  }

  function closePopover() { popEl.style.display = "none"; pending = null; }

  function addComment() {
    var c = commentEl.value.trim();
    if (!c || !pending) { closePopover(); return; }
    annotations.push({ selector: pending.selector, selectedText: pending.selectedText, comment: c });
    countEl.textContent = String(annotations.length);
    sendBtn.disabled = annotations.length === 0;
    closePopover();
  }

  var toastEl;
  function toast(msg, isErr) {
    if (!toastEl) { toastEl = document.createElement("div"); toastEl.className = "toast"; root.appendChild(toastEl); }
    toastEl.textContent = msg;
    toastEl.classList.toggle("err", !!isErr);
    toastEl.classList.add("show");
    setTimeout(function () { toastEl.classList.remove("show"); }, 2600);
  }

  function send() {
    if (!annotations.length) return;
    sendBtn.disabled = true;
    fetch("/annotations/" + artifactId, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ annotations: annotations })
    }).then(function (res) {
      if (!res.ok) throw new Error("HTTP " + res.status);
      toast("Sent " + annotations.length + " annotation" + (annotations.length === 1 ? "" : "s") + " to the agent");
      annotations = [];
      countEl.textContent = "0";
      setActive(false);
    }).catch(function (err) {
      toast("Failed to send: " + err.message, true);
      sendBtn.disabled = false;
    });
  }

  toggleBtn.addEventListener("click", function () { setActive(!active); });
  sendBtn.addEventListener("click", send);
  $("add").addEventListener("click", addComment);
  $("cancel").addEventListener("click", closePopover);
  document.addEventListener("mousemove", onMove, true);
  document.addEventListener("click", onClick, true);
  window.addEventListener("keydown", function (e) { if (e.key === "Escape") { closePopover(); setActive(false); } });

  // Show any previously-sent annotations count on load (non-blocking).
  fetch("/annotations/" + artifactId).then(function (r) {
    return r.ok ? r.json() : null;
  }).then(function (data) {
    if (data && data.annotations && data.annotations.length) {
      toast(data.annotations.length + " annotation(s) already pending for this artifact");
    }
  }).catch(function () {});
})();
