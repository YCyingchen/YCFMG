/* YCFMG 前端 API 封装：统一处理 JSON、错误与未登录跳转 */
(function () {
  "use strict";
  var BASE = "";

  function withBase(p) {
    if (!p) { return BASE + "/"; }
    if (p.charAt(0) !== "/") { p = "/" + p; }
    return BASE + p;
  }

  async function request(method, url, body, isForm) {
    var opts = { method: method, credentials: "same-origin", headers: {} };
    if (body !== undefined && body !== null) {
      if (isForm) {
        opts.body = body;
      } else {
        opts.headers["Content-Type"] = "application/json";
        opts.body = JSON.stringify(body);
      }
    }
    var resp;
    try {
      resp = await fetch(withBase(url), opts);
    } catch (e) {
      throw new Error("网络请求失败，请检查连接");
    }
    var text = await resp.text();
    var data = null;
    if (text) {
      try { data = JSON.parse(text); } catch (e) { data = null; }
    }
    if (resp.status === 401) {
      if (window.App && window.App.onUnauthorized) { window.App.onUnauthorized(); }
      throw new Error((data && data.error) || "未登录");
    }
    if (!resp.ok || (data && data.ok === false)) {
      throw new Error((data && data.error) || ("请求失败 (" + resp.status + ")"));
    }
    if (data && Object.prototype.hasOwnProperty.call(data, "data")) { return data.data; }
    return data;
  }

  function qs(obj) {
    var parts = [];
    for (var k in obj) {
      if (!Object.prototype.hasOwnProperty.call(obj, k)) { continue; }
      var v = obj[k];
      if (v === undefined || v === null || v === "") { continue; }
      parts.push(encodeURIComponent(k) + "=" + encodeURIComponent(v));
    }
    return parts.length ? ("?" + parts.join("&")) : "";
  }

  window.API = {
    setBase: function (b) { BASE = b || ""; },
    base: function () { return BASE; },
    qs: qs,
    thumbURL: function (path, size) {
      return withBase("/api/thumb" + qs({ path: path, size: size || 256 }));
    },
    fileURL: function (path, download) {
      return withBase("/api/file" + qs({ path: path, download: download ? 1 : "" }));
    },
    public: function () { return request("GET", "/api/public"); },
    health: function () { return request("GET", "/api/health"); },
    login: function (u, p) { return request("POST", "/api/login", { username: u, password: p }); },
    logout: function () { return request("POST", "/api/logout"); },
    me: function () { return request("GET", "/api/me"); },
    changePassword: function (o, n) { return request("POST", "/api/password", { old: o, new: n }); },

    list: function (path, hidden) { return request("GET", "/api/fs/list" + qs({ path: path, hidden: hidden ? 1 : "" })); },
    tree: function () { return request("GET", "/api/fs/tree"); },
    mkdir: function (path, name) { return request("POST", "/api/fs/mkdir", { path: path, name: name }); },
    rename: function (path, newName) { return request("POST", "/api/fs/rename", { path: path, new_name: newName }); },
    move: function (paths, dst) { return request("POST", "/api/fs/move", { paths: paths, dst: dst }); },
    copy: function (paths, dst) { return request("POST", "/api/fs/copy", { paths: paths, dst: dst }); },
    remove: function (paths, forever) { return request("POST", "/api/fs/delete", { paths: paths, forever: !!forever }); },
    trashList: function () { return request("GET", "/api/fs/trash"); },
    trashRestore: function (id) { return request("POST", "/api/fs/trash/restore", { id: id }); },
    trashPurge: function (ids) { return request("POST", "/api/fs/trash/purge", { ids: ids || [] }); },
    indexNow: function (path, recursive) { return request("GET", "/api/fs/index" + qs({ path: path, recursive: recursive ? 1 : "" })); },
    upload: function (dir, file, onProgress) {
      return new Promise(function (resolve, reject) {
        var fd = new FormData();
        fd.append("path", dir);
        fd.append("file", file, file.name);
        var xhr = new XMLHttpRequest();
        xhr.open("POST", withBase("/api/fs/upload"));
        xhr.withCredentials = true;
        if (onProgress) {
          xhr.upload.onprogress = function (e) {
            if (e.lengthComputable) { onProgress(e.loaded / e.total); }
          };
        }
        xhr.onload = function () {
          try {
            var d = JSON.parse(xhr.responseText);
            if (d && d.ok) { resolve(d.data); } else { reject(new Error((d && d.error) || "上传失败")); }
          } catch (e) { reject(new Error("上传响应解析失败")); }
        };
        xhr.onerror = function () { reject(new Error("上传失败")); };
        xhr.send(fd);
      });
    },

    gallery: function (o) { return request("GET", "/api/gallery" + qs(o || {})); },
    facets: function (kind) { return request("GET", "/api/gallery/facets" + qs({ kind: kind || "image" })); },
    mapPoints: function () { return request("GET", "/api/gallery/map"); },
    duplicates: function (lib, tol) { return request("GET", "/api/gallery/duplicates" + qs({ lib: lib, tolerance: tol })); },
    search: function (o) { return request("GET", "/api/search" + qs(o || {})); },
    similar: function (id, o) { o = o || {}; o.id = id; return request("GET", "/api/search/similar" + qs(o)); },
    suggest: function () { return request("GET", "/api/search/suggest"); },
    searchByImage: function (file) {
      var fd = new FormData();
      fd.append("file", file, file.name || "query.jpg");
      return request("POST", "/api/search/image", fd, true);
    },
    fileMeta: function (payload) { return request("POST", "/api/file/meta", payload); },
    tags: function () { return request("GET", "/api/tags"); },

    shares: function (o) { return request("GET", "/api/shares" + qs(o || {})); },
    shareCreate: function (payload) { return request("POST", "/api/shares", payload); },
    shareGet: function (id) { return request("GET", "/api/shares/" + id); },
    shareUpdate: function (id, payload) { return request("PATCH", "/api/shares/" + id, payload); },
    shareDelete: function (id) { return request("DELETE", "/api/shares/" + id); },
    shareLinks: function (id) { return request("GET", "/api/shares/" + id + "/links"); },

    stats: function () { return request("GET", "/api/stats"); },
    libraries: function () { return request("GET", "/api/libraries"); },
    indexStatus: function () { return request("GET", "/api/index/status"); },
    indexScan: function (payload) { return request("POST", "/api/index/scan", payload || {}); },
    settings: function () { return request("GET", "/api/settings"); },
    settingsSave: function (payload) { return request("POST", "/api/settings", payload); },
    thumbClean: function () { return request("POST", "/api/thumbs/clean"); }
  };
})();