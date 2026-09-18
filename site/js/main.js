// FM NewGen Faces – tiny progressive-enhancement script.
// The site works fully without this: it only (a) highlights/reorders the
// download button matching the visitor's OS, and (b) tries to show the
// exact latest version number and fix up an asset link if the plain
// "-windows.zip"/"-macos.zip"/"-linux.tar.gz" name isn't published yet.
(function () {
  "use strict";

  function detectOS() {
    var ua = (navigator.userAgent || "") + " " + (navigator.platform || "");
    if (/Win/i.test(ua)) return "windows";
    if (/Mac/i.test(ua)) return "macos";
    if (/Linux|X11/i.test(ua)) return "linux";
    return null;
  }

  function highlightOS() {
    var os = detectOS();
    if (!os) return;
    var btn = document.querySelector('.dl-btn[data-os="' + os + '"]');
    var hint = document.getElementById("os-hint");
    if (!btn) return;
    btn.classList.add("is-match");
    var group = btn.closest(".downloads");
    if (group && group.firstElementChild !== btn) {
      group.insertBefore(btn, group.firstElementChild);
    }
    if (hint) {
      var label = os === "macos" ? "macOS" : os.charAt(0).toUpperCase() + os.slice(1);
      hint.textContent = "Looks like you're on " + label + " – we highlighted the right download.";
    }
  }

  function enhanceReleaseInfo() {
    var versionEls = document.querySelectorAll("[data-latest-version]");
    var dlBtns = document.querySelectorAll(".dl-btn[data-asset]");
    if (!versionEls.length && !dlBtns.length) return;

    fetch("https://api.github.com/repos/Chafficui/fm-newgen-faces/releases/latest", {
      headers: { Accept: "application/vnd.github+json" },
    })
      .then(function (r) {
        return r.ok ? r.json() : null;
      })
      .then(function (data) {
        if (!data) return;
        var tag = (data.tag_name || "").replace(/^v/, "");
        if (tag) {
          versionEls.forEach(function (el) {
            el.textContent = "v" + tag;
          });
        }
        var assets = data.assets || [];
        var byName = {};
        assets.forEach(function (a) {
          byName[a.name] = a.browser_download_url;
        });
        dlBtns.forEach(function (btn) {
          var expected = btn.getAttribute("data-asset");
          if (byName[expected]) return; // stable unversioned name exists, keep it
          // Fall back to a versioned asset name for this OS, if present.
          var os = btn.getAttribute("data-os");
          var re =
            os === "windows"
              ? /windows.*\.zip$/i
              : os === "macos"
              ? /(macos|darwin).*\.(zip|tar\.gz)$/i
              : /linux.*\.tar\.gz$/i;
          var match = assets.find(function (a) {
            return re.test(a.name);
          });
          if (match) btn.setAttribute("href", match.browser_download_url);
        });
      })
      .catch(function () {
        /* offline or rate-limited: static links already work */
      });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }

  function init() {
    highlightOS();
    enhanceReleaseInfo();
  }
})();
