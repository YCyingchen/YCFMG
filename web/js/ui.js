/* YCFMG 前端通用 UI 组件：提示、弹窗、右键菜单、格式化 */
(function () {
  "use strict";

  function toast(msg, ms) {
    var el = document.getElementById("toast");
    if (!el) { return; }
    el.textContent = msg;
    el.classList.add("on");
    clearTimeout(el._t);
    el._t = setTimeout(function () { el.classList.remove("on"); }, ms || 2000);
  }

  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) { n.className = cls; }
    if (text !== undefined && text !== null) { n.textContent = text; }
    return n;
  }

  function icon(id, cls) {
    var svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    if (cls) { svg.setAttribute("class", cls); }
    var use = document.createElementNS("http://www.w3.org/2000/svg", "use");
    use.setAttribute("href", "#" + id);
    svg.appendChild(use);
    return svg;
  }

  function fmtSize(n) {
    n = Number(n) || 0;
    if (n < 1024) { return n + " B"; }
    var units = ["KB", "MB", "GB", "TB", "PB"];
    var v = n / 1024, i = -1;
    while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
    return (v >= 100 ? v.toFixed(0) : v.toFixed(1)) + " " + units[i];
  }

  function pad(n) { return n < 10 ? ("0" + n) : ("" + n); }

  function fmtTime(ts, withTime) {
    if (!ts) { return "-"; }
    var d = new Date(ts * 1000);
    var s = d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate());
    if (withTime !== false) { s += " " + pad(d.getHours()) + ":" + pad(d.getMinutes()); }
    return s;
  }

  function fmtDay(ts) {
    if (!ts) { return "未知时间"; }
    var d = new Date(ts * 1000);
    var now = new Date();
    var today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
    var t = new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
    var diff = Math.round((today - t) / 86400000);
    var base = d.getFullYear() + " 年 " + (d.getMonth() + 1) + " 月 " + d.getDate() + " 日";
    if (diff === 0) { return "今天 · " + base; }
    if (diff === 1) { return "昨天 · " + base; }
    if (diff < 7 && diff > 0) { return diff + " 天前 · " + base; }
    return base;
  }

  function esc(s) {
    return String(s === undefined || s === null ? "" : s)
      .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  }

  function baseName(p) {
    if (!p) { return ""; }
    var parts = String(p).split("/");
    return parts[parts.length - 1] || p;
  }

  function dirName(p) {
    if (!p) { return ""; }
    var i = String(p).lastIndexOf("/");
    return i > 0 ? p.slice(0, i) : "/";
  }

  function joinPath(a, b) {
    if (!a) { return b; }
    if (a.charAt(a.length - 1) === "/") { return a + b; }
    return a + "/" + b;
  }

  function confirmBox(title, message, okText) {
    return new Promise(function (resolve) {
      var box = document.getElementById("modalBox");
      var wrap = document.getElementById("modal");
      box.innerHTML = "";
      box.appendChild(el("h2", null, title));
      if (message) { box.appendChild(el("p", "muted", message)); }
      var actions = el("div", "actions");
      var cancel = el("button", "btn", "取消");
      var okBtn = el("button", "btn primary", okText || "确定");
      actions.appendChild(cancel);
      actions.appendChild(okBtn);
      box.appendChild(actions);
      wrap.classList.remove("hidden");
      function close(v) { wrap.classList.add("hidden"); resolve(v); }
      cancel.onclick = function () { close(false); };
      okBtn.onclick = function () { close(true); };
      wrap.onclick = function (e) { if (e.target === wrap) { close(false); } };
    });
  }

  function promptBox(title, label, value, placeholder) {
    return new Promise(function (resolve) {
      var box = document.getElementById("modalBox");
      var wrap = document.getElementById("modal");
      box.innerHTML = "";
      box.appendChild(el("h2", null, title));
      var field = el("div", "field");
      field.appendChild(el("span", null, label));
      var input = el("input");
      input.value = value || "";
      if (placeholder) { input.placeholder = placeholder; }
      field.appendChild(input);
      box.appendChild(field);
      var actions = el("div", "actions");
      var cancel = el("button", "btn", "取消");
      var okBtn = el("button", "btn primary", "确定");
      actions.appendChild(cancel);
      actions.appendChild(okBtn);
      box.appendChild(actions);
      wrap.classList.remove("hidden");
      input.focus();
      input.select();
      function close(v) { wrap.classList.add("hidden"); resolve(v); }
      cancel.onclick = function () { close(null); };
      okBtn.onclick = function () { close(input.value); };
      input.onkeydown = function (e) {
        if (e.key === "Enter") { close(input.value); }
        if (e.key === "Escape") { close(null); }
      };
      wrap.onclick = function (e) { if (e.target === wrap) { close(null); } };
    });
  }

  function formBox(title, fields, okText) {
    return new Promise(function (resolve) {
      var box = document.getElementById("modalBox");
      var wrap = document.getElementById("modal");
      box.innerHTML = "";
      box.appendChild(el("h2", null, title));
      var inputs = {};
      fields.forEach(function (f) {
        var field = el("div", "field");
        field.appendChild(el("span", null, f.label));
        var input;
        if (f.type === "select") {
          input = el("select");
          (f.options || []).forEach(function (o) {
            var op = el("option", null, o.label);
            op.value = o.value;
            input.appendChild(op);
          });
          if (f.value !== undefined) { input.value = f.value; }
        } else if (f.type === "textarea") {
          input = el("textarea");
          input.rows = 3;
          input.value = f.value || "";
        } else {
          input = el("input");
          input.type = f.type || "text";
          input.value = f.value === undefined ? "" : f.value;
          if (f.placeholder) { input.placeholder = f.placeholder; }
        }
        if (f.name) { inputs[f.name] = input; }
        field.appendChild(input);
        box.appendChild(field);
      });
      var actions = el("div", "actions");
      var cancel = el("button", "btn", "取消");
      var okBtn = el("button", "btn primary", okText || "确定");
      actions.appendChild(cancel);
      actions.appendChild(okBtn);
      box.appendChild(actions);
      wrap.classList.remove("hidden");
      var first = box.querySelector("input,select,textarea");
      if (first) { first.focus(); }
      function close(v) { wrap.classList.add("hidden"); resolve(v); }
      cancel.onclick = function () { close(null); };
      okBtn.onclick = function () {
        var out = {};
        for (var k in inputs) { out[k] = inputs[k].value; }
        close(out);
      };
      wrap.onclick = function (e) { if (e.target === wrap) { close(null); } };
    });
  }

  var menuEl = null;

  function closeMenu() {
    if (menuEl) { menuEl.remove(); menuEl = null; }
  }

  function contextMenu(x, y, items) {
    closeMenu();
    var m = el("div", "ctxmenu");
    items.forEach(function (it) {
      if (it.sep) { m.appendChild(el("div", "sep")); return; }
      var b = el("button", it.danger ? "danger" : "");
      if (it.icon) { b.appendChild(icon(it.icon, "ic")); }
      b.appendChild(el("span", null, it.label));
      if (it.disabled) {
        b.disabled = true;
        b.style.opacity = ".45";
      } else {
        b.onclick = function () { closeMenu(); it.action(); };
      }
      m.appendChild(b);
    });
    document.body.appendChild(m);
    var w = m.offsetWidth, h = m.offsetHeight;
    if (x + w > window.innerWidth - 6) { x = window.innerWidth - w - 6; }
    if (y + h > window.innerHeight - 6) { y = window.innerHeight - h - 6; }
    m.style.left = x + "px";
    m.style.top = y + "px";
    menuEl = m;
  }

  document.addEventListener("click", closeMenu);
  document.addEventListener("contextmenu", function (e) {
    if (!e.target.closest(".ctxmenu")) { closeMenu(); }
  });

  function kindLabel(kind) {
    var map = { image: "图片", video: "视频", audio: "音频", archive: "压缩包", doc: "文档", folder: "文件夹", other: "文件" };
    return map[kind] || "文件";
  }

  function extBadge(name) {
    var i = String(name || "").lastIndexOf(".");
    if (i < 0) { return "FILE"; }
    return name.slice(i + 1, i + 5).toUpperCase();
  }

  window.UI = {
    toast: toast, el: el, icon: icon, esc: esc,
    fmtSize: fmtSize, fmtTime: fmtTime, fmtDay: fmtDay,
    baseName: baseName, dirName: dirName, joinPath: joinPath,
    confirm: confirmBox, prompt: promptBox, form: formBox,
    menu: contextMenu, closeMenu: closeMenu, kindLabel: kindLabel, extBadge: extBadge
  };
})();