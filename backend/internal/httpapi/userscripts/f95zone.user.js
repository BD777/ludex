// ==UserScript==
// @name         Ludex F95zone Importer
// @namespace    http://127.0.0.1:8787/ludex
// @version      0.1.0
// @description  Send the current logged-in F95zone thread HTML to local Ludex.
// @author       Ludex
// @updateURL    http://127.0.0.1:8787/userscripts/f95zone.user.js
// @downloadURL  http://127.0.0.1:8787/userscripts/f95zone.user.js
// @match        https://f95zone.to/threads/*
// @match        https://f95zone.to/threads/*/
// @grant        GM_xmlhttpRequest
// @connect      127.0.0.1
// @connect      localhost
// ==/UserScript==

(function () {
  "use strict";

  const LUDEX_BASE_URL = "http://127.0.0.1:8787";
  const BUTTON_ID = "ludex-import-f95zone";
  const STYLE_ID = "ludex-import-style";

  function installStyle() {
    if (document.getElementById(STYLE_ID)) {
      return;
    }
    const style = document.createElement("style");
    style.id = STYLE_ID;
    style.textContent = `
      #${BUTTON_ID} {
        position: fixed;
        right: 18px;
        bottom: 18px;
        z-index: 2147483647;
        display: inline-flex;
        align-items: center;
        gap: 8px;
        min-height: 38px;
        padding: 0 13px;
        border: 0;
        border-radius: 7px;
        color: #ffffff;
        background: #2f6f57;
        box-shadow: 0 14px 32px rgba(0, 0, 0, 0.25);
        font: 700 13px/1.2 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
        cursor: pointer;
      }

      #${BUTTON_ID}:hover {
        background: #265f4a;
      }

      #${BUTTON_ID}[disabled] {
        cursor: default;
        opacity: 0.72;
      }

      #${BUTTON_ID}.ludex-error {
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

  function postToLudex(payload) {
    const url = `${LUDEX_BASE_URL}/api/import/f95zone`;
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

  async function importCurrentThread(button) {
    setButtonState(button, "Sending to Ludex...", "busy");
    try {
      const result = await postToLudex({
        url: window.location.href,
        html: pageHTML(),
        create_game: true
      });
      const taskID = result && result.task && result.task.id ? ` #${result.task.id}` : "";
      setButtonState(button, `Imported${taskID}`, "ok");
      window.setTimeout(() => setButtonState(button, "Import to Ludex", "idle"), 3500);
    } catch (err) {
      setButtonState(button, err && err.message ? err.message : "Import failed", "error");
      window.setTimeout(() => setButtonState(button, "Import to Ludex", "idle"), 6000);
    }
  }

  function mountButton() {
    if (document.getElementById(BUTTON_ID)) {
      return;
    }
    installStyle();
    const button = document.createElement("button");
    button.id = BUTTON_ID;
    button.type = "button";
    button.textContent = "Import to Ludex";
    button.addEventListener("click", () => {
      void importCurrentThread(button);
    });
    document.body.appendChild(button);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", mountButton, { once: true });
  } else {
    mountButton();
  }
})();
