/* YCFMG 主应用：登录、路由、导航、工具栏、文件操作、详情与灯箱 */
(function () {
  "use strict";
  var E = UI.el, I = UI.icon;

  var App = (window.App = window.App || {});
  App.user = null;
  App.pub = null;
  App.route = "";
  App.lightboxList = [];
  App.lightboxIndex = 0;
  App.uploading = false;

  function $(id) { return document.getElementById(id); }

  function show(view) {
    ["viewExplorer", "viewGallery", "viewSearch", "viewShares", "viewSettings", "viewTrash"].forEach(function (id) {
      var el = $(id);
      if (el) { el.classList.toggle("hidden", id !== view); }
    });
  }

  /* ---------- 启动与登录 ---------- */

  async function boot() {
    try { App.pub = await API.public(); } catch (e) { App.pub = { brand: "YCFMG", subtitle: "文件管理 · 智能图库" }; }
    API.setBase((App.pub && App.pub.base_path) || "");
    $("loginBrand").textContent = (App.pub && App.pub.brand) || "YCFMG";
    $("loginSub").textContent = (App.pub && App.pub.subtitle) || "";
    applyTheme(localStorage.getItem("ycfmg-theme") || "");
    $("loginForm").onsubmit = doLogin;
    try {
      var me = await API.me();
      App.user = me.username;
      enterApp();
    } catch (e) {
      $("loginView").classList.remove("hidden");
      $("app").classList.add("hidden");
    }
  }

  async function doLogin(ev) {
    ev.preventDefault();
    var u = $("loginUser").value.trim() || "admin";
    var p = $("loginPass").value;
    $("loginErr").textContent = "";
    try {
      var d = await API.login(u, p);
      App.user = d.username;
      $("loginPass").value = "";
      enterApp();
    } catch (e) { $("loginErr").textContent = e.message; }
  }

  function applyTheme(t) {
    if (t === "dark" || t === "light") { document.documentElement.setAttribute("data-theme", t); }
    else { document.documentElement.removeAttribute("data-theme"); }
    localStorage.setItem("ycfmg-theme", t || "");
  }

  function enterApp() {
    $("loginView").classList.add("hidden");
    $("app").classList.remove("hidden");
    $("userChip").textContent = App.user || "admin";
    bindUI();
    refreshNav();
    if (App.startPolling) { App.startPolling(); }
    route();
    window.addEventListener("hashchange", route);
  }

  App.onUnauthorized = function () {
    $("app").classList.add("hidden");
    $("loginView").classList.remove("hidden");
  };

  /* ---------- 路由 ---------- */

  function parseHash() {
    var h = location.hash.replace(/^#/, "");
    if (!h) { return { name: "gallery", params: {} }; }
    var qi = h.indexOf("?");
    var name = qi >= 0 ? h.slice(0, qi) : h;
    var params = {};
    if (qi >= 0) {
      h.slice(qi + 1).split("&").forEach(function (kv) {
        if (!kv) { return; }
        var i = kv.indexOf("=");
        if (i < 0) { params[decodeURIComponent(kv)] = ""; return; }
        params[decodeURIComponent(kv.slice(0, i))] = decodeURIComponent(kv.slice(i + 1));
      });
    }
    return { name: name.replace(/^\//, ""), params: params };
  }

  async function route() {
    var r = parseHash();
    App.route = r.name;
    document.querySelectorAll(".nav-item").forEach(function (n) { n.classList.remove("active"); });
    if (r.name === "files") {
      show("viewExplorer");
      var p = r.params.path || "";
      if (!p && App.pub && App.pub.libraries && App.pub.libraries.length) { p = App.pub.libraries[0].path; }
      if (!p) { renderDrives(); return; }
      try { await Explorer.load(p); } catch (e) { renderDrives(); }
      return;
    }
    if (r.name === "gallery") {
      show("viewGallery");
      Gallery.setQuery(r.params.q || "");
      if (r.params.classify) { Gallery.state.filter.classify = r.params.classify; }
      if (r.params.lib) { Gallery.state.filter.lib = r.params.lib; }
      if (r.params.color) { Gallery.state.filter.color = r.params.color; }
      if (r.params.tag) { Gallery.setQuery(r.params.tag); }
      await Gallery.reload();
      return;
    }
    if (r.name === "search") { show("viewSearch"); if (App.renderSearchHome) { App.renderSearchHome(); } return; }
    if (r.name === "shares") { show("viewShares"); if (App.renderShares) { App.renderShares(); } return; }
    if (r.name === "trash") { show("viewTrash"); if (App.renderTrash) { App.renderTrash(); } return; }
    if (r.name === "settings") { show("viewSettings"); if (App.renderSettings) { App.renderSettings(); } return; }
    show("viewGallery");
    await Gallery.reload();
  }

  /* ---------- 导航窗格 ---------- */

  function renderDrives() {
    show("viewExplorer");
    var c = $("viewExplorer");
    c.innerHTML = "";
    c.appendChild(E("div", "empty-big", "请选择左侧的文件库开始浏览"));
    setCrumbs([], "");
  }

  async function refreshNav() {
    try {
      var d = await API.facets("image");
      var box = $("navClasses");
      box.innerHTML = "";
      var classify = d.classify || [];
      if (!classify.length) { box.appendChild(E("div", "nav-empty", "尚未索引图片")); }
      classify.slice(0, 14).forEach(function (f) {
        var n = E("div", "nav-item");
        n.appendChild(I("i-image", "nic"));
        n.appendChild(E("span", null, f.key));
        n.appendChild(E("span", "cnt", String(f.count)));
        n.onclick = function () {
          document.querySelectorAll(".nav-item").forEach(function (x) { x.classList.remove("active"); });
          n.classList.add("active");
          location.hash = "#/gallery?classify=" + encodeURIComponent(f.key);
        };
        box.appendChild(n);
      });
    } catch (e) { }
    var libs = (App.pub && App.pub.libraries) || [];
    var lb = $("navLibs");
    lb.innerHTML = "";
    libs.forEach(function (l) {
      var n = E("div", "nav-item");
      n.appendChild(I("i-folder", "nic"));
      n.appendChild(E("span", null, l.name || l.path));
      n.onclick = function () { navigateFiles(l.path); };
      lb.appendChild(n);
    });
    if (!libs.length) { lb.appendChild(E("div", "nav-empty", "未配置文件库")); }
    try {
      var t = await API.tags();
      var tb = $("navTags");
      tb.innerHTML = "";
      var tags = (t.tags || []).slice(0, 20);
      tags.forEach(function (tag) {
        var n = E("div", "nav-item");
        var dot = E("span", "dot");
        dot.style.background = tag.color || "#0a63c9";
        n.appendChild(dot);
        n.appendChild(E("span", null, tag.name));
        n.appendChild(E("span", "cnt", String(tag.count || 0)));
        n.onclick = function () { location.hash = "#/gallery?tag=" + encodeURIComponent(tag.name); };
        tb.appendChild(n);
      });
      if (!tags.length) { tb.appendChild(E("div", "nav-empty", "暂无标签")); }
    } catch (e) { }
  }
  App.refreshNav = refreshNav;

  document.addEventListener("DOMContentLoaded", function () {
    document.querySelectorAll(".nav-item[data-route]").forEach(function (n) {
      n.onclick = function () { location.hash = n.dataset.route; };
    });
  });

  function setCrumbs(crumbs, path) {
    var box = $("crumbs");
    var input = $("addrInput");
    box.innerHTML = "";
    if (input) { input.value = path || ""; }
    if (!crumbs.length) { box.appendChild(E("span", "c", "此设备")); return; }
    var first = E("span", "c", "此设备");
    first.onclick = function () {
      var libs = (App.pub && App.pub.libraries) || [];
      navigateFiles(libs.length ? libs[0].path : "");
    };
    box.appendChild(first);
    crumbs.forEach(function (c, idx) {
      box.appendChild(E("span", "sep", "›"));
      var n = E("span", "c", c.name);
      n.title = c.path;
      n.onclick = function () { navigateFiles(c.path); };
      if (idx === crumbs.length - 1) { n.style.fontWeight = "600"; }
      box.appendChild(n);
    });
  }
  App.setCrumbs = setCrumbs;

  function navigateFiles(p) { location.hash = "#/files?path=" + encodeURIComponent(p || ""); }
  App.navigateFiles = navigateFiles;

  /* ---------- 工具栏与快捷键 ---------- */

  function bindUI() {
    $("tbBack").onclick = function () { history.back(); };
    $("tbFwd").onclick = function () { history.forward(); };
    $("tbUp").onclick = function () { if (Explorer.state.parent) { navigateFiles(Explorer.state.parent); } };
    $("tbRefresh").onclick = function () {
      if (App.route === "gallery") { Gallery.reload(); } else { Explorer.refresh(); }
      refreshNav();
    };
    $("tbNew").onclick = newFolder;
    $("tbUpload").onclick = pickUpload;
    $("tbDownload").onclick = function () {
      var sel = Explorer.getSelection();
      if (!sel.length) { UI.toast("请先选择文件"); return; }
      sel.forEach(function (s, i) {
        setTimeout(function () { window.location.href = API.fileURL(s.path, true); }, i * 400);
      });
    };
    $("tbShare").onclick = function () {
      var sel = Explorer.getSelection();
      if (!sel.length) { UI.toast("请先选择要分享的内容"); return; }
      if (App.sharePaths) { App.sharePaths(sel.map(function (s) { return s.path; })); }
    };
    $("tbDelete").onclick = function () { deleteEntries(Explorer.getSelection()); };
    $("btnIndex").onclick = function () { indexNow(Explorer.state.path || ""); };
    $("btnTheme").onclick = function () {
      var cur = document.documentElement.getAttribute("data-theme") || "light";
      applyTheme(cur === "dark" ? "light" : "dark");
    };
    $("btnSettings").onclick = function () { location.hash = "#/settings"; };
    $("userChip").onclick = function (ev) {
      UI.menu(ev.clientX, ev.clientY, [
        { label: "个人设置", action: function () { location.hash = "#/settings"; } },
        { label: "修改密码", action: function () { if (App.changePassword) { App.changePassword(); } } },
        { label: "退出登录", danger: true, action: async function () { await API.logout(); location.reload(); } }
      ]);
    };
    document.querySelectorAll("#viewSwitch .icon-btn").forEach(function (b) {
      b.onclick = function () { Explorer.setView(b.dataset.view); };
    });
    $("sortSelect").onchange = function () {
      if (App.route === "gallery") { Gallery.state.sort = this.value; Gallery.reload(); }
      else { Explorer.setSort(this.value); }
    };
    $("sortDir").onclick = function () {
      if (App.route === "gallery") { Gallery.state.desc = !Gallery.state.desc; Gallery.reload(); }
      else { Explorer.setSort(Explorer.state.sort, !Explorer.state.desc); }
    };
    $("crumbsToggle").onclick = function () {
      var inp = $("addrInput");
      inp.classList.toggle("hidden");
      if (!inp.classList.contains("hidden")) { inp.focus(); inp.select(); }
    };
    $("addrInput").onkeydown = function (ev) {
      if (ev.key === "Enter") { navigateFiles(this.value.trim()); this.classList.add("hidden"); }
      if (ev.key === "Escape") { this.classList.add("hidden"); }
    };
    var search = $("globalSearch");
    search.onkeydown = function (ev) {
      if (ev.key === "Enter") {
        var q = this.value.trim();
        if (!q) { return; }
        location.hash = "#/gallery";
        Gallery.byText(q);
      }
    };
    search.ondragover = function (ev) { ev.preventDefault(); };
    search.ondrop = function (ev) {
      ev.preventDefault();
      var f = ev.dataTransfer.files && ev.dataTransfer.files[0];
      if (f) { location.hash = "#/gallery"; Gallery.byImage(f); }
    };
    $("filePicker").onchange = function () { uploadFiles(this.files); this.value = ""; };
    $("dirPicker").onchange = function () { uploadFiles(this.files); this.value = ""; };
    $("imagePicker").onchange = function () {
      if (this.files[0]) { location.hash = "#/gallery"; Gallery.byImage(this.files[0]); }
      this.value = "";
    };
    $("lbClose").onclick = closeLightbox;
    $("lbPrev").onclick = function () { stepLightbox(-1); };
    $("lbNext").onclick = function () { stepLightbox(1); };
    $("lbInfo").onclick = function () { showDetails(App.lightboxList[App.lightboxIndex]); };
    $("lightbox").onclick = function (ev) {
      if (ev.target.id === "lightbox" || ev.target.id === "lbBody") { closeLightbox(); }
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("dragover", function (ev) { ev.preventDefault(); });
    document.addEventListener("drop", onGlobalDrop);
  }

  function onKey(ev) {
    var tag = (ev.target.tagName || "").toLowerCase();
    if (tag === "input" || tag === "textarea" || tag === "select") { return; }
    if (ev.key === "Escape") { closeLightbox(); UI.closeMenu(); return; }
    if (!$("lightbox").classList.contains("hidden")) {
      if (ev.key === "ArrowLeft") { stepLightbox(-1); }
      if (ev.key === "ArrowRight") { stepLightbox(1); }
      return;
    }
    if (App.route !== "files") { return; }
    var ctrl = ev.ctrlKey || ev.metaKey;
    var k = (ev.key || "").toLowerCase();
    if (ctrl && k === "a") { ev.preventDefault(); Explorer.selectAll(); return; }
    if (ctrl && k === "c") { copySel("copy"); return; }
    if (ctrl && k === "x") { copySel("cut"); return; }
    if (ctrl && k === "v") { pasteTo(Explorer.state.path); return; }
    if (ev.key === "Delete") { deleteEntries(Explorer.getSelection()); return; }
    if (ev.key === "F2") {
      var sel = Explorer.getSelection();
      if (sel.length === 1) { renameEntry(sel[0]); }
      return;
    }
    if (ev.key === "Enter") {
      var s2 = Explorer.getSelection();
      if (s2.length === 1) { Explorer.open(s2[0]); }
    }
  }

  function copySel(mode) {
    var sel = Explorer.getSelection();
    if (!sel.length) { return; }
    Explorer.setClipboard(mode, sel.map(function (s) { return s.path; }));
    UI.toast((mode === "copy" ? "已复制 " : "已剪切 ") + sel.length + " 项");
  }

  async function onGlobalDrop(ev) {
    ev.preventDefault();
    var files = ev.dataTransfer && ev.dataTransfer.files;
    if (!files || !files.length) { return; }
    if (App.route !== "files" || !Explorer.state.writable) { return; }
    await uploadFiles(files);
  }

  /* ---------- 文件操作 ---------- */

  App.onSelectionChanged = function (sel) {
    if (sel && sel.length === 1) { showDetails(sel[0]); }
    else if (!sel || !sel.length) { $("detailsBody").innerHTML = "<div class=\"empty-tip\">选择文件或图片查看详情</div>"; }
  };

  async function newFolder() {
    if (!Explorer.state.writable) { UI.toast("当前位置为只读"); return; }
    var name = await UI.prompt("新建文件夹", "文件夹名称", "新建文件夹");
    if (!name) { return; }
    try {
      await API.mkdir(Explorer.state.path, name);
      UI.toast("已创建");
      Explorer.refresh();
    } catch (e) { UI.toast(e.message); }
  }

  async function renameEntry(it) {
    var name = await UI.prompt("重命名", "新名称", it.name);
    if (!name || name === it.name) { return; }
    try {
      await API.rename(it.path, name);
      UI.toast("已重命名");
      Explorer.refresh();
    } catch (e) { UI.toast(e.message); }
  }
  App.renameEntry = renameEntry;

  async function deleteEntries(sel) {
    if (!sel || !sel.length) { UI.toast("请先选择项目"); return; }
    await deletePaths(sel.map(function (s) { return s.path; }));
  }
  App.deleteEntries = deleteEntries;

  async function deletePaths(paths) {
    var ok = await UI.confirm("删除 " + paths.length + " 个项目", "项目将移入回收站，可随时恢复。", "移入回收站");
    if (!ok) { return; }
    try {
      await API.remove(paths, false);
      UI.toast("已移入回收站");
      if (App.route === "gallery") { Gallery.reload(); } else { Explorer.refresh(); }
    } catch (e) { UI.toast(e.message); }
  }
  App.deletePaths = deletePaths;

  async function pasteTo(dst) {
    var cb = Explorer.getClipboard();
    if (!cb.paths.length) { return; }
    try {
      if (cb.mode === "cut") {
        await API.move(cb.paths, dst);
        Explorer.setClipboard("", []);
        UI.toast("已移动");
      } else {
        await API.copy(cb.paths, dst);
        UI.toast("已复制");
      }
      Explorer.refresh();
    } catch (e) { UI.toast(e.message); }
  }
  App.pasteTo = pasteTo;

  async function movePaths(paths, dst) {
    try {
      await API.move(paths, dst);
      UI.toast("已移动 " + paths.length + " 项");
      Explorer.refresh();
    } catch (e) { UI.toast(e.message); }
  }
  App.movePaths = movePaths;

  function pickUpload() { $("filePicker").click(); }
  function pickUploadDir() { $("dirPicker").click(); }
  App.pickUpload = pickUpload;
  App.pickUploadDir = pickUploadDir;

  async function uploadFiles(files) {
    if (!files || !files.length) { return; }
    if (App.uploading) { UI.toast("已有上传任务进行中"); return; }
    App.uploading = true;
    var dir = Explorer.state.path;
    var done = 0;
    for (var i = 0; i < files.length; i++) {
      var f = files[i];
      var targetDir = dir;
      if (f.webkitRelativePath) {
        var rel = f.webkitRelativePath.split("/");
        rel.pop();
        if (rel.length) {
          var sub = rel[rel.length - 1];
          try { await API.mkdir(dir, sub); } catch (e) { }
          targetDir = dir + "/" + rel.join("/");
        }
      }
      try {
        await API.upload(targetDir, f, function (p) {
          $("statusLeft").textContent = "上传 " + f.name + " " + Math.round(p * 100) + "%";
        });
        done++;
      } catch (e) { UI.toast("上传失败: " + f.name + " " + e.message); }
    }
    App.uploading = false;
    UI.toast("上传完成 " + done + "/" + files.length);
    Explorer.refresh();
    indexNow(dir);
  }

  async function indexNow(path) {
    try {
      await API.indexScan({ path: path || "", force: false });
      UI.toast("已开始索引");
    } catch (e) { UI.toast(e.message); }
  }
  App.indexNow = indexNow;

  /* ---------- 详情面板 ---------- */

  function kv(box, k, v) {
    if (v === undefined || v === null || v === "" || v === 0) { return; }
    var row = E("div", "kv");
    row.appendChild(E("b", null, k));
    row.appendChild(E("span", null, String(v)));
    box.appendChild(row);
  }

  function showDetails(it) {
    var box = $("detailsBody");
    box.innerHTML = "";
    if (!it) { box.innerHTML = "<div class=\"empty-tip\">选择文件或图片查看详情</div>"; return; }
    if (it.kind === "image") {
      var img = document.createElement("img");
      img.className = "det-preview";
      img.src = API.thumbURL(it.path, 512);
      box.appendChild(img);
    }
    kv(box, "名称", it.name || UI.baseName(it.path));
    kv(box, "类型", UI.kindLabel(it.kind) + (it.ext ? " " + it.ext : ""));
    kv(box, "大小", it.size >= 0 ? UI.fmtSize(it.size) : "未知");
    kv(box, "尺寸", it.width ? it.width + " × " + it.height : "");
    kv(box, "拍摄时间", it.taken_at ? UI.fmtTime(it.taken_at) : "");
    kv(box, "修改时间", UI.fmtTime(it.mtime));
    kv(box, "分类", it.classify);
    kv(box, "主色", it.color);
    kv(box, "相机", it.camera);
    kv(box, "镜头", it.lens);
    kv(box, "ISO", it.iso);
    kv(box, "光圈", it.fnum);
    kv(box, "快门", it.exposure);
    kv(box, "焦段", it.focal);
    if (it.gps_lat) { kv(box, "位置", it.gps_lat.toFixed(5) + ", " + it.gps_lon.toFixed(5)); }
    kv(box, "路径", it.path);
    if (it.tags && it.tags.length) { kv(box, "标签", it.tags.join("、")); }
    var ops = E("div", "field");
    var row = E("div", "tb-group");
    var star = E("button", "chip-btn", it.favorite ? "取消收藏" : "收藏");
    star.onclick = function () { toggleFavorite(it); };
    row.appendChild(star);
    var shr = E("button", "chip-btn", "分享");
    shr.onclick = function () { if (App.sharePaths) { App.sharePaths([it.path]); } };
    row.appendChild(shr);
    if (it.kind === "image") {
      var sim = E("button", "chip-btn", "以图搜图");
      sim.onclick = function () { similarByPath(it.path, it.id); };
      row.appendChild(sim);
    }
    ops.appendChild(row);
    box.appendChild(ops);
  }
  App.showDetails = showDetails;

  async function toggleFavorite(it) {
    try {
      await API.fileMeta({ id: it.id, path: it.path, favorite: !it.favorite });
      it.favorite = !it.favorite;
      UI.toast(it.favorite ? "已收藏" : "已取消收藏");
    } catch (e) { UI.toast(e.message); }
  }
  App.toggleFavorite = toggleFavorite;

  /* ---------- 以图搜图 ---------- */

  async function similarByPath(path, id) {
    var wantId = id || 0;
    if (!wantId && window.Explorer) {
      var hit = Explorer.state.entries.filter(function (x) { return x.path === path; })[0];
      if (hit && hit.id) { wantId = hit.id; }
    }
    location.hash = "#/gallery";
    if (wantId) { Gallery.bySimilarId(wantId); return; }
    UI.toast("正在以该图片检索相似内容 ...");
    try {
      var resp = await fetch(API.fileURL(path, false), { credentials: "same-origin" });
      var blob = await resp.blob();
      var file = new File([blob], UI.baseName(path), { type: blob.type || "image/jpeg" });
      Gallery.byImage(file);
    } catch (e) { UI.toast(e.message); }
  }
  App.similarByPath = similarByPath;

  /* ---------- 灯箱 ---------- */

  function openLightbox(path, list) {
    var imgs = (list || []).filter(function (x) { return x && x.path; });
    if (!imgs.length) { imgs = [{ path: path, name: UI.baseName(path) }]; }
    App.lightboxList = imgs;
    var idx = 0;
    imgs.forEach(function (x, i) { if (x.path === path) { idx = i; } });
    App.lightboxIndex = idx;
    renderLightbox();
    $("lightbox").classList.remove("hidden");
  }
  App.openLightbox = openLightbox;

  function renderLightbox() {
    var it = App.lightboxList[App.lightboxIndex];
    if (!it) { return; }
    $("lbImg").src = API.fileURL(it.path, false);
    $("lbTitle").textContent = (it.name || UI.baseName(it.path)) + "  (" + (App.lightboxIndex + 1) + "/" + App.lightboxList.length + ")";
    $("lbDownload").href = API.fileURL(it.path, true);
  }

  function stepLightbox(d) {
    if (!App.lightboxList.length) { return; }
    App.lightboxIndex = (App.lightboxIndex + d + App.lightboxList.length) % App.lightboxList.length;
    renderLightbox();
  }

  function closeLightbox() {
    $("lightbox").classList.add("hidden");
    $("lbImg").src = "";
  }

  window.App = App;
  document.addEventListener("DOMContentLoaded", boot);
  if (document.readyState !== "loading") { boot(); }
})();