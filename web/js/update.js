/* YCFMG 在线更新面板：检查下载站 / GitHub / Docker Hub，并支持代理设置 */
(function () {
  "use strict";
  var E = UI.el;

  function render(grid) {
    var panel = E("div", "panel");
    panel.appendChild(E("h3", null, "在线更新"));
    panel.appendChild(E("p", "muted", "从官方下载站、GitHub、Docker Hub 检查新版本。GitHub 与 Docker Hub 在境内通常需要代理。"));

    var info = E("p", "muted", "读取配置中 ...");
    panel.appendChild(info);
    var body = E("div");
    panel.appendChild(body);

    var row = E("div", "tb-group");
    row.style.marginTop = "10px";
    var proxy = E("input");
    proxy.placeholder = "代理地址，如 http://192.168.1.8:7890";
    proxy.style.cssText = "flex:1;min-width:220px;padding:8px 10px;border:1px solid var(--line2);border-radius:8px;background:transparent;color:inherit;outline:none";
    var saveBtn = E("button", "chip-btn", "保存代理");
    var checkBtn = E("button", "chip-btn", "检查更新");
    row.appendChild(proxy);
    row.appendChild(saveBtn);
    row.appendChild(checkBtn);
    panel.appendChild(row);
    grid.appendChild(panel);

    function loadCfg() {
      API.updateConfig().then(function (c) {
        info.textContent = "当前版本 " + (c.current || "-") + " · 下载页 " + (c.download_page || "-");
        proxy.value = c.proxy || "";
      }).catch(function (e) { info.textContent = e.message; });
    }
    loadCfg();

    saveBtn.onclick = function () {
      API.updateSave({ proxy: proxy.value.trim() }).then(function () {
        UI.toast("代理已保存");
        loadCfg();
      }).catch(function (e) { UI.toast(e.message); });
    };

    function line(name, text, ok, tag) {
      var r = E("div", "link-row");
      var u = E("span", "u", name + "：" + text);
      r.appendChild(u);
      if (tag) { r.appendChild(E("span", "tag", tag)); }
      if (!ok) { r.style.opacity = ".75"; }
      return r;
    }

    checkBtn.onclick = function () {
      checkBtn.disabled = true;
      body.innerHTML = "";
      body.appendChild(E("p", "muted", "正在检查 ..."));
      API.updateCheck().then(function (d) {
        body.innerHTML = "";
        var msg = d.has_update
          ? ("发现新版本 " + d.latest + "（当前 " + d.current + "）")
          : ("已是最新版本 " + d.current);
        body.appendChild(E("p", d.has_update ? "ok" : "muted", msg));
        (d.sources || []).forEach(function (x) {
          body.appendChild(line(x.name, x.ok ? ("v" + x.version) : (x.error || "不可用"), x.ok, x.proxy ? "代理" : ""));
        });
        if ((d.notes || []).length) {
          body.appendChild(E("h3", null, "更新记录"));
          d.notes.forEach(function (n) { body.appendChild(E("p", "muted", "· " + n)); });
        }
        if (d.download) {
          var a = E("a", "btn primary", "前往下载页");
          a.href = d.download;
          a.target = "_blank";
          a.style.marginTop = "10px";
          a.style.display = "inline-flex";
          body.appendChild(a);
        }
      }).catch(function (e) {
        body.innerHTML = "";
        body.appendChild(E("p", "badge-danger", e.message));
      }).then(function () { checkBtn.disabled = false; });
    };
  }

  window.UpdatePanel = { render: render };
})();