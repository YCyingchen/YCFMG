/* YCFMG 图库视图：时间线瀑布流、分类筛选、以图搜图入口 */
(function () {
  "use strict";
  var E = UI.el, I = UI.icon;

  var state = {
    items: [],
    total: 0,
    offset: 0,
    limit: 120,
    filter: { classify: "", lib: "", color: "", year: "", favorite: "" },
    sort: "taken",
    desc: true,
    query: "",
    mode: "browse",
    loading: false
  };

  function container() { return document.getElementById("viewGallery"); }

  function params() {
    var p = { limit: state.limit, offset: state.offset, sort: state.sort, desc: state.desc ? 1 : "", kind: "image" };
    if (state.filter.classify) { p.classify = state.filter.classify; }
    if (state.filter.lib) { p.lib = state.filter.lib; }
    if (state.filter.color) { p.color = state.filter.color; }
    if (state.filter.year) { p.from = Math.floor(new Date(state.filter.year + "-01-01T00:00:00").getTime() / 1000); p.to = p.from + 31536000; }
    if (state.filter.favorite) { p.favorite = 1; }
    if (state.query) { p.q = state.query; }
    return p;
  }

  function chips() {
    var head = E("div", "gal-head");
    var groups = [
      { key: "classify", label: "分类", values: ["", "照片", "人物", "风景", "美食", "夜景", "截图", "长截图", "文档", "动漫", "表情包", "壁纸", "图标", "极简"] },
      { key: "color", label: "色调", values: ["", "红色", "橙色", "黄色", "绿色", "青色", "蓝色", "紫色", "粉色", "白色", "黑色", "灰色"] },
      { key: "favorite", label: "收藏", values: ["", "true"] }
    ];
    groups.forEach(function (g) {
      g.values.forEach(function (v) {
        if (!v) { return; }
        var on = String(state.filter[g.key] || "") === String(v === "true" ? "true" : v);
        var c = E("div", "chip" + (on ? " on" : ""), v === "true" ? "仅收藏" : v);
        c.onclick = function () {
          state.filter[g.key] = on ? "" : (v === "true" ? "true" : v);
          reload();
        };
        head.appendChild(c);
      });
    });
    if (state.filter.classify || state.filter.color || state.filter.favorite || state.query) {
      var reset = E("div", "chip on", "清除筛选");
      reset.onclick = function () {
        state.filter = { classify: "", lib: "", color: "", year: "", favorite: "" };
        state.query = "";
        reload();
      };
      head.appendChild(reset);
    }
    var right = E("div", "fill");
    right.style.flex = "1";
    head.appendChild(right);
    head.appendChild(E("span", "muted", "共 " + state.total + " 张"));
    return head;
  }

  function groupByDay(items) {
    var map = {};
    var order = [];
    items.forEach(function (it) {
      var ts = it.taken_at || it.mtime || 0;
      var d = new Date(ts * 1000);
      var key = d.getFullYear() + "-" + ("0" + (d.getMonth() + 1)).slice(-2) + "-" + ("0" + d.getDate()).slice(-2);
      if (!map[key]) { map[key] = []; order.push(key); }
      map[key].push(it);
    });
    return { map: map, order: order };
  }

  function cell(it) {
    var c = E("div", "cell");
    var img = document.createElement("img");
    img.loading = "lazy";
    img.src = API.thumbURL(it.path, 512);
    img.alt = it.name;
    c.appendChild(img);
    c.appendChild(E("div", "cap", it.name));
    if (it.classify) { c.appendChild(E("div", "badge", it.classify)); }
    c.onclick = function (ev) {
      if (ev.ctrlKey || ev.metaKey) { c.classList.toggle("sel"); return; }
      var imgs = state.items.filter(function (x) { return x.kind === "image"; });
      window.App.openLightbox(it.path, imgs);
    };
    c.oncontextmenu = function (ev) {
      ev.preventDefault();
      ev.stopPropagation();
      var sel = Array.prototype.slice.call(container().querySelectorAll(".cell.sel"));
      var paths = sel.length ? sel.map(function (n) { return n.dataset.path; }) : [it.path];
      UI.menu(ev.clientX, ev.clientY, [
        { label: "查看大图", icon: "i-eye", action: function () { window.App.openLightbox(it.path, state.items); } },
        { label: "下载", icon: "i-download", action: function () { window.location.href = API.fileURL(it.path, true); } },
        { label: "分享", icon: "i-share", action: function () { window.App.sharePaths(paths); } },
        { label: "以图搜图", icon: "i-search", action: function () { window.App.similarByPath(it.path); } },
        { label: "查找重复图", action: function () { window.App.showDuplicates(); } },
        { sep: true },
        { label: it.favorite ? "取消收藏" : "加入收藏", icon: "i-star", action: function () { window.App.toggleFavorite(it); } },
        { label: "显示详细信息", action: function () { window.App.showDetails(it); } },
        { sep: true },
        { label: "删除", icon: "i-trash", danger: true, action: function () { window.App.deletePaths(paths); } }
      ]);
    };
    c.dataset.path = it.path;
    return c;
  }

  function render() {
    var c = container();
    c.innerHTML = "";
    c.appendChild(chips());
    if (state.loading && !state.items.length) {
      var l = E("div", "empty-big");
      l.appendChild(E("div", null, "正在读取图库 ..."));
      c.appendChild(l);
      return;
    }
    if (!state.items.length) {
      var emp = E("div", "empty-big");
      emp.appendChild(I("i-image"));
      emp.appendChild(E("div", null, "没有符合条件的图片"));
      emp.appendChild(E("div", "muted", "试试切换分类，或运行一次索引"));
      c.appendChild(emp);
      return;
    }
    var g = groupByDay(state.items);
    g.order.forEach(function (key) {
      var block = E("div", "gal-day");
      var title = key;
      if (state.sort === "taken" && state.desc) { title = UI.fmtDay(new Date(key + "T12:00:00").getTime() / 1000); }
      var h = E("h3", null, title + " · " + g.map[key].length + " 张");
      block.appendChild(h);
      var wf = E("div", "waterfall");
      g.map[key].forEach(function (it) { wf.appendChild(cell(it)); });
      block.appendChild(wf);
      c.appendChild(block);
    });
    if (state.items.length < state.total) {
      var more = E("div", "empty-big");
      var btn = E("button", "btn primary", "加载更多（已显示 " + state.items.length + " / " + state.total + "）");
      btn.onclick = function () { loadMore(); };
      more.appendChild(btn);
      c.appendChild(more);
    }
    c.oncontextmenu = function (ev) {
      if (ev.target === c) {
        ev.preventDefault();
        UI.menu(ev.clientX, ev.clientY, [
          { label: "刷新图库", icon: "i-refresh", action: reload },
          { label: "查找重复图", action: function () { window.App.showDuplicates(); } },
          { label: "按分类浏览", action: function () { window.App.refreshNav(); } }
        ]);
      }
    };
  }

  async function fetchPage() {
    state.loading = true;
    try {
      var d;
      if (state.query) {
        d = await API.search(params());
      } else {
        d = await API.gallery(params());
      }
      state.items = (d.items || []).filter(function (x) { return x.kind === "image"; });
      state.total = d.total || state.items.length;
      state.mode = d.mode || "browse";
    } catch (e) {
      UI.toast(e.message);
      state.items = [];
      state.total = 0;
    }
    state.loading = false;
  }

  async function reload() { state.offset = 0; await fetchPage(); render(); }

  async function loadMore() {
    state.offset += state.limit;
    state.loading = true;
    try {
      var d = state.query ? await API.search(params()) : await API.gallery(params());
      var more = (d.items || []).filter(function (x) { return x.kind === "image"; });
      state.items = state.items.concat(more);
    } catch (e) { UI.toast(e.message); }
    state.loading = false;
    render();
  }

  async function byText(q) {
    state.query = q || "";
    state.offset = 0;
    await fetchPage();
    render();
  }

  async function byImage(file) {
    var c = container();
    c.innerHTML = "";
    var l = E("div", "empty-big");
    l.appendChild(E("div", null, "正在分析图片并检索相似内容 ..."));
    c.appendChild(l);
    try {
      var d = await API.searchByImage(file);
      state.items = (d.items || []).filter(function (x) { return x.kind === "image"; });
      state.total = d.total || state.items.length;
      state.query = "";
      state.filter = { classify: "", lib: "", color: "", year: "", favorite: "" };
      state.mode = "image";
      render();
      if (!state.items.length) { UI.toast("没有找到相似的图片"); }
    } catch (e) {
      UI.toast(e.message);
      render();
    }
  }

  async function bySimilarId(id) {
    state.loading = true;
    render();
    try {
      var d = await API.similar(id, { limit: state.limit, offset: 0 });
      state.items = (d.items || []).filter(function (x) { return x.kind === "image"; });
      state.total = d.total || state.items.length;
      state.mode = "similar";
    } catch (e) { UI.toast(e.message); }
    state.loading = false;
    render();
  }

  async function setFilter(f) {
    Object.keys(f).forEach(function (k) { state.filter[k] = f[k]; });
    await reload();
  }

  window.Gallery = {
    state: state,
    reload: reload,
    render: render,
    byText: byText,
    byImage: byImage,
    bySimilarId: bySimilarId,
    setFilter: setFilter,
    setQuery: function (q) { state.query = q; }
  };
})();