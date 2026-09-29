/* YCFMG 文件管理视图：按 YCNAS 的资源管理器样式渲染（fx- 结构） */
(function () {
  "use strict";
  var E = UI.el, I = UI.icon;

  var state = {
    path: "", parent: "", entries: [], sel: [], anchor: -1,
    view: "details", sort: "name", desc: false,
    loading: false, writable: false, hidden: false
  };
  var clipboard = { mode: "", paths: [] };

  function rowsEl() { return document.getElementById("fxRows"); }
  function colsEl() { return document.getElementById("fxCols"); }
  function contentEl() { return document.getElementById("fxContent"); }

  function onSelChange() {
    if (window.App) { window.App.onSelectionChanged(state.sel); }
    syncStatus();
  }

  function syncStatus() {
    var left = document.getElementById("statusLeft");
    if (left) {
      var text = "共 " + state.entries.length + " 个项目";
      if (state.sel.length) {
        var total = state.sel.reduce(function (a, e) { return a + (e.size || 0); }, 0);
        text += " · 已选中 " + state.sel.length + " 个，共 " + UI.fmtSize(total);
      }
      left.textContent = text;
    }
    var right = document.getElementById("statusRight");
    if (right) { right.textContent = state.writable ? "可写" : "只读"; }
  }

  function sorted() {
    var list = state.entries.slice();
    var dir = state.desc ? -1 : 1;
    list.sort(function (a, b) {
      if (a.is_dir !== b.is_dir) { return a.is_dir ? -1 : 1; }
      var r = 0;
      switch (state.sort) {
        case "size": r = (a.size || 0) - (b.size || 0); break;
        case "mtime": r = (a.mtime || 0) - (b.mtime || 0); break;
        case "taken": r = (a.taken_at || a.mtime || 0) - (b.taken_at || b.mtime || 0); break;
        case "type": r = String(a.ext || "").localeCompare(String(b.ext || "")); break;
        case "rating": r = (a.rating || 0) - (b.rating || 0); break;
        default: r = String(a.name).localeCompare(String(b.name), "zh-Hans-CN"); break;
      }
      if (r === 0) { r = String(a.name).localeCompare(String(b.name), "zh-Hans-CN"); }
      return r * dir;
    });
    return list;
  }

  function isSelected(p) { return state.sel.some(function (e) { return e.path === p; }); }
  function leadIcon(it) { return I(it.is_dir ? "i-folder" : (it.kind === "image" ? "i-image" : "i-file")); }

  function renderCols() {
    var c = colsEl();
    c.innerHTML = "";
    if (state.view !== "details") { c.style.display = "none"; return; }
    c.style.display = "flex";
    var defs = [
      ["name", "名称", "fx-col fx-col-name"],
      ["mtime", "修改日期", "fx-col fx-col-modified"],
      ["type", "类型", "fx-col fx-col-type"],
      ["size", "大小", "fx-col fx-col-size"]
    ];
    defs.forEach(function (d) {
      var b = E("button", d[2]);
      b.appendChild(E("span", null, d[1]));
      if (state.sort === d[0]) {
        var s = E("span", "fx-sort" + (state.desc ? " fx-desc" : ""));
        s.appendChild(I("i-up"));
        b.appendChild(s);
      }
      b.onclick = function () {
        if (state.sort === d[0]) { state.desc = !state.desc; } else { state.sort = d[0]; state.desc = false; }
        var sel = document.getElementById("sortSelect");
        if (sel) { sel.value = state.sort; }
        render();
      };
      c.appendChild(b);
    });
    c.appendChild(E("div", "fx-col fx-col-acts"));
  }

  function row(it) {
    var cls = "fx-row" + (it.is_dir ? " fx-dir" : "") + (isSelected(it.path) ? " fx-sel" : "");
    var r = E("div", cls);
    var lead = E("span", "fx-cell fx-lead");
    lead.appendChild(leadIcon(it));
    r.appendChild(lead);
    if (state.view === "icons" || state.view === "tiles") {
      var th = E("div", "fx-thumb");
      if (it.kind === "image" && it.size >= 0) {
        var img = document.createElement("img");
        img.loading = "lazy";
        img.src = API.thumbURL(it.path, state.view === "icons" ? 256 : 128);
        img.alt = it.name;
        th.appendChild(img);
      } else {
        var big = leadIcon(it);
        big.setAttribute("width", "40");
        big.setAttribute("height", "40");
        th.appendChild(big);
      }
      r.appendChild(th);
    }
    var nm = E("span", "fx-cell fx-name");
    nm.appendChild(E("span", "fx-name-text", it.name));
    if (it.favorite) {
      var st = I("i-star");
      st.setAttribute("width", "12");
      st.setAttribute("height", "12");
      nm.appendChild(st);
    }
    r.appendChild(nm);
    r.appendChild(E("span", "fx-cell fx-modified", UI.fmtTime(it.mtime, false)));
    r.appendChild(E("span", "fx-cell fx-type", it.is_dir ? "文件夹" : (it.ext || "文件")));
    r.appendChild(E("span", "fx-cell fx-size", it.is_dir ? "" : UI.fmtSize(it.size)));
    var acts = E("span", "fx-cell fx-acts");
    function actBtn(icon, title, fn) {
      var b = E("button", "icon-btn");
      b.title = title;
      b.appendChild(I(icon));
      b.onclick = function (ev) { ev.stopPropagation(); fn(); };
      return b;
    }
    acts.appendChild(actBtn("i-eye", "查看", function () { open(it); }));
    acts.appendChild(actBtn("i-download", "下载", function () { window.location.href = API.fileURL(it.path, true); }));
    acts.appendChild(actBtn("i-share", "分享", function () { if (window.App.sharePaths) { window.App.sharePaths([it.path]); } }));
    if (state.writable) {
      acts.appendChild(actBtn("i-plus", "重命名", function () { if (window.App.renameEntry) { window.App.renameEntry(it); } }));
      acts.appendChild(actBtn("i-trash", "删除", function () { if (window.App.deletePaths) { window.App.deletePaths([it.path]); } }));
    }
    r.appendChild(acts);
    bindItem(r, it);
    return r;
  }

  function render() {
    var c = contentEl();
    c.className = "fx-content" +
      (state.view === "icons" ? " fx-grid" : "") +
      (state.view === "tiles" ? " fx-tiles" : "") +
      (state.view === "list" ? " fx-list" : "");
    renderCols();
    var rows = rowsEl();
    rows.innerHTML = "";
    if (state.loading) { rows.appendChild(E("div", "empty-big", "正在读取目录 ...")); return; }
    var list = sorted();
    if (!list.length) {
      var empty = E("div", "empty-big");
      empty.appendChild(I("i-folder"));
      empty.appendChild(E("div", null, "此文件夹为空"));
      rows.appendChild(empty);
      return;
    }
    list.forEach(function (it) { rows.appendChild(row(it)); });
    syncStatus();
    rows.oncontextmenu = function (ev) {
      if (ev.target === rows) { ev.preventDefault(); blankMenu(ev.clientX, ev.clientY); }
    };
    rows.onmousedown = function (ev) { if (ev.target === rows) { clearSel(); } };
  }

  function bindItem(node, it) {
    node.dataset.path = it.path;
    node.onclick = function (ev) { ev.stopPropagation(); select(it, ev.ctrlKey || ev.metaKey, ev.shiftKey); };
    node.ondblclick = function (ev) { ev.stopPropagation(); open(it); };
    node.oncontextmenu = function (ev) {
      ev.preventDefault();
      ev.stopPropagation();
      if (!isSelected(it.path)) { state.sel = [it]; render(); onSelChange(); }
      itemMenu(ev.clientX, ev.clientY, it);
    };
    node.draggable = true;
    node.ondragstart = function (ev) {
      if (!isSelected(it.path)) { state.sel = [it]; render(); onSelChange(); }
      ev.dataTransfer.setData("text/plain", state.sel.map(function (s) { return s.path; }).join(String.fromCharCode(10)));
      ev.dataTransfer.effectAllowed = "copyMove";
    };
    if (it.is_dir) {
      node.ondragover = function (ev) { ev.preventDefault(); node.classList.add("fx-sel"); };
      node.ondragleave = function () { if (!isSelected(it.path)) { node.classList.remove("fx-sel"); } };
      node.ondrop = function (ev) {
        ev.preventDefault();
        ev.stopPropagation();
        node.classList.remove("fx-sel");
        var paths = (ev.dataTransfer.getData("text/plain") || "").split(String.fromCharCode(10)).filter(Boolean);
        if (paths.length) { window.App.movePaths(paths, it.path); }
      };
    }
  }

  function select(it, additive, range) {
    if (range && state.anchor >= 0) {
      var list = sorted();
      var a = state.anchor, b = list.indexOf(it);
      if (b < 0) { b = list.length - 1; }
      state.sel = list.slice(Math.min(a, b), Math.max(a, b) + 1);
    } else if (additive) {
      if (isSelected(it.path)) { state.sel = state.sel.filter(function (s) { return s.path !== it.path; }); }
      else { state.sel = state.sel.concat([it]); state.anchor = sorted().indexOf(it); }
    } else {
      state.sel = [it];
      state.anchor = sorted().indexOf(it);
    }
    render();
    onSelChange();
  }

  function clearSel() { if (!state.sel.length) { return; } state.sel = []; render(); onSelChange(); }
  function selectAll() { state.sel = sorted().slice(); render(); onSelChange(); }

  function open(it) {
    if (it.is_dir) { window.App.navigateFiles(it.path); return; }
    if (it.kind === "image") {
      window.App.openLightbox(it.path, state.entries.filter(function (e) { return e.kind === "image"; }));
      return;
    }
    window.location.href = API.fileURL(it.path, true);
  }

  function itemMenu(x, y, it) {
    var items = [
      { label: "打开", icon: "i-folder", action: function () { open(it); } },
      { label: "下载", icon: "i-download", action: function () { window.location.href = API.fileURL(it.path, true); } },
      { label: "分享", icon: "i-share", action: function () { if (window.App.sharePaths) { window.App.sharePaths(state.sel.map(function (s) { return s.path; })); } } },
      { label: "以图搜图", icon: "i-search", disabled: it.kind !== "image", action: function () { window.App.similarByPath(it.path); } }
    ];
    if (state.writable) {
      items = items.concat([
        { sep: true },
        { label: "重命名", action: function () { if (window.App.renameEntry) { window.App.renameEntry(it); } } },
        { label: "复制", action: function () { clipboard = { mode: "copy", paths: state.sel.map(function (s) { return s.path; }) }; UI.toast("已复制 " + state.sel.length + " 项"); } },
        { label: "剪切", action: function () { clipboard = { mode: "cut", paths: state.sel.map(function (s) { return s.path; }) }; UI.toast("已剪切 " + state.sel.length + " 项"); } },
        { label: "删除", icon: "i-trash", danger: true, action: function () { if (window.App.deleteEntries) { window.App.deleteEntries(state.sel); } } }
      ]);
    }
    items.push({ sep: true });
    items.push({ label: "详细信息", action: function () { if (window.App.showDetails) { window.App.showDetails(it); } } });
    UI.menu(x, y, items);
  }

  function blankMenu(x, y) {
    var items = [];
    if (state.writable) {
      items.push({ label: "新建文件夹", icon: "i-plus", action: function () { window.App.newFolder(); } });
      items.push({ label: "上传文件", icon: "i-upload", action: function () { window.App.pickUpload(); } });
      items.push({ label: "上传文件夹", icon: "i-upload", action: function () { window.App.pickUploadDir(); } });
      items.push({ sep: true });
      items.push({ label: "粘贴", disabled: !clipboard.paths.length, action: function () { window.App.pasteTo(state.path); } });
      items.push({ sep: true });
    }
    items.push({ label: "全选", action: selectAll });
    items.push({ label: "刷新", icon: "i-refresh", action: function () { load(state.path); } });
    items.push({ label: "为当前目录建立索引", action: function () { window.App.indexNow(state.path); } });
    UI.menu(x, y, items);
  }

  async function load(path, opts) {
    opts = opts || {};
    state.loading = true;
    render();
    try {
      var d = await API.list(path, state.hidden);
      state.path = d.path;
      state.parent = d.parent || "";
      state.entries = d.entries || [];
      state.writable = !!d.writable;
      state.sel = [];
      state.anchor = -1;
      state.loading = false;
      if (!opts.keepView && window.App.setCrumbs) { window.App.setCrumbs(d.crumbs || [], d.path); }
      render();
      onSelChange();
      return d;
    } catch (e) {
      state.loading = false;
      render();
      UI.toast(e.message);
      throw e;
    }
  }

  async function refresh() { if (state.path) { await load(state.path, { keepView: true }); } }

  function setView(v) {
    state.view = v;
    document.querySelectorAll("#viewSwitch .icon-btn").forEach(function (b) {
      b.classList.toggle("active", b.dataset.view === v);
    });
    render();
  }

  function setSort(sort, desc) {
    state.sort = sort;
    if (desc !== undefined) { state.desc = desc; }
    render();
  }

  window.Explorer = {
    state: state, load: load, refresh: refresh, render: render,
    setView: setView, setSort: setSort, selectAll: selectAll, clearSel: clearSel, open: open,
    getSelection: function () { return state.sel; },
    setClipboard: function (mode, paths) { clipboard = { mode: mode, paths: paths }; },
    getClipboard: function () { return clipboard; },
    isSelected: isSelected
  };
})();