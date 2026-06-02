// ==UserScript==
// @name         Ludex Browser Bridge
// @namespace    http://127.0.0.1:8787/ludex
// @version      0.3.0
// @description  Sync supported source auth profiles to local Ludex.
// @author       Ludex
// @updateURL    http://127.0.0.1:8787/userscripts/ludex.user.js
// @downloadURL  http://127.0.0.1:8787/userscripts/ludex.user.js
// @match        https://f95zone.to/*
// @grant        GM_xmlhttpRequest
// @grant        GM_cookie
// @grant        GM_registerMenuCommand
// @connect      127.0.0.1
// @connect      localhost
// ==/UserScript==

(function () {
  "use strict";

  const LUDEX_BASE_URL = "http://127.0.0.1:8787";
  const TOAST_ID = "ludex-browser-bridge-toast";
  const STYLE_ID = "ludex-browser-bridge-style";

  const ADAPTERS = [
    {
      id: "f95zone",
      name: "F95zone",
      domain: "f95zone.to",
      matches(location) {
        return location.hostname === "f95zone.to";
      }
    }
  ];

  function currentAdapter() {
    return ADAPTERS.find((adapter) => adapter.matches(window.location));
  }

  function installStyle() {
    if (document.getElementById(STYLE_ID)) {
      return;
    }
    const style = document.createElement("style");
    style.id = STYLE_ID;
    style.textContent = `
      #${TOAST_ID} {
        position: fixed;
        right: 18px;
        bottom: 18px;
        z-index: 2147483647;
        display: flex;
        align-items: center;
        min-height: 38px;
        max-width: min(360px, calc(100vw - 36px));
        padding: 0 13px;
        overflow: hidden;
        border-radius: 7px;
        color: #ffffff;
        background: #2f6f57;
        box-shadow: 0 14px 32px rgba(0, 0, 0, 0.25);
        font: 700 13px/1.2 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
        text-overflow: ellipsis;
        white-space: nowrap;
        pointer-events: none;
      }

      #${TOAST_ID}.ludex-error {
        background: #a83b3b;
      }
    `;
    document.head.appendChild(style);
  }

  function showToast(message, state) {
    installStyle();
    const previous = document.getElementById(TOAST_ID);
    if (previous) {
      previous.remove();
    }
    const toast = document.createElement("div");
    toast.id = TOAST_ID;
    toast.classList.toggle("ludex-error", state === "error");
    toast.textContent = message;
    document.body.appendChild(toast);
    window.setTimeout(() => {
      toast.remove();
    }, state === "error" ? 6000 : 3500);
  }

  function postJSON(path, payload) {
    const url = `${LUDEX_BASE_URL}${path}`;
    const body = JSON.stringify(payload);

    if (typeof GM_xmlhttpRequest === "function") {
      return new Promise((resolve, reject) => {
        GM_xmlhttpRequest({
          method: "POST",
          url,
          data: body,
          headers: { "Content-Type": "application/json" },
          timeout: 30000,
          onload: (response) => {
            let json = {};
            try {
              json = JSON.parse(response.responseText || "{}");
            } catch (err) {
              reject(new Error(`Ludex returned invalid JSON: ${err.message}`));
              return;
            }
            if (response.status >= 200 && response.status < 300) {
              resolve(json);
            } else {
              reject(new Error(json.error || `Ludex returned HTTP ${response.status}`));
            }
          },
          ontimeout: () => reject(new Error("Timed out connecting to Ludex")),
          onerror: () => reject(new Error("Could not connect to Ludex"))
        });
      });
    }

    return fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body
    }).then(async (response) => {
      const json = await response.json().catch(() => ({}));
      if (!response.ok) {
        throw new Error(json.error || response.statusText);
      }
      return json;
    });
  }

  function readableDocumentCookie() {
    return (document.cookie || "").trim();
  }

  function cookieHeaderFromGM(adapter) {
    if (typeof GM_cookie === "undefined" || typeof GM_cookie.list !== "function") {
      return Promise.resolve("");
    }
    return new Promise((resolve) => {
      try {
        GM_cookie.list({ url: window.location.href }, (cookies, error) => {
          if (error || !Array.isArray(cookies)) {
            resolve("");
            return;
          }
          resolve(
            cookies
              .filter((cookie) => cookie && cookie.name && typeof cookie.value === "string")
              .map((cookie) => `${cookie.name}=${cookie.value}`)
              .join("; ")
          );
        });
      } catch (err) {
        resolve("");
      }
    });
  }

  async function cookieHeaderForAdapter(adapter) {
    const fromGM = await cookieHeaderFromGM(adapter);
    if (fromGM) {
      return fromGM;
    }
    return readableDocumentCookie();
  }

  async function syncAuthProfile(adapter) {
    showToast(`Syncing ${adapter.name} auth...`, "busy");
    try {
      const cookieHeader = await cookieHeaderForAdapter(adapter);
      if (!cookieHeader) {
        throw new Error("No readable cookies found");
      }
      const result = await postJSON("/api/auth-profiles/import", {
        adapter_id: adapter.id,
        domain: adapter.domain,
        cookie_header: cookieHeader,
        user_agent: navigator.userAgent,
        source_url: window.location.href
      });
      const count = result && typeof result.cookie_count === "number" ? result.cookie_count : 0;
      showToast(`${adapter.name} auth synced (${count} cookies)`, "ok");
    } catch (err) {
      const message = err && err.message ? err.message : "Auth sync failed";
      showToast(message, "error");
    }
  }

  function registerMenuCommand(adapter) {
    if (typeof GM_registerMenuCommand === "function") {
      GM_registerMenuCommand(`Sync ${adapter.name} auth to Ludex`, () => {
        void syncAuthProfile(adapter);
      });
      return;
    }
    console.warn("Ludex Browser Bridge requires Tampermonkey menu command support.");
  }

  function boot() {
    const adapter = currentAdapter();
    if (!adapter) {
      return;
    }
    registerMenuCommand(adapter);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot, { once: true });
  } else {
    boot();
  }
})();
