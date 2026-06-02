const button = document.querySelector("#sync");
const statusEl = document.querySelector("#status");

button.addEventListener("click", async () => {
  button.disabled = true;
  statusEl.className = "";
  statusEl.textContent = "Syncing...";
  try {
    const response = await chrome.runtime.sendMessage({ type: "sync-auth" });
    if (!response || !response.ok) {
      throw new Error(response && response.error ? response.error : "Sync failed");
    }
    const profile = response.result;
    const account = profile.username ? ` as ${profile.username}` : "";
    const names = Array.isArray(profile.cookies) ? profile.cookies.map((cookie) => cookie.name).join(", ") : "";
    statusEl.textContent = `Synced ${profile.cookie_count} cookies${account}${names ? `: ${names}` : ""}.`;
  } catch (err) {
    statusEl.className = "error";
    statusEl.textContent = err && err.message ? err.message : "Sync failed";
  } finally {
    button.disabled = false;
  }
});
