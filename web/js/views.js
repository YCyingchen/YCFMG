/* YCFMG 次级视图：分享管理、回收站、搜索中心、设置与状态轮询 */
(function () {
  "use strict";
  var App = (window.App = window.App || {});
  var E = UI.el, I = UI.icon;

  function $(id) { return document.getElementById(id); }

  /* ---------- 分享 ---------- */

  async function sharePaths(paths) {
    if (!paths || !paths.length) { return; }
    var form = await UI.form("创建分享", [
      { name: "title", label: "分享标题（留空自动生成）", value: paths.length === 1 ? UI.baseName(paths[0]) : "" },
      { name: "descr", label: "描述", type: "textarea" },
      { name: "password", label: "提取密码（留空表示公开）" },
      { name: "expire_days", label: "有效期（天，0 表示长期有效）", value: "7" },
      { name: "max_download", label: "下载次数上限（0 表示不限）", value: "0" },
      { name: "allow_download", label: "允许下载", type: "select", value: "1", options: [{ value: "1", label: "允许" }, { value: "0", label: "仅在线预览" }] }
    ], "生成分享链接");
    if (!form) { return; }
    try {
      var d = await API.shareCreate({
        title: form.title,
        descr: form.descr,
        paths: paths,
        password: form.password,
        expire_days: parseInt(form.expire_days || "0", 10) || 0,
        max_download: parseInt(form.max_download || "0", 10) || 0,
        allow_download: form.allow_download === "1"
      });
      showShareResult(d);
      if (App.refreshNav) { App.refreshNav(); }
    } catch (e) { UI.toast(e.message); }
  }
  App.sharePaths = sharePaths;

  function showShareResult(sh) {
    var box = $("modalBox");
    var wrap = $("modal");
    box.innerHTML = "";
    box.appendChild(E("h2", null, "分享已创建"));
    box.appendChild(E("p", "muted", "以下域名均已绑定，点击复制即可直接使用："));
    var links = sh.links || [];
    links.forEach(function (l) {
      var row = E("div", "link-row");
      var a = E("a", "u", l.url);
      a.href = l.url;
      a.target = "_blank";
      row.appendChild(a);
      if (l.current) { row.appendChild(E("span", "tag", "当前域名")); }
      if (l.primary) { row.appendChild(E("span", "tag", "首选")); }
      var cp = E("button", "btn", "复制");
      cp.onclick = function () {
        if (navigator.clipboard) { navigator.clipboard.writeText(l.url); }
        UI.toast("已复制链接");
      };
      row.appendChild(cp);
      box.appendChild(row);
    });
    if (sh.has_password) {
      box.appendChild(E("p", "muted", "该分享已设置提取密码，请一并告知对方。"));
    }
    var actions = E("div", "actions");
    var openBtn = E("button", "btn", "打开分享页");
    openBtn.onclick = function () { window.open(sh.url, "_blank"); };
    var goList = E("button", "btn primary", "查看全部分享");
    goList.onclick = function () {
      wrap.classList.add("hidden");
      location.hash = "#/shares";
    };
    actions.appendChild(openBtn);
    actions.appendChild(goList);
    box.appendChild(actions);
    wrap.classList.remove("hidden");
  }

  async function renderShares() {
    var c = $("viewShares");
    c.innerHTML = "";
    var head = E("div", "gal-head");
    head.appendChild(E("h3", null, "分享管理"));
    var fill = E("div", "fill");
    fill.style.flex = "1";
    head.appendChild(fill);
    var shareSel = E("button", "chip-btn", "分享当前选中内容");
    shareSel.onclick = function () {
      var sel = Explorer.getSelection();
      if (!sel.length) { UI.toast("请先选择要分享的内容"); return; }
      sharePaths(sel.map(function (s) { return s.path; }));
    };
    head.appendChild(shareSel);
    c.appendChild(head);
    try {
      var d = await API.shares({ limit: 100 });
      var ov = d.overview || {};
      var stats = E("div", "stat-grid");
      var pairs = [["分享总数", ov.total], ["有效分享", ov.active], ["浏览次数", ov.views], ["下载次数", ov.downloads]];
      pairs.forEach(function (p) {
        var s = E("div", "stat");
        s.appendChild(E("div", "k", p[0]));
        s.appendChild(E("div", "v", String(p[1] === undefined ? 0 : p[1])));
        stats.appendChild(s);
      });
      c.appendChild(stats);
      var spacer = E("div");
      spacer.style.height = "14px";
      c.appendChild(spacer);
      var items = d.items || [];
      items.forEach(function (sh) { c.appendChild(shareCard(sh)); });
      if (!items.length) { c.appendChild(E("div", "empty-big", "还没有创建过分享")); }
    } catch (e) { UI.toast(e.message); }
  }
  App.renderShares = renderShares;

  function shareCard(sh) {
    var card = E("div", "share-card");
    var first = (sh.items || [])[0];
    if (first && first.kind === "image") {
      var img = document.createElement("img");
      img.className = "cover";
      img.src = API.thumbURL(first.path, 256);
      card.appendChild(img);
    } else {
      var cover = E("div", "cover");
      cover.appendChild(I("i-share"));
      card.appendChild(cover);
    }
    var info = E("div", "info");
    info.appendChild(E("h4", null, sh.title || "未命名分享"));
    var sub = sh.count + " 个条目 · " + UI.fmtSize(sh.size) + " · 浏览 " + sh.view_count + " · 下载 " + sh.download_count;
    if (sh.expire_at) { sub += " · 到期 " + UI.fmtTime(sh.expire_at, false); } else { sub += " · 长期有效"; }
    if (sh.has_password) { sub += " · 已加密"; }
    if (sh.disabled) { sub += " · 已停用"; }
    info.appendChild(E("p", null, sub));
    var link = (sh.links && sh.links[0]) ? sh.links[0].url : (sh.url || "");
    info.appendChild(E("p", "muted", link));
    card.appendChild(info);
    var ops = E("div", "ops");
    var copy = E("button", "chip-btn", "复制链接");
    copy.onclick = function () {
      if (navigator.clipboard) { navigator.clipboard.writeText(link); }
      UI.toast("已复制");
    };
    ops.appendChild(copy);
    var linkBtn = E("button", "chip-btn", "多域名 / 访问记录");
    linkBtn.onclick = function () { showShareLinks(sh); };
    ops.appendChild(linkBtn);
    var toggle = E("button", "chip-btn", sh.disabled ? "启用" : "停用");
    toggle.onclick = async function () {
      try { await API.shareUpdate(sh.id, { disabled: !sh.disabled }); renderShares(); } catch (e) { UI.toast(e.message); }
    };
    ops.appendChild(toggle);
    var del = E("button", "chip-btn", "删除");
    del.onclick = async function () {
      var yes = await UI.confirm("删除分享", "删除后分享链接立即失效。", "删除");
      if (!yes) { return; }
      try { await API.shareDelete(sh.id); UI.toast("已删除"); renderShares(); } catch (e) { UI.toast(e.message); }
    };
    ops.appendChild(del);
    card.appendChild(ops);
    return card;
  }

  async function showShareLinks(sh) {
    try {
      var d = await API.shareLinks(sh.id);
      var box = $("modalBox");
      var wrap = $("modal");
      box.innerHTML = "";
      box.appendChild(E("h2", null, "全部链接"));
      (d.links || []).forEach(function (l) {
        var row = E("div", "link-row");
        row.appendChild(E("span", "u", l.url));
        if (l.current) { row.appendChild(E("span", "tag", "当前")); }
        if (l.primary) { row.appendChild(E("span", "tag", "首选")); }
        box.appendChild(row);
      });
      box.appendChild(E("h2", null, "最近访问"));
      var visits = (d.visits || []).slice(0, 20);
      if (!visits.length) { box.appendChild(E("p", "muted", "暂无访问记录")); }
      visits.forEach(function (v) {
        var row = E("div", "link-row");
        row.appendChild(E("span", "u", v.ip + "  " + (v.action || "view")));
        row.appendChild(E("span", "tag", UI.fmtTime(v.at)));
        box.appendChild(row);
      });
      var actions = E("div", "actions");
      var close = E("button", "btn primary", "关闭");
      close.onclick = function () { wrap.classList.add("hidden"); };
      actions.appendChild(close);
      box.appendChild(actions);
      wrap.classList.remove("hidden");
    } catch (e) { UI.toast(e.message); }
  }

  /* ---------- 回收站 ---------- */

  async function renderTrash() {
    var c = $("viewTrash");
    c.innerHTML = "";
    var head = E("div", "gal-head");
    head.appendChild(E("h3", null, "回收站"));
    var fill = E("div", "fill");
    fill.style.flex = "1";
    head.appendChild(fill);
    var purge = E("button", "chip-btn", "清空回收站");
    purge.onclick = async function () {
      var yes = await UI.confirm("清空回收站", "其中的文件将被永久删除，无法恢复。", "清空");
      if (!yes) { return; }
      try { await API.trashPurge([]); UI.toast("已清空"); renderTrash(); } catch (e) { UI.toast(e.message); }
    };
    head.appendChild(purge);
    c.appendChild(head);
    try {
      var d = await API.trashList();
      var items = d.items || [];
      if (!items.length) { c.appendChild(E("div", "empty-big", "回收站是空的")); return; }
      var list = E("div", "files-list");
      items.forEach(function (it) {
        var row = E("div", "row");
        row.appendChild(I("i-trash", "ico"));
        row.appendChild(E("div", "nm", it.name));
        row.appendChild(E("div", "muted", UI.fmtSize(it.size)));
        row.appendChild(E("div", "muted", UI.fmtTime(it.deleted)));
        var act = E("div", "tb-group");
        var restore = E("button", "chip-btn", "恢复");
        restore.onclick = async function () {
          try { await API.trashRestore(it.id); UI.toast("已恢复"); renderTrash(); } catch (e) { UI.toast(e.message); }
        };
        var del = E("button", "chip-btn", "彻底删除");
        del.onclick = async function () {
          var yes = await UI.confirm("彻底删除", it.name + " 将被永久删除。", "删除");
          if (!yes) { return; }
          try { await API.trashPurge([it.id]); UI.toast("已删除"); renderTrash(); } catch (e) { UI.toast(e.message); }
        };
        act.appendChild(restore);
        act.appendChild(del);
        row.appendChild(act);
        list.appendChild(row);
      });
      c.appendChild(list);
    } catch (e) { UI.toast(e.message); }
  }
  App.renderTrash = renderTrash;

  /* ---------- 搜索中心 ---------- */

  async function renderSearchHome() {
    var c = $("viewSearch");
    c.innerHTML = "";
    var head = E("div", "gal-head");
    head.appendChild(E("h3", null, "搜索中心"));
    c.appendChild(head);
    var panel = E("div", "panel");
    panel.appendChild(E("h3", null, "以文搜图"));
    panel.appendChild(E("p", "muted", "支持中文与英文关键词，覆盖文件名、目录、标签、分类、相机与色调。"));
    var row = E("div", "tb-group");
    var input = E("input");
    input.placeholder = "例如：海边日落、猫、截图、美食、婚礼、iPhone";
    input.style.cssText = "flex:1;padding:10px 12px;border:1px solid var(--line2);border-radius:8px;background:transparent;color:inherit;outline:none";
    var go = E("button", "btn primary", "搜索");
    go.onclick = function () {
      if (!input.value.trim()) { return; }
      location.hash = "#/gallery";
      Gallery.byText(input.value.trim());
    };
    input.onkeydown = function (ev) { if (ev.key === "Enter") { go.onclick(); } };
    row.appendChild(input);
    row.appendChild(go);
    panel.appendChild(row);
    c.appendChild(panel);

    var panel2 = E("div", "panel");
    panel2.style.marginTop = "14px";
    panel2.appendChild(E("h3", null, "以图搜图"));
    panel2.appendChild(E("p", "muted", "上传或拖入一张图片，系统会用感知哈希与颜色直方图在全库中找出相似图片与重复图。"));
    var pick = E("button", "btn primary", "选择图片");
    pick.onclick = function () { $("imagePicker").click(); };
    panel2.appendChild(pick);
    c.appendChild(panel2);

    var panel3 = E("div", "panel");
    panel3.style.marginTop = "14px";
    panel3.appendChild(E("h3", null, "快捷入口"));
    var row3 = E("div", "tb-group");
    var b1 = E("button", "chip-btn", "查看全部图库");
    b1.onclick = function () { location.hash = "#/gallery"; };
    var b2 = E("button", "chip-btn", "查找重复图片");
    b2.onclick = function () { showDuplicates(); };
    row3.appendChild(b1);
    row3.appendChild(b2);
    panel3.appendChild(row3);
    c.appendChild(panel3);

    try {
      var s = await API.suggest();
      var panel4 = E("div", "panel");
      panel4.style.marginTop = "14px";
      panel4.appendChild(E("h3", null, "热门关键词"));
      var wrap = E("div", "gal-head");
      var words = (s.keywords || []).concat(s.tags || []).slice(0, 40);
      words.forEach(function (k) {
        var chip = E("div", "chip", k);
        chip.onclick = function () {
          location.hash = "#/gallery";
          Gallery.byText(k);
        };
        wrap.appendChild(chip);
      });
      panel4.appendChild(wrap);
      c.appendChild(panel4);
    } catch (e) { }
  }
  App.renderSearchHome = renderSearchHome;

  async function showDuplicates() {
    location.hash = "#/gallery";
    var c = $("viewGallery");
    c.innerHTML = "";
    c.appendChild(E("div", "empty-big", "正在扫描重复图片 ..."));
    try {
      var d = await API.duplicates("", 0);
      c.innerHTML = "";
      var head = E("div", "gal-head");
      head.appendChild(E("h3", null, "重复图片 · " + d.total + " 组"));
      c.appendChild(head);
      if (!d.total) { c.appendChild(E("div", "empty-big", "没有发现重复图片")); return; }
      (d.groups || []).forEach(function (g, gi) {
        var block = E("div", "gal-day");
        block.appendChild(E("h3", null, "第 " + (gi + 1) + " 组 · " + g.count + " 张 · 占用 " + g.size_text));
        var wf = E("div", "waterfall");
        g.items.forEach(function (it) {
          var cell = E("div", "cell");
          var img = document.createElement("img");
          img.loading = "lazy";
          img.src = API.thumbURL(it.path, 512);
          cell.appendChild(img);
          cell.appendChild(E("div", "cap", it.name));
          cell.onclick = function () { App.openLightbox(it.path, g.items); };
          cell.oncontextmenu = function (ev) {
            ev.preventDefault();
            UI.menu(ev.clientX, ev.clientY, [
              { label: "查看大图", icon: "i-eye", action: function () { App.openLightbox(it.path, g.items); } },
              { label: "删除这一张", icon: "i-trash", danger: true, action: function () { App.deletePaths([it.path]); } }
            ]);
          };
          wf.appendChild(cell);
        });
        block.appendChild(wf);
        c.appendChild(block);
      });
    } catch (e) { UI.toast(e.message); }
  }
  App.showDuplicates = showDuplicates;

  /* ---------- 设置 ---------- */

  async function renderSettings() {
    var c = $("viewSettings");
    c.innerHTML = "";
    var head = E("div", "gal-head");
    head.appendChild(E("h3", null, "设置"));
    c.appendChild(head);
    var grid = E("div", "settings-grid");
    c.appendChild(grid);
    try {
      var st = await API.stats();
      var p1 = E("div", "panel");
      p1.appendChild(E("h3", null, "运行概况"));
      var sg = E("div", "stat-grid");
      var kinds = st.by_kind || {};
      var images = (kinds.image && kinds.image.count) || 0;
      var pairs = [
        ["文件总数", st.total],
        ["图片数量", images],
        ["占用空间", UI.fmtSize(st.total_size)],
        ["缩略图缓存", UI.fmtSize(st.cache_size)],
        ["分享数量", st.shares],
        ["标签数量", st.tags]
      ];
      pairs.forEach(function (p) {
        var s = E("div", "stat");
        s.appendChild(E("div", "k", p[0]));
        s.appendChild(E("div", "v", String(p[1] === undefined ? 0 : p[1])));
        sg.appendChild(s);
      });
      p1.appendChild(sg);
      p1.appendChild(E("p", "muted", "版本 " + (st.version || "")));
      grid.appendChild(p1);

      var prog = st.progress || {};
      var p2 = E("div", "panel");
      p2.appendChild(E("h3", null, "索引与缓存"));
      var state1 = prog.running ? ("正在索引：" + (prog.current || "")) : "当前空闲";
      p2.appendChild(E("p", "muted", state1 + " · 已扫描 " + (prog.scanned || 0) + " · 跳过 " + (prog.skipped || 0) + " · 失败 " + (prog.failed || 0)));
      var row2 = E("div", "tb-group");
      var b1 = E("button", "chip-btn", "立即全量索引");
      b1.onclick = function () { App.indexNow(""); };
      var b2 = E("button", "chip-btn", "索引当前目录");
      b2.onclick = function () { App.indexNow(Explorer.state.path || ""); };
      var b3 = E("button", "chip-btn", "清理缩略图缓存");
      b3.onclick = async function () {
        try { await API.thumbClean(); UI.toast("缓存已清理"); } catch (e) { UI.toast(e.message); }
      };
      row2.appendChild(b1);
      row2.appendChild(b2);
      row2.appendChild(b3);
      p2.appendChild(row2);
      grid.appendChild(p2);
    } catch (e) { }

    try {
      var cfg = await API.settings();
      var p3 = E("div", "panel");
      p3.appendChild(E("h3", null, "域名绑定"));
      p3.appendChild(E("p", "muted", "每行一个域名，第一个为首选域名。访问者用哪个域名打开，分享链接就用哪个域名生成；保存后立即生效。"));
      var ta = E("textarea");
      ta.rows = 5;
      ta.style.cssText = "width:100%;padding:10px;border:1px solid var(--line2);border-radius:8px;background:transparent;color:inherit;outline:none";
      var urls = (cfg.server && cfg.server.public_urls) || [];
      ta.value = urls.join("\n");
      p3.appendChild(ta);
      var save = E("button", "btn primary", "保存域名");
      save.style.marginTop = "10px";
      save.onclick = async function () {
        var list = ta.value.split("\n").map(function (s) { return s.trim(); }).filter(Boolean);
        try {
          await API.settingsSave({ public_urls: list });
          UI.toast("已保存");
          App.pub = await API.public();
        } catch (e) { UI.toast(e.message); }
      };
      p3.appendChild(save);
      grid.appendChild(p3);

      var p4 = E("div", "panel");
      p4.appendChild(E("h3", null, "文件库"));
      ((cfg.libraries) || []).forEach(function (l) {
        var line = E("div", "link-row");
        line.appendChild(E("span", "u", l.name + "   " + l.path));
        line.appendChild(E("span", "tag", l.readonly ? "只读" : "可写"));
        p4.appendChild(line);
      });
      p4.appendChild(E("p", "muted", "配置文件：" + (cfg.config_path || "")));
      p4.appendChild(E("p", "muted", "数据目录：" + (cfg.data_dir || "")));
      grid.appendChild(p4);

      var p5 = E("div", "panel");
      p5.appendChild(E("h3", null, "账号安全"));
      var b5 = E("button", "chip-btn", "修改登录密码");
      b5.onclick = function () { changePassword(); };
      p5.appendChild(b5);
      var b6 = E("button", "chip-btn", "退出登录");
      b6.style.marginLeft = "8px";
      b6.onclick = async function () { await API.logout(); location.reload(); };
      p5.appendChild(b6);
      grid.appendChild(p5);
      if (window.UpdatePanel) { window.UpdatePanel.render(grid); }
    } catch (e) { }
  }
  App.renderSettings = renderSettings;

  async function changePassword() {
    var f = await UI.form("修改密码", [
      { name: "old", label: "当前密码", type: "password" },
      { name: "neo", label: "新密码（至少 6 位）", type: "password" }
    ], "确认修改");
    if (!f) { return; }
    try {
      await API.changePassword(f.old, f.neo);
      UI.toast("密码已修改");
    } catch (e) { UI.toast(e.message); }
  }
  App.changePassword = changePassword;

  /* ---------- 状态轮询 ---------- */

  var timer = null;
  function startPolling() {
    if (timer) { return; }
    timer = setInterval(async function () {
      try {
        var d = await API.indexStatus();
        var p = d.progress || {};
        var el = $("indexState");
        if (!el) { return; }
        if (p.running) { el.textContent = "索引中 · 已扫描 " + (p.scanned || 0) + " 项"; }
        else if (p.finished_at) { el.textContent = "上次索引：" + UI.fmtTime(p.finished_at); }
      } catch (e) { }
    }, 5000);
  }
  App.startPolling = startPolling;
})();