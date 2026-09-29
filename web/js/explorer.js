/* YCFMG 文件管理器视图：Windows 资源管理器式的浏览、选择、拖拽与右键操作 */
(function () {
  "use strict";

  var E = UI.el, I = UI.icon;

  var state = {
    path: "",
    parent: "",
    entries: [],
    sel: [],
    anchor: -1,
    view: "icons",
    sort: "name",
    desc: false,
    loading: false,
    writable: false,
    hidden: false
  };

  var clipboard = { mode: "", paths: [] };

  function container() { return document.getElementById("viewExplorer"); }

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

  function isSelected(path) {
    return state.sel.some(function (e) { return e.path === path; });
  }

  function render() {
    var c = container();
    c.innerHTML = "";
    if (state.loading) {
      var load = E("div", "empty-big");
      load.appendChild(E("div", null, "正在读取目录 ..."));
      c.appendChild(load);
      return;
    }
    var list = sorted();
    if (!list.length) {
      var empty = E("div", "empty-big");
      empty.appendChild(I("i-folder"));
      empty.appendChild(E("div", null, "此文件夹为空"));
      empty.appendChild(E("div", "muted", "可拖入文件上传，或右键新建文件夹"));
      c.appendChild(empty);
      return;
    }
    var wrap;
    if (state.view === "icons") { wrap = E("div", "files-grid"); }
    else if (state.view === "list") { wrap = E("div", "files-list"); }
    else { wrap = E("div", "files-list"); }
    if (state.view === "details") {
      wrap.appendChild(headRow());
    }
    list.forEach(function (it, idx) {
      var row;
      if (state.view === "icons") { row = tileIcon(it); }
      else if (state.view === "list") { row = rowLine(it, idx); }
      else { row = rowDetail(it, idx); }
      wrap.appendChild(row);
    });
    c.appendChild(wrap);
    c.onmousedown = function (ev) {
      if (ev.target === c || ev.target === wrap) { clearSel(); }
    };
    syncStatus();
  }

  function headRow() {
    var cols = [["", ""], ["name", "名称"], ["mtime", "修改日期"], ["type", "类型"], ["size", "大小"]];
    var r = E("div", "row head-row");
    cols.forEach(function (c) {
      var cell = E("div", null, c[1]);
      if (c[0]) {
        cell.style.cursor = "pointer";
        cell.onclick = function () {
          if (state.sort === c[0]) { state.desc = !state.desc; } else { state.sort = c[0]; state.desc = false; }
          render();
        };
        if (state.sort === c[0]) { cell.textContent = c[1] + (state.desc ? " ↓" : " ↑"); }
      }
      r.appendChild(cell);
    });
    return r;
  }

  function bindItem(node, it) {
    node.dataset.path = it.path;
    if (isSelected(it.path)) { node.classList.add("sel"); }
    node.onclick = function (ev) {
      ev.stopPropagation();
      select(it, ev.ctrlKey || ev.metaKey, ev.shiftKey);
    };
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
      ev.dataTransfer.setData("text/plain", state.sel.map(function (s) { return s.path; }).join("\n"));
      ev.dataTransfer.effectAllowed = "copyMove";
    };
    if (it.is_dir) {
      node.ondragover = function (ev) { ev.preventDefault(); node.classList.add("sel"); };
      node.ondragleave = function () { if (!isSelected(it.path)) { node.classList.remove("sel"); } };
      node.ondrop = function (ev) {
        ev.preventDefault();
        ev.stopPropagation();
        node.classList.remove("sel");
        var paths = (ev.dataTransfer.getData("text/plain") || "").split("\n").filter(Boolean);
        if (paths.length) { window.App.movePaths(paths, it.path); }
      };
    }
  }

  function tileIcon(it) {
    var t = E("div", "tile");
    var th = E("div", "thumb");
    if (it.kind === "image" && it.size >= 0) {
      var img = document.createElement("img");
      img.loading = "lazy";
      img.src = API.thumbURL(it.path, 256);
      img.alt = it.name;
      th.appendChild(img);
    } else if (it.is_dir) {
      th.appendChild(I("i-folder"));
    } else if (it.kind === "video") {
      th.appendChild(I("i-image"));
    } else {
      th.appendChild(I("i-file"));
    }
    t.appendChild(th);
    t.appendChild(E("div", "nm", it.name));
    bindItem(t, it);
    return t;
  }

  function rowLine(it, idx) {
    var r = E("div", "row");
    var ic = I(it.is_dir ? "i-folder" : (it.kind === "image" ? "i-image" : "i-file"), "ico");
    r.appendChild(ic);
    r.appendChild(E("div", "nm", it.name));
    r.appendChild(E("div", "muted", UI.fmtSize(it.size)));
    r.appendChild(E("div", "muted", UI.fmtTime(it.mtime)));
    r.appendChild(E("div", "muted", UI.kindLabel(it.kind)));
    bindItem(r, it);
    return r;
  }

  function rowDetail(it, idx) {
    var r = E("div", "row");
    var ic = I(it.is_dir ? "i-folder" : (it.kind === "image" ? "i-image" : "i-file"), "ico");
    r.appendChild(ic);
    r.appendChild(E("div", "nm", it.name));
    r.appendChild(E("div", "muted", UI.fmtTime(it.mtime)));
    r.appendChild(E("div", "muted", it.is_dir ? "文件夹" : (it.ext || "文件")));
    r.appendChild(E("div", "muted", it.is_dir ? "" : UI.fmtSize(it.size)));
    bindItem(r, it);
    return r;
  }

  function select(it, additive, range) {
    if (range && state.anchor >= 0) {
      var list = sorted();
      var a = state.anchor, b = list.indexOf(it);
      if (b < 0) { b = list.length - 1; }
      var lo = Math.min(a, b), hi = Math.max(a, b);
      state.sel = list.slice(lo, hi + 1);
    } else if (additive) {
      if (isSelected(it.path)) {
        state.sel = state.sel.filter(function (s) { return s.path !== it.path; });
      } else {
        state.sel = state.sel.concat([it]);
        state.anchor = sorted().indexOf(it);
      }
    } else {
      state.sel = [it];
      state.anchor = sorted().indexOf(it);
    }
    render();
    onSelChange();
  }

  function clearSel() {
    if (!state.sel.length) { return; }
    state.sel = [];
    render();
    onSelChange();
  }

  function selectAll() {
    state.sel = sorted().slice();
    render();
    onSelChange();
  }

  function open(it) {
    if (it.is_dir) { window.App.navigateFiles(it.path); return; }
    if (it.kind === "image") { window.App.openLightbox(it.path, state.entries.filter(function (e) { return e.kind === "image"; })); return; }
    window.location.href = API.fileURL(it.path, true);
  }

  function itemMenu(x, y, it) {
    var many = state.sel.length > 1;
    var items = [
      { label: "打开", icon: "i-folder", action: function () { open(it); } },
      { label: "在新标签页打开", icon: "i-eye", action: function () { window.open(API.fileURL(it.path, false), "_blank"); } },
      { sep: true },
      { label: "下载", icon: "i-download", action: function () { window.location.href = API.fileURL(it.path, true); } },
      { label: "分享", icon: "i-share", action: function () { window.App.sharePaths(state.sel.map(function (s) { return s.path; })); } },
      { label: "以图搜图", icon: "i-search", disabled: it.kind !== "image", action: function () { window.App.similarByPath(it.path); } }
    ];
    if (state.writable) {
      items = items.concat([
        { sep: true },
        { label: "重命名", action: function () { window.App.renameEntry(it); } },
        { label: "复制", action: function () { clipboard = { mode: "copy", paths: state.sel.map(function (s) { return s.path; }) }; UI.toast("已复制 " + state.sel.length + " 项"); } },
        { label: "剪切", action: function () { clipboard = { mode: "cut", paths: state.sel.map(function (s) { return s.path; }) }; UI.toast("已剪切 " + state.sel.length + " 项"); } },
        { label: "删除", icon: "i-trash", danger: true, action: function () { window.App.deleteEntries(state.sel); } }
      ]);
    }
    items.push({ sep: true });
    items.push({ label: "详细信息", action: function () { window.App.showDetails(it); } });
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
    items.push({ label: "在此处为图片建立索引", action: function () { window.App.indexNow(state.path); } });
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
      if (!opts.keepView) { window.App.setCrumbs(d.crumbs || [], d.path); }
      render();
      onSelChange();
      container().oncontextmenu = function (ev) {
        if (ev.target === container() || ev.target.classList.contains("files-grid") || ev.target.classList.contains("files-list") || ev.target.classList.contains("empty-big")) {
          ev.preventDefault();
          blankMenu(ev.clientX, ev.clientY);
        }
      };
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

  function setPath(p) { state.path = p; }

  window.Explorer = {
    state: state,
    load: load,
    refresh: refresh,
    render: render,
    setView: setView,
    setSort: setSort,
    setPath: setPath,
    selectAll: selectAll,
    clearSel: clearSel,
    open: open,
    getSelection: function () { return state.sel; },
    setClipboard: function (mode, paths) { clipboard = { mode: mode, paths: paths }; },
    getClipboard: function () { return clipboard; },
    isSelected: isSelected
  };
})();