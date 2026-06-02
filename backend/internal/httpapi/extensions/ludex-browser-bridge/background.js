const LUDEX_BASE_URL = "http://127.0.0.1:8787";

const ADAPTERS = [
  {
    id: "f95zone",
    name: "F95zone",
    domain: "f95zone.to",
    cookieURL: "https://f95zone.to/",
    matches(url) {
      try {
        return new URL(url).hostname === "f95zone.to";
      } catch (err) {
        return false;
      }
    }
  }
];

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (!message || message.type !== "sync-auth") {
    return false;
  }
  syncAuthProfile()
    .then((result) => sendResponse({ ok: true, result }))
    .catch((err) => sendResponse({ ok: false, error: err && err.message ? err.message : "Sync failed" }));
  return true;
});

async function syncAuthProfile() {
  const tab = await activeTab();
  const adapter = adapterForTab(tab) || ADAPTERS[0];
  const cookies = await getCookies({ url: adapter.cookieURL });
  if (!cookies.length) {
    throw new Error(`No ${adapter.name} cookies found`);
  }
  const cookieHeader = cookies.map((cookie) => `${cookie.name}=${cookie.value}`).join("; ");
  const username = await usernameFromTab(tab, adapter);
  const response = await fetch(`${LUDEX_BASE_URL}/api/auth-profiles/import`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      adapter_id: adapter.id,
      domain: adapter.domain,
      cookie_header: cookieHeader,
      cookies: cookies.map(sanitizeCookie),
      username,
      user_agent: navigator.userAgent,
      source_url: tab && adapter.matches(tab.url) ? tab.url : adapter.cookieURL
    })
  });
  const json = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(json.error || `Ludex returned HTTP ${response.status}`);
  }
  return json;
}

function adapterForTab(tab) {
  if (!tab || !tab.url) {
    return null;
  }
  return ADAPTERS.find((adapter) => adapter.matches(tab.url)) || null;
}

function activeTab() {
  return new Promise((resolve, reject) => {
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      const err = chrome.runtime.lastError;
      if (err) {
        reject(new Error(err.message));
        return;
      }
      resolve(tabs && tabs.length ? tabs[0] : null);
    });
  });
}

function getCookies(details) {
  return new Promise((resolve, reject) => {
    chrome.cookies.getAll(details, (cookies) => {
      const err = chrome.runtime.lastError;
      if (err) {
        reject(new Error(err.message));
        return;
      }
      resolve(cookies || []);
    });
  });
}

async function usernameFromTab(tab, adapter) {
  if (!tab || !tab.id || !adapter.matches(tab.url)) {
    return "";
  }
  try {
    const results = await chrome.scripting.executeScript({
      target: { tabId: tab.id },
      func: readUsernameFromPage
    });
    return (results && results[0] && results[0].result) || "";
  } catch (err) {
    return "";
  }
}

function readUsernameFromPage() {
  const selectors = [
    ".p-navgroup-link--user .p-navgroup-linkText",
    ".p-navgroup-link--user",
    "a[href*='/account/'][data-xf-click='menu']",
    "a[href*='/members/'].username",
    "a[href*='/members/'][data-user-id]",
    ".avatar[data-user-id]",
    ".username"
  ];
  function cleanText(value) {
    return (value || "").replace(/\s+/g, " ").trim();
  }
  function usernameFromElement(element) {
    if (!element) {
      return "";
    }
    const candidates = [
      element.getAttribute("data-username"),
      element.getAttribute("data-user-name"),
      element.getAttribute("title"),
      element.getAttribute("aria-label"),
      element.textContent
    ];
    for (const candidate of candidates) {
      const username = cleanText(candidate)
        .replace(/^account\s*/i, "")
        .replace(/^profile\s*/i, "")
        .replace(/\s*menu$/i, "");
      if (username && !/^(log in|login|register|sign up|alerts|inbox)$/i.test(username)) {
        return username;
      }
    }
    return "";
  }
  for (const selector of selectors) {
    const username = usernameFromElement(document.querySelector(selector));
    if (username) {
      return username;
    }
  }
  return "";
}

function sanitizeCookie(cookie) {
  return {
    name: cookie.name,
    domain: cookie.domain || "f95zone.to",
    path: cookie.path || "/",
    expires_at: cookie.expirationDate ? new Date(Number(cookie.expirationDate) * 1000).toISOString() : "",
    session: Boolean(cookie.session) || !cookie.expirationDate,
    secure: Boolean(cookie.secure),
    http_only: Boolean(cookie.httpOnly)
  };
}
