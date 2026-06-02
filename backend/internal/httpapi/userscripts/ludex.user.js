// ==UserScript==
// @name         Ludex Browser Bridge
// @namespace    http://127.0.0.1:8787/ludex
// @version      0.2.0
// @description  Send supported logged-in source pages to local Ludex.
// @author       Ludex
// @updateURL    http://127.0.0.1:8787/userscripts/ludex.user.js
// @downloadURL  http://127.0.0.1:8787/userscripts/ludex.user.js
// @match        https://f95zone.to/threads/*
// @match        https://f95zone.to/threads/*/
// @grant        GM_xmlhttpRequest
// @grant        GM_cookie
// @grant        GM_registerMenuCommand
// @connect      127.0.0.1
// @connect      localhost
// ==/UserScript==

(function () {
  "use strict";

  const LUDEX_BASE_URL = "http://127.0.0.1:8787";
  const WRAPPER_ID = "ludex-browser-bridge";
  const STYLE_ID = "ludex-browser-bridge-style";

  const ADAPTERS = [
    {
      id: "f95zone",
      name: "F95zone",
      domain: "f95zone.to",
      endpoint: "/api/import/f95zone",
      matches(location) {
        return location.hostname === "f95zone.to" && location.pathname.startsWith("/threads/");
      },
      buildPayload() {
        return {
          url: window.location.href,
          html: pageHTML(),
          create_game: true
        };
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
      #${WRAPPER_ID} {
        position: fixed;
        right: 18px;
        bottom: 18px;
        z-index: 2147483647;
        display: flex;
        flex-direction: column;
        align-items: flex-end;
        gap: 8px;
        max-width: min(360px, calc(100vw - 36px));
      }

      #${WRAPPER_ID} button {
        display: inline-flex;
        align-items: center;
        gap: 8px;
        min-height: 38px;
        max-width: 100%;
        padding: 0 13px;
        overflow: hidden;
        border: 0;
        border-radius: 7px;
        color: #ffffff;
        background: #2f6f57;
        box-shadow: 0 14px 32px rgba(0, 0, 0, 0.25);
        font: 700 13px/1.2 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
        text-overflow: ellipsis;
        white-space: nowrap;
        cursor: pointer;
      }

      #${WRAPPER_ID} button:hover {
        background: #265f4a;
      }

      #${WRAPPER_ID} button[disabled] {
        cursor: default;
        opacity: 0.72;
      }

      #${WRAPPER_ID} button.ludex-secondary {
        color: #243d35;
        background: #f3efe6;
      }

      #${WRAPPER_ID} button.ludex-secondary:hover {
        background: #e8e1d4;
      }

      #${WRAPPER_ID} button.ludex-error {
        color: #ffffff;
        background: #a83b3b;
      }
    `;
    document.head.appendChild(style);
  }

  function setButtonState(button, text, state) {
    button.textContent = text;
    button.classList.toggle("ludex-error", state === "error");
    button.disabled = state === "busy";
  }

  function pageHTML() {
    const doctype = document.doctype
      ? `<!DOCTYPE ${document.doctype.name}>`
      : "<!DOCTYPE html>";
    return doctype + "\n" + document.documentElement.outerHTML;
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

  function postToLudex(adapter, payload) {
    return postJSON(adapter.endpoint, payload);
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

  async function importCookies(adapter, button) {
    if (button) {
      setButtonState(button, "Saving cookies...", "busy");
    }
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
      if (button) {
        setButtonState(button, `Cookies saved (${count})`, "ok");
        window.setTimeout(() => setButtonState(button, "Cookies", "idle"), 3500);
      }
    } catch (err) {
      if (button) {
        setButtonState(button, err && err.message ? err.message : "Cookie import failed", "error");
        window.setTimeout(() => setButtonState(button, "Cookies", "idle"), 6000);
      } else {
        window.alert(err && err.message ? err.message : "Cookie import failed");
      }
    }
  }

  async function importCurrentPage(adapter, button) {
    setButtonState(button, `Sending ${adapter.name} to Ludex...`, "busy");
    try {
      const result = await postToLudex(adapter, adapter.buildPayload());
      const taskID = result && result.task && result.task.id ? ` #${result.task.id}` : "";
      setButtonState(button, `Imported${taskID}`, "ok");
      window.setTimeout(() => setButtonState(button, `Import ${adapter.name} to Ludex`, "idle"), 3500);
    } catch (err) {
      setButtonState(button, err && err.message ? err.message : "Import failed", "error");
      window.setTimeout(() => setButtonState(button, `Import ${adapter.name} to Ludex`, "idle"), 6000);
    }
  }

  function mountButton(adapter) {
    if (document.getElementById(WRAPPER_ID)) {
      return;
    }
    installStyle();
    const wrapper = document.createElement("div");
    wrapper.id = WRAPPER_ID;

    const importButton = document.createElement("button");
    importButton.type = "button";
    importButton.title = `Send the current ${adapter.name} page HTML to local Ludex`;
    importButton.textContent = `Import ${adapter.name} to Ludex`;
    importButton.addEventListener("click", () => {
      void importCurrentPage(adapter, importButton);
    });

    const cookieButton = document.createElement("button");
    cookieButton.type = "button";
    cookieButton.className = "ludex-secondary";
    cookieButton.title = `Save readable ${adapter.name} cookies to local Ludex`;
    cookieButton.textContent = "Cookies";
    cookieButton.addEventListener("click", () => {
      void importCookies(adapter, cookieButton);
    });

    wrapper.append(importButton, cookieButton);
    document.body.appendChild(wrapper);

    if (typeof GM_registerMenuCommand === "function") {
      GM_registerMenuCommand(`Import ${adapter.name} cookies to Ludex`, () => {
        void importCookies(adapter, null);
      });
    }
  }

  function boot() {
    const adapter = currentAdapter();
    if (!adapter) {
      return;
    }
    mountButton(adapter);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", boot, { once: true });
  } else {
    boot();
  }
})();
