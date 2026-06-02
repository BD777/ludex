<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from "vue";
import Icon from "./components/Icon.vue";

type Game = {
  id: number;
  title: string;
  aliases: string[];
  description: string;
  current_version: string;
  cover_image: string;
  created_at: string;
  updated_at: string;
};

type Source = {
  id: number;
  name: string;
  type: string;
  url: string;
  proxy_url: string;
  enabled: boolean;
  trust_level: number;
  config_json: string;
  created_at: string;
  updated_at: string;
};

type AuthProfile = {
  id: number;
  adapter_id: string;
  domain: string;
  cookie_count: number;
  cookie_expires_at: string;
  cookies: AuthCookie[];
  username: string;
  user_agent: string;
  source_url: string;
  imported_at: string;
  last_used_at: string;
  created_at: string;
  updated_at: string;
};

type AuthCookie = {
  name: string;
  domain: string;
  path: string;
  expires_at: string;
  session: boolean;
  secure: boolean;
  http_only: boolean;
};

type SourceItem = {
  id: number;
  source_id?: number;
  source_type: string;
  external_id: string;
  title: string;
  raw_url: string;
  raw_content_path: string;
  parsed_json: Transcript;
  fetched_at: string;
  matched_game_id?: number;
  status: string;
  created_at: string;
  updated_at: string;
};

type Task = {
  id: number;
  kind: string;
  dedupe_key: string;
  status: "queued" | "running" | "succeeded" | "failed";
  title: string;
  message: string;
  progress_current: number;
  progress_total: number;
  result_json?: {
    item?: SourceItem;
    game?: Game;
    transcript?: Transcript;
  };
  error: string;
  created_at: string;
  started_at: string;
  finished_at: string;
  updated_at: string;
};

type Transcript = {
  source?: string;
  source_url?: string;
  external_id?: string;
  title?: string;
  fields?: TranscriptFields;
  key_values?: Record<string, string[]>;
  sections?: Array<{ heading: string; body: string }>;
  images?: string[];
  media_items?: MediaItem[];
  media_failures?: MediaFailure[];
  tags?: string[];
  inferred?: {
    game_title?: string;
    version?: string;
    developer?: string;
    description?: string;
    cover_image?: string;
  };
  warnings?: string[];
};

type MediaItem = {
  position: number;
  role?: string;
  status: "cached" | "failed";
  original_url: string;
  public_url?: string;
  error?: string;
};

type MediaFailure = {
  position: number;
  role?: string;
  original_url: string;
  error: string;
};

type MediaEntry = {
  key: string;
  position: number;
  role: string;
  status: "cached" | "failed";
  original_url: string;
  public_url: string;
  error: string;
};

type NamedURL = {
  name: string;
  url?: string;
};

type DownloadGroup = {
  platform: string;
  note?: string;
  links?: NamedURL[];
};

type TranscriptFields = {
  game_name?: string;
  prefixes?: string[];
  engine?: string;
  cover_image?: string;
  description?: string;
  thread_updated?: string;
  release_date?: string;
  developer?: string;
  developer_links?: NamedURL[];
  censored?: boolean | null;
  version?: string;
  operating_systems?: string[];
  languages?: string[];
  genres?: string[];
  changelog?: string;
  changelog_html?: string;
  download_groups?: DownloadGroup[];
  screenshots?: string[];
};

type ViewName = "games" | "adapters" | "import" | "items" | "tasks";

type GamePanelMode = "detail" | "edit" | "new";

type DeleteTarget =
  | {
      kind: "game";
      id: number;
      title: string;
      firstMessage: string;
      secondMessage: string;
    }
  | {
      kind: "item";
      id: number;
      title: string;
      firstMessage: string;
      secondMessage: string;
    };

const view = ref<ViewName>("games");
const loading = ref(false);
const error = ref("");
const notice = ref("");
const query = ref("");

const games = ref<Game[]>([]);
const sources = ref<Source[]>([]);
const authProfiles = ref<AuthProfile[]>([]);
const sourceItems = ref<SourceItem[]>([]);
const tasks = ref<Task[]>([]);
const selectedGame = ref<Game | null>(null);
const selectedItem = ref<SourceItem | null>(null);
const selectedTask = ref<Task | null>(null);
const lastImportTaskId = ref<number | null>(null);
const selectedTaskSourceItemId = ref<number | null>(null);
const pendingDelete = ref<DeleteTarget | null>(null);
const deleting = ref(false);
const previewImage = ref("");
const previewSequence = ref<string[]>([]);
const gamePanelMode = ref<GamePanelMode>("detail");
const taskHistoryExpanded = ref(false);
const matchEditing = ref(false);
const mediaRetrying = ref(false);
const retryingMediaURL = ref("");
let noticeTimer: number | undefined;
let errorTimer: number | undefined;

const importDraftStorageKey = "gmb.importDraft.v1";
const lastImportTaskStorageKey = "gmb.lastImportTaskId.v1";

const gameDraft = reactive({
  id: 0,
  title: "",
  aliases: "",
  description: "",
  current_version: "",
  cover_image: ""
});

const importDraftDefaults = {
  url: "",
  proxy_url: "",
  create_game: true
};

const importDraft = reactive({ ...importDraftDefaults });
const selectedAdapterId = ref("f95zone");

const browserBridge = {
  name: "Ludex Browser Bridge",
  extensionPackage: "/extensions/ludex-browser-bridge.zip",
  mode: "Chromium extension",
  coverage: "F95zone now, more adapters later",
  install: "Download ZIP, then Load unpacked",
  action: "Use the extension popup to sync auth",
  extension: "Reads HttpOnly cookies via chrome.cookies",
  extensionSource: "backend/internal/httpapi/extensions/ludex-browser-bridge",
  limitation: "Chrome/Edge local extensions cannot be one-click installed from a web page"
};

const builtInAdapters = [
  {
    id: "f95zone",
    name: "F95zone",
    kind: "Forum thread",
    status: "built-in",
    input: "Thread URL / raw HTML",
    dedupe: "Thread ID",
    endpoint: "/api/import/f95zone",
    attachments: "Cached images",
    authDomain: "f95zone.to",
    authCookieNames: ["xf_user"],
    withoutBridge: "Download links, login-only spoilers/changelog, and some developer/social links may be unavailable."
  }
] as const;

type BuiltInAdapter = (typeof builtInAdapters)[number];

const matchGameId = ref("");

const filteredGames = computed(() => {
  const needle = query.value.trim().toLowerCase();
  if (!needle) return games.value;
  return games.value.filter((game) => {
    return (
      game.title.toLowerCase().includes(needle) ||
      game.aliases.some((alias) => alias.toLowerCase().includes(needle))
    );
  });
});

const selectedTranscript = computed(() => selectedItem.value?.parsed_json ?? {});

const keyValues = computed(() => {
  const values = selectedTranscript.value.key_values ?? {};
  return Object.entries(values);
});

const selectedGameSourceItems = computed(() => {
  if (!selectedGame.value) {
    return [];
  }
  return sourceItems.value
    .filter((item) => item.matched_game_id === selectedGame.value?.id)
    .sort((left, right) => {
      const leftTime = Date.parse(left.updated_at || left.fetched_at || left.created_at);
      const rightTime = Date.parse(right.updated_at || right.fetched_at || right.created_at);
      return rightTime - leftTime;
    });
});

const selectedGameSourceItem = computed(() => selectedGameSourceItems.value[0] ?? null);

const selectedGameTranscript = computed(() => selectedGameSourceItem.value?.parsed_json ?? {});

const selectedGameKeyValues = computed(() => {
  const values = selectedGameTranscript.value.key_values ?? {};
  return Object.entries(values);
});

const selectedGameMediaEntries = computed(() => {
  return transcriptImageEntries(selectedGameTranscript.value);
});

const selectedGamePreviewImages = computed(() => {
  return selectedGameMediaEntries.value.filter((entry) => entry.status === "cached").map((entry) => entry.public_url);
});

const selectedGameCoverImage = computed(() => {
  return (
    selectedGame.value?.cover_image ||
    selectedGameTranscript.value.fields?.cover_image ||
    selectedGameTranscript.value.inferred?.cover_image ||
    ""
  );
});

const previewMediaEntries = computed(() => {
  return transcriptImageEntries(selectedTranscript.value);
});

const previewImages = computed(() => {
  return previewMediaEntries.value.filter((entry) => entry.status === "cached").map((entry) => entry.public_url);
});

const previewImageIndex = computed(() => {
  return previewSequence.value.findIndex((image) => image === previewImage.value);
});

const canStepPreview = computed(() => previewSequence.value.length > 1 && previewImageIndex.value >= 0);

const activeTasks = computed(() => tasks.value.filter(isTaskActive));

const completedTasks = computed(() => tasks.value.filter((task) => !isTaskActive(task)));

const recentImportTask = computed(() => {
  return lastImportTaskId.value ? tasks.value.find((task) => task.id === lastImportTaskId.value) ?? null : null;
});

const recentImportSourceItem = computed(() => {
  const result = recentImportTask.value?.result_json;
  const itemID = result?.item?.id;
  if (!itemID) {
    return null;
  }
  return sourceItems.value.find((item) => item.id === itemID) ?? result.item ?? null;
});

const recentImportTranscript = computed(() => {
  const result = recentImportTask.value?.result_json;
  return recentImportSourceItem.value?.parsed_json ?? result?.transcript ?? result?.item?.parsed_json ?? {};
});

const recentImportMediaEntries = computed(() => {
  return transcriptImageEntries(recentImportTranscript.value);
});

const recentImportImages = computed(() => {
  return recentImportMediaEntries.value.filter((entry) => entry.status === "cached").map((entry) => entry.public_url);
});

const selectedAdapter = computed(() => {
  return builtInAdapters.find((adapter) => adapter.id === selectedAdapterId.value) ?? builtInAdapters[0];
});

const selectedAdapterAuthProfile = computed(() => {
  return (
    authProfiles.value.find(
      (profile) => profile.adapter_id === selectedAdapter.value.id && profile.domain === selectedAdapter.value.authDomain
    ) ?? null
  );
});

const syncedAuthProfiles = computed(() => {
  return authProfiles.value.filter((profile) => profile.cookie_count > 0);
});

const globalAuthSummary = computed(() => {
  if (syncedAuthProfiles.value.length === 0) {
    return "No auth profiles";
  }
  return syncedAuthProfiles.value
    .map((profile) => `${adapterName(profile.adapter_id)}${profile.username ? ` as ${profile.username}` : ""}`)
    .join(", ");
});

const selectedTaskSourceItems = computed(() => {
  if (!selectedTask.value) {
    return [];
  }
  return collectSourceItemsForTask(selectedTask.value);
});

const selectedTaskSourceItem = computed(() => {
  if (selectedTaskSourceItems.value.length === 0) {
    return null;
  }
  if (selectedTaskSourceItemId.value) {
    const selected = selectedTaskSourceItems.value.find((item) => item.id === selectedTaskSourceItemId.value);
    if (selected) {
      return selected;
    }
  }
  return selectedTaskSourceItems.value[0];
});

const selectedTaskSourceTranscript = computed(() => {
  return selectedTaskSourceItem.value?.parsed_json ?? selectedTask.value?.result_json?.transcript ?? {};
});

const selectedTaskSourceImages = computed(() => {
  return transcriptImageList(selectedTaskSourceTranscript.value);
});

const selectedMatchedGame = computed(() => {
  const gameID = selectedItem.value?.matched_game_id;
  return gameID ? games.value.find((game) => game.id === gameID) ?? null : null;
});

const activeCoverImage = computed(() => {
  return (
    selectedMatchedGame.value?.cover_image ||
    selectedTranscript.value.fields?.cover_image ||
    selectedTranscript.value.inferred?.cover_image ||
    ""
  );
});

const availableMatchGames = computed(() => {
  const currentID = selectedItem.value?.matched_game_id;
  return games.value.filter((game) => game.id !== currentID);
});

const canApplyMatch = computed(() => {
  if (!selectedItem.value || matchGameId.value === "") {
    return false;
  }
  const nextGameID = Number(matchGameId.value);
  if (!Number.isFinite(nextGameID)) {
    return false;
  }
  return availableMatchGames.value.some((game) => game.id === nextGameID);
});

const selectedTaskSourceKeyValues = computed(() => {
  const values = selectedTaskSourceTranscript.value.key_values ?? {};
  return Object.entries(values);
});

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {})
    }
  });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(payload.error || response.statusText);
  }
  return payload as T;
}

async function loadAll() {
  loading.value = true;
  error.value = "";
  try {
    await Promise.all([loadLibrary(), loadTasks()]);
  } catch (err) {
    error.value = toMessage(err);
  } finally {
    loading.value = false;
  }
}

async function loadLibrary() {
  const [nextGames, nextSources, nextAuthProfiles, nextItems] = await Promise.all([
    api<Game[]>("/api/games"),
    api<Source[]>("/api/sources"),
    api<AuthProfile[]>("/api/auth-profiles"),
    api<SourceItem[]>("/api/source-items")
  ]);
  games.value = nextGames;
  sources.value = nextSources;
  authProfiles.value = nextAuthProfiles;
  sourceItems.value = nextItems;
  if (selectedGame.value) {
    const refreshedGame = nextGames.find((game) => game.id === selectedGame.value?.id) ?? null;
    selectedGame.value = refreshedGame;
    if (refreshedGame) {
      assignGameDraft(refreshedGame);
    } else if (gamePanelMode.value !== "new") {
      gamePanelMode.value = "detail";
    }
  }
  if (!selectedGame.value && nextGames.length > 0 && gamePanelMode.value !== "new") {
    selectGame(nextGames[0]);
  }
  if (nextGames.length === 0 && gamePanelMode.value !== "new") {
    selectedGame.value = null;
    gamePanelMode.value = "detail";
  }
  if (selectedItem.value) {
    const refreshedItem = nextItems.find((item) => item.id === selectedItem.value?.id) ?? null;
    selectedItem.value = refreshedItem;
    if (!refreshedItem) {
      matchGameId.value = "";
      matchEditing.value = false;
    }
  }
  if (!selectedItem.value && nextItems.length > 0) {
    selectedItem.value = nextItems[0];
    matchGameId.value = "";
    matchEditing.value = false;
  }
}

async function loadTasks() {
  const hadActiveTasks = activeTasks.value.length > 0;
  const selectedTaskWasActive = selectedTask.value ? isTaskActive(selectedTask.value) : false;
  const nextTasks = await api<Task[]>("/api/tasks");
  const nextActiveTasks = nextTasks.filter(isTaskActive);
  tasks.value = nextTasks;
  if (selectedTask.value) {
    selectedTask.value = nextTasks.find((task) => task.id === selectedTask.value?.id) ?? null;
  }
  if (!selectedTask.value && nextActiveTasks.length > 0) {
    selectedTask.value = nextActiveTasks[0];
  }
  if ((hadActiveTasks || selectedTaskWasActive) && nextActiveTasks.length === 0) {
    await loadLibrary();
  }
}

function newGame() {
  selectedGame.value = null;
  gamePanelMode.value = "new";
  Object.assign(gameDraft, {
    id: 0,
    title: "",
    aliases: "",
    description: "",
    current_version: "",
    cover_image: ""
  });
}

function assignGameDraft(game: Game) {
  selectedGame.value = game;
  Object.assign(gameDraft, {
    id: game.id,
    title: game.title,
    aliases: game.aliases.join(", "),
    description: game.description,
    current_version: game.current_version,
    cover_image: game.cover_image
  });
}

function selectGame(game: Game) {
  assignGameDraft(game);
  gamePanelMode.value = "detail";
}

function editGame(game: Game) {
  assignGameDraft(game);
  gamePanelMode.value = "edit";
}

function startGameEdit() {
  if (selectedGame.value) {
    editGame(selectedGame.value);
  }
}

function cancelGameForm() {
  if (selectedGame.value) {
    selectGame(selectedGame.value);
    return;
  }
  if (games.value.length > 0) {
    selectGame(games.value[0]);
    return;
  }
  gamePanelMode.value = "detail";
}

async function saveGame() {
  error.value = "";
  clearNotice();
  const payload = {
    title: gameDraft.title.trim(),
    aliases: gameDraft.aliases
      .split(",")
      .map((alias) => alias.trim())
      .filter(Boolean),
    description: gameDraft.description.trim(),
    current_version: gameDraft.current_version.trim(),
    cover_image: gameDraft.cover_image.trim()
  };
  try {
    const saved = gameDraft.id
      ? await api<Game>(`/api/games/${gameDraft.id}`, {
          method: "PATCH",
          body: JSON.stringify(payload)
        })
      : await api<Game>("/api/games", {
          method: "POST",
          body: JSON.stringify(payload)
    });
    showNotice("Saved");
    await loadAll();
    selectGame(saved);
  } catch (err) {
    error.value = toMessage(err);
  }
}

async function runImport() {
  error.value = "";
  clearNotice();
  loading.value = true;
  const payload = {
    url: importDraft.url.trim(),
    proxy_url: importDraft.proxy_url.trim(),
    create_game: importDraft.create_game
  };
  try {
    const result = await api<{ task: Task; duplicate?: boolean }>("/api/import/f95zone", {
      method: "POST",
      body: JSON.stringify(payload)
    });
    showNotice(result.duplicate ? "Import task already running" : "Import task started");
    lastImportTaskId.value = result.task.id;
    persistLastImportTaskId();
    selectedTask.value = result.task;
    await loadTasks();
  } catch (err) {
    error.value = toMessage(err);
  } finally {
    loading.value = false;
  }
}

async function createGameFromItem(item: SourceItem) {
  error.value = "";
  clearNotice();
  try {
    const result = await api<{ game: Game; item: SourceItem }>(
      `/api/source-items/${item.id}/create-game`,
      { method: "POST", body: "{}" }
    );
    showNotice("Game created");
    await loadAll();
    selectedItem.value = result.item;
    selectGame(result.game);
  } catch (err) {
    error.value = toMessage(err);
  }
}

async function matchItem(item: SourceItem) {
  if (!canApplyMatch.value) {
    return;
  }
  error.value = "";
  clearNotice();
  const gameId = Number(matchGameId.value);
  const wasMatched = Boolean(item.matched_game_id);
  try {
    const updated = await api<SourceItem>(`/api/source-items/${item.id}/match`, {
      method: "POST",
      body: JSON.stringify({ game_id: gameId })
    });
    selectedItem.value = updated;
    matchGameId.value = "";
    matchEditing.value = false;
    showNotice(wasMatched ? "Match updated" : "Matched");
    await loadAll();
  } catch (err) {
    error.value = toMessage(err);
  }
}

async function retryItemImages(item: SourceItem) {
  error.value = "";
  clearNotice();
  mediaRetrying.value = true;
  try {
    const result = await api<{ task: Task; duplicate?: boolean }>(`/api/source-items/${item.id}/retry-media`, {
      method: "POST",
      body: JSON.stringify({ proxy_url: importDraft.proxy_url.trim() })
    });
    selectedTask.value = result.task;
    showNotice(result.duplicate ? "Image retry already running" : "Image retry started");
    await loadTasks();
  } catch (err) {
    error.value = toMessage(err);
  } finally {
    mediaRetrying.value = false;
  }
}

async function retryItemImage(item: SourceItem, entry: MediaEntry) {
  if (!entry.original_url) {
    return;
  }
  error.value = "";
  clearNotice();
  retryingMediaURL.value = entry.original_url;
  try {
    const result = await api<{ task: Task; duplicate?: boolean }>(`/api/source-items/${item.id}/retry-media`, {
      method: "POST",
      body: JSON.stringify({
        proxy_url: importDraft.proxy_url.trim(),
        original_url: entry.original_url
      })
    });
    selectedTask.value = result.task;
    showNotice(result.duplicate ? "Image retry already running" : "Image retry started");
    await loadTasks();
  } catch (err) {
    error.value = toMessage(err);
  } finally {
    retryingMediaURL.value = "";
  }
}

async function setGameCoverFromImage(image: string) {
  if (!selectedMatchedGame.value) {
    return;
  }
  const game = selectedMatchedGame.value;
  error.value = "";
  clearNotice();
  try {
    const saved = await api<Game>(`/api/games/${game.id}`, {
      method: "PATCH",
      body: JSON.stringify({
        ...game,
        cover_image: image
      })
    });
    games.value = games.value.map((entry) => (entry.id === saved.id ? saved : entry));
    if (selectedGame.value?.id === saved.id) {
      selectedGame.value = saved;
      assignGameDraft(saved);
    }
    showNotice("Cover updated");
  } catch (err) {
    error.value = toMessage(err);
  }
}

function openMatchedGame() {
  if (!selectedMatchedGame.value) {
    return;
  }
  selectGame(selectedMatchedGame.value);
  view.value = "games";
}

function startMatchEdit() {
  matchEditing.value = true;
  matchGameId.value = "";
}

function cancelMatchEdit() {
  matchEditing.value = false;
  matchGameId.value = "";
}

function openImagePreview(image: string, sequence: string[] = previewImages.value) {
  previewImage.value = image;
  previewSequence.value = sequence.length ? [...sequence] : [image];
}

function closeImagePreview() {
  previewImage.value = "";
  previewSequence.value = [];
}

function stepPreview(direction: 1 | -1) {
  if (!canStepPreview.value) {
    return;
  }
  const index = previewImageIndex.value;
  const length = previewSequence.value.length;
  previewImage.value = previewSequence.value[(index + direction + length) % length];
}

function handlePreviewKeydown(event: KeyboardEvent) {
  if (!previewImage.value) {
    return;
  }
  if (event.key === "Escape") {
    event.preventDefault();
    closeImagePreview();
  } else if (event.key === "ArrowLeft") {
    event.preventDefault();
    stepPreview(-1);
  } else if (event.key === "ArrowRight") {
    event.preventDefault();
    stepPreview(1);
  }
}

function requestDeleteGame(game: Game) {
  pendingDelete.value = {
    kind: "game",
    id: game.id,
    title: game.title,
    firstMessage: "Delete this game record?",
    secondMessage: "This detaches matched items and removes unreferenced game attachments."
  };
}

function requestDeleteSourceItem(item: SourceItem) {
  pendingDelete.value = {
    kind: "item",
    id: item.id,
    title: item.title,
    firstMessage: "Delete this source item?",
    secondMessage: "This removes the raw HTML and unreferenced attachments from disk."
  };
}

function cancelDelete() {
  if (deleting.value) {
    return;
  }
  pendingDelete.value = null;
}

async function confirmDelete() {
  if (!pendingDelete.value) {
    return;
  }

  const target = pendingDelete.value;
  error.value = "";
  clearNotice();
  deleting.value = true;
  try {
    if (target.kind === "game") {
      await api<{ deleted: boolean }>(`/api/games/${target.id}`, { method: "DELETE" });
      showNotice("Game deleted");
      selectedGame.value = null;
      gamePanelMode.value = "detail";
    } else {
      await api<{ deleted: boolean }>(`/api/source-items/${target.id}`, { method: "DELETE" });
      showNotice("Item deleted");
      selectedItem.value = null;
      matchGameId.value = "";
    }
    pendingDelete.value = null;
    await loadAll();
    if (target.kind === "item" && sourceItems.value.length > 0) {
      selectItem(sourceItems.value[0]);
    }
  } catch (err) {
    error.value = toMessage(err);
  } finally {
    deleting.value = false;
  }
}

function selectItem(item: SourceItem) {
  selectedItem.value = item;
  matchGameId.value = "";
  matchEditing.value = false;
}

function selectTask(task: Task) {
  selectedTask.value = task;
  selectedTaskSourceItemId.value = null;
  if (!isTaskActive(task)) {
    taskHistoryExpanded.value = true;
  }
}

function selectTaskSourceItem(item: SourceItem) {
  selectedTaskSourceItemId.value = item.id;
}

function openTask(task: Task) {
  selectedTask.value = task;
  view.value = "tasks";
}

function openTaskResult(task: Task) {
  if (task.result_json?.game) {
    selectGame(task.result_json.game);
    view.value = "games";
    return;
  }
  if (task.result_json?.item) {
    selectItem(task.result_json.item);
    view.value = "items";
  }
}

function isTaskActive(task: Task) {
  return task.status === "queued" || task.status === "running";
}

function taskProgress(task: Task) {
  if (task.progress_total <= 0) {
    return task.status === "succeeded" ? 100 : 0;
  }
  return Math.round((task.progress_current / task.progress_total) * 100);
}

function taskResultTitle(task: Task) {
  return task.result_json?.game?.title || task.result_json?.item?.title || "";
}

function collectSourceItemsForTask(task: Task) {
  const resultItem = task.result_json?.item;
  const gameId = task.result_json?.game?.id ?? resultItem?.matched_game_id;
  const seen = new Set<number>();
  const items: SourceItem[] = [];

  const addItem = (item: SourceItem | undefined) => {
    if (!item || seen.has(item.id)) {
      return;
    }
    seen.add(item.id);
    items.push(item);
  };

  if (resultItem) {
    addItem(sourceItems.value.find((item) => item.id === resultItem.id) ?? resultItem);
  }
  if (gameId) {
    sourceItems.value
      .filter((item) => item.matched_game_id === gameId)
      .forEach(addItem);
  }
  if (resultItem?.source_type && resultItem.external_id) {
    sourceItems.value
      .filter((item) => item.source_type === resultItem.source_type && item.external_id === resultItem.external_id)
      .forEach(addItem);
  }
  return items.sort((left, right) => {
    const leftTime = Date.parse(left.updated_at || left.fetched_at || left.created_at);
    const rightTime = Date.parse(right.updated_at || right.fetched_at || right.created_at);
    return rightTime - leftTime;
  });
}

function sourceItemLabel(item: SourceItem) {
  const source = item.source_id ? sources.value.find((entry) => entry.id === item.source_id) : null;
  return source?.name || item.source_type || "Unknown adapter";
}

function transcriptImageList(transcript?: Transcript | null) {
  const mediaItems = transcript?.media_items ?? [];
  if (mediaItems.length) {
    return mediaItems
      .filter((item) => item.status === "cached" && item.public_url)
      .map((item) => item.public_url as string);
  }
  const images: string[] = [];
  const add = (value?: string) => {
    if (value && !images.includes(value)) {
      images.push(value);
    }
  };
  add(transcript?.fields?.cover_image);
  transcript?.fields?.screenshots?.forEach(add);
  transcript?.images?.forEach(add);
  return images;
}

function transcriptImageEntries(transcript?: Transcript | null): MediaEntry[] {
  const mediaItems = transcript?.media_items ?? [];
  if (mediaItems.length) {
    return mediaItems.map((item, index) => {
      const position = Number.isFinite(item.position) ? item.position : index;
      const publicURL = item.public_url ?? "";
      const originalURL = item.original_url || publicURL;
      return {
        key: originalURL || publicURL || `media-${position}`,
        position,
        role: item.role || (position === 0 ? "cover" : "screenshot"),
        status: item.status === "cached" && publicURL ? "cached" : "failed",
        original_url: originalURL,
        public_url: publicURL,
        error: item.error || ""
      };
    });
  }
  return transcriptImageList(transcript).map((image, index) => ({
    key: image,
    position: index,
    role: index === 0 ? "cover" : "screenshot",
    status: "cached",
    original_url: image,
    public_url: image,
    error: ""
  }));
}

function formatList(values?: string[]) {
  const filtered = values?.filter((value) => value.trim() !== "") ?? [];
  return filtered.length ? filtered.join(", ") : "Unknown";
}

function formatBoolean(value?: boolean | null) {
  if (typeof value !== "boolean") {
    return "Unknown";
  }
  return value ? "Yes" : "No";
}

function displayTranscriptSections(transcript?: Transcript | null) {
  const hidden = new Set(["overview", "story", "description", "changelog", "change log", "download", "downloads"]);
  return (transcript?.sections ?? []).filter((section) => !hidden.has(section.heading.trim().toLowerCase()));
}

function formatTimestamp(value?: string, fallback = "Never") {
  if (!value) {
    return fallback;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function adapterName(adapterID: string) {
  return builtInAdapters.find((adapter) => adapter.id === adapterID)?.name ?? adapterID;
}

function formatAuthExpiry(profile?: AuthProfile | null) {
  if (!profile) {
    return "Not synced";
  }
  if (profile.cookie_expires_at) {
    return formatTimestamp(profile.cookie_expires_at, "Unknown");
  }
  if (profile.cookies?.length && profile.cookies.every((cookie) => cookie.session || !cookie.expires_at)) {
    return "Browser session";
  }
  return "Unknown";
}

function formatCookieExpiry(cookie: AuthCookie) {
  if (cookie.expires_at) {
    return formatTimestamp(cookie.expires_at, "Unknown");
  }
  return cookie.session ? "Session" : "Unknown";
}

function formatCookieFlags(cookie: AuthCookie) {
  const flags = [];
  if (cookie.secure) flags.push("Secure");
  if (cookie.http_only) flags.push("HttpOnly");
  if (cookie.session) flags.push("Session");
  return flags.join(" · ");
}

function hasAdapterAuth(profile: AuthProfile | null | undefined, adapter: BuiltInAdapter) {
  if (!profile) {
    return false;
  }
  const requiredCookies: readonly string[] = adapter.authCookieNames ?? [];
  if (requiredCookies.length === 0) {
    return profile.cookie_count > 0;
  }
  const names = new Set((profile.cookies ?? []).map((cookie) => cookie.name));
  return requiredCookies.some((name) => names.has(name));
}

function missingAdapterAuthCookies(profile: AuthProfile | null | undefined, adapter: BuiltInAdapter) {
  const requiredCookies: readonly string[] = adapter.authCookieNames ?? [];
  if (!profile || requiredCookies.length === 0) {
    return [];
  }
  const names = new Set((profile.cookies ?? []).map((cookie) => cookie.name));
  return requiredCookies.filter((name) => !names.has(name));
}

function adapterAuthState(profile: AuthProfile | null | undefined, adapter: BuiltInAdapter) {
  if (!profile) {
    return "missing";
  }
  return hasAdapterAuth(profile, adapter) ? "authorized" : "incomplete";
}

function adapterAuthStatusClass(profile: AuthProfile | null | undefined, adapter: BuiltInAdapter) {
  const state = adapterAuthState(profile, adapter);
  if (state === "authorized") return "succeeded";
  if (state === "incomplete") return "warning";
  return "failed";
}

function adapterAuthStatusLabel(profile: AuthProfile | null | undefined, adapter: BuiltInAdapter) {
  return adapterAuthState(profile, adapter);
}

function clearImportState() {
  Object.assign(importDraft, importDraftDefaults);
  localStorage.removeItem(importDraftStorageKey);
  lastImportTaskId.value = null;
  persistLastImportTaskId();
  showNotice("Import cleared");
}

function showNotice(message: string) {
  notice.value = message;
  if (noticeTimer !== undefined) {
    window.clearTimeout(noticeTimer);
  }
  noticeTimer = window.setTimeout(() => {
    if (notice.value === message) {
      notice.value = "";
    }
  }, 2600);
}

function clearNotice() {
  notice.value = "";
  if (noticeTimer !== undefined) {
    window.clearTimeout(noticeTimer);
    noticeTimer = undefined;
  }
}

function restoreImportState() {
  const savedDraft = localStorage.getItem(importDraftStorageKey);
  if (savedDraft) {
    try {
      const draft = JSON.parse(savedDraft) as Partial<typeof importDraftDefaults>;
      Object.assign(importDraft, {
        url: typeof draft.url === "string" ? draft.url : "",
        proxy_url: typeof draft.proxy_url === "string" ? draft.proxy_url : "",
        create_game: typeof draft.create_game === "boolean" ? draft.create_game : true
      });
    } catch {
      localStorage.removeItem(importDraftStorageKey);
    }
  }

  const savedTaskId = Number(localStorage.getItem(lastImportTaskStorageKey) ?? "");
  if (Number.isFinite(savedTaskId) && savedTaskId > 0) {
    lastImportTaskId.value = savedTaskId;
  }
}

function persistImportDraft() {
  try {
    if (isImportDraftEmpty()) {
      localStorage.removeItem(importDraftStorageKey);
      return;
    }
    localStorage.setItem(importDraftStorageKey, JSON.stringify(importDraft));
  } catch (err) {
    error.value = `Could not save import draft: ${toMessage(err)}`;
  }
}

function isImportDraftEmpty() {
  return (
    importDraft.url === importDraftDefaults.url &&
    importDraft.proxy_url === importDraftDefaults.proxy_url &&
    importDraft.create_game === importDraftDefaults.create_game
  );
}

function persistLastImportTaskId() {
  if (lastImportTaskId.value) {
    localStorage.setItem(lastImportTaskStorageKey, lastImportTaskId.value.toString());
  } else {
    localStorage.removeItem(lastImportTaskStorageKey);
  }
}

function toMessage(err: unknown) {
  return err instanceof Error ? err.message : String(err);
}

let taskPoll: number | undefined;

watch(importDraft, persistImportDraft, { deep: true });

watch(error, (message) => {
  if (errorTimer !== undefined) {
    window.clearTimeout(errorTimer);
    errorTimer = undefined;
  }
  if (!message) {
    return;
  }
  errorTimer = window.setTimeout(() => {
    if (error.value === message) {
      error.value = "";
    }
  }, 4200);
});

onMounted(() => {
  restoreImportState();
  void loadAll();
  window.addEventListener("keydown", handlePreviewKeydown);
  taskPoll = window.setInterval(() => {
    void loadTasks().catch((err) => {
      error.value = toMessage(err);
    });
  }, 1500);
});

onUnmounted(() => {
  if (taskPoll !== undefined) {
    window.clearInterval(taskPoll);
  }
  if (noticeTimer !== undefined) {
    window.clearTimeout(noticeTimer);
  }
  if (errorTimer !== undefined) {
    window.clearTimeout(errorTimer);
  }
  window.removeEventListener("keydown", handlePreviewKeydown);
});
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <Icon name="gamepad" :size="22" />
        <span>Ludex</span>
      </div>

      <nav class="nav-stack">
        <button :class="{ active: view === 'games' }" title="Games" @click="view = 'games'">
          <Icon name="database" :size="18" />
          <span>Games</span>
        </button>
        <button :class="{ active: view === 'adapters' }" title="Adapters" @click="view = 'adapters'">
          <Icon name="globe" :size="18" />
          <span>Adapters</span>
        </button>
        <button :class="{ active: view === 'import' }" title="Import" @click="view = 'import'">
          <Icon name="download" :size="18" />
          <span>Import</span>
        </button>
        <button :class="{ active: view === 'items' }" title="Items" @click="view = 'items'">
          <Icon name="file-search" :size="18" />
          <span>Items</span>
        </button>
        <button :class="{ active: view === 'tasks' }" title="Tasks" @click="view = 'tasks'">
          <Icon name="activity" :size="18" />
          <span>Tasks</span>
          <small v-if="activeTasks.length" class="nav-badge">{{ activeTasks.length }}</small>
        </button>
      </nav>
    </aside>

    <main class="main">
      <header class="topbar">
        <div class="searchbox">
          <Icon name="search" :size="17" />
          <input v-model="query" type="search" placeholder="Search games" />
        </div>
        <button class="icon-button" title="Refresh" @click="loadAll">
          <Icon name="refresh" :size="18" />
        </button>
      </header>

      <section v-if="view === 'games'" class="workspace two-column">
        <div class="list-pane">
          <div class="pane-title">
            <h1>Games</h1>
            <button class="icon-button" title="New game" @click="newGame">
              <Icon name="plus" :size="18" />
            </button>
          </div>
          <div
            v-for="game in filteredGames"
            :key="game.id"
            :class="{ selected: selectedGame?.id === game.id }"
            class="row-entry"
          >
            <button
              class="row-button"
              :class="{ selected: selectedGame?.id === game.id }"
              @click="selectGame(game)"
            >
              <img v-if="game.cover_image" :src="game.cover_image" alt="" />
              <span class="row-main">
                <strong>{{ game.title }}</strong>
                <small>{{ game.current_version || "No version" }}</small>
              </span>
            </button>
            <button class="row-delete" title="Delete game" @click.stop="requestDeleteGame(game)">
              <Icon name="trash" :size="16" />
            </button>
          </div>
          <div v-if="filteredGames.length === 0" class="empty-state">
            <Icon name="database" :size="20" />
            <strong>No games</strong>
          </div>
        </div>

        <div v-if="gamePanelMode === 'new' || gamePanelMode === 'edit'" class="detail-pane">
          <div class="pane-title">
            <h2>{{ gamePanelMode === "edit" ? "Edit Game" : "New Game" }}</h2>
            <div class="button-row">
              <button class="secondary" @click="cancelGameForm">
                <Icon name="x" :size="17" />
                <span>Cancel</span>
              </button>
              <button class="primary" @click="saveGame">
                <Icon name="save" :size="17" />
                <span>Save</span>
              </button>
            </div>
          </div>
          <label>
            <span>Title</span>
            <input v-model="gameDraft.title" type="text" />
          </label>
          <label>
            <span>Aliases</span>
            <input v-model="gameDraft.aliases" type="text" />
          </label>
          <label>
            <span>Version</span>
            <input v-model="gameDraft.current_version" type="text" />
          </label>
          <label>
            <span>Cover image</span>
            <input v-model="gameDraft.cover_image" type="text" />
          </label>
          <label class="wide">
            <span>Description</span>
            <textarea v-model="gameDraft.description" rows="12"></textarea>
          </label>
        </div>

        <div v-else-if="selectedGame" class="detail-pane transcript-pane game-detail-pane">
          <div class="pane-title">
            <h2>{{ selectedGame.title }}</h2>
            <div class="button-row">
              <button class="secondary" @click="startGameEdit">
                <Icon name="pencil" :size="17" />
                <span>Edit</span>
              </button>
              <button class="primary" @click="newGame">
                <Icon name="plus" :size="17" />
                <span>New</span>
              </button>
            </div>
          </div>

          <div v-if="selectedGameMediaEntries.length" class="image-strip">
            <div
              v-for="entry in selectedGameMediaEntries"
              :key="entry.key"
              class="image-card"
              :class="{ failed: entry.status === 'failed' }"
            >
              <button
                v-if="entry.status === 'cached'"
                class="image-thumb"
                type="button"
                @click="openImagePreview(entry.public_url, selectedGamePreviewImages)"
              >
                <img :src="entry.public_url" alt="" draggable="false" @dragstart.prevent />
                <span v-if="entry.public_url === selectedGameCoverImage" class="image-badge">Cover</span>
              </button>
              <div v-else class="image-thumb image-placeholder">
                <Icon name="image-off" :size="20" />
                <strong>{{ entry.role === "cover" ? "Cover failed" : "Image failed" }}</strong>
                <small>Open item to retry</small>
              </div>
            </div>
          </div>
          <div v-else-if="selectedGame.cover_image" class="image-strip compact">
            <button class="image-thumb" type="button" @click="openImagePreview(selectedGame.cover_image, [selectedGame.cover_image])">
              <img :src="selectedGame.cover_image" alt="" draggable="false" @dragstart.prevent />
              <span class="image-badge">Cover</span>
            </button>
          </div>

          <div class="meta-grid">
            <div>
              <span>Game</span>
              <strong>{{ selectedGame.title }}</strong>
            </div>
            <div>
              <span>Version</span>
              <strong>{{ selectedGame.current_version || selectedGameTranscript.fields?.version || selectedGameTranscript.inferred?.version || "Unknown" }}</strong>
            </div>
            <div>
              <span>Developer</span>
              <strong>{{ selectedGameTranscript.fields?.developer || selectedGameTranscript.inferred?.developer || "Unknown" }}</strong>
            </div>
            <div>
              <span>Engine</span>
              <strong>{{ selectedGameTranscript.fields?.engine || "Unknown" }}</strong>
            </div>
            <div>
              <span>Prefixes</span>
              <strong>{{ formatList(selectedGameTranscript.fields?.prefixes) }}</strong>
            </div>
            <div>
              <span>Censored</span>
              <strong>{{ formatBoolean(selectedGameTranscript.fields?.censored) }}</strong>
            </div>
            <div>
              <span>Updated</span>
              <strong>{{ selectedGameTranscript.fields?.thread_updated || "Unknown" }}</strong>
            </div>
            <div>
              <span>Released</span>
              <strong>{{ selectedGameTranscript.fields?.release_date || "Unknown" }}</strong>
            </div>
            <div>
              <span>OS</span>
              <strong>{{ formatList(selectedGameTranscript.fields?.operating_systems) }}</strong>
            </div>
            <div>
              <span>Language</span>
              <strong>{{ formatList(selectedGameTranscript.fields?.languages) }}</strong>
            </div>
            <div>
              <span>Genre</span>
              <strong>{{ formatList(selectedGameTranscript.fields?.genres) }}</strong>
            </div>
            <div>
              <span>Adapter</span>
              <strong>{{ selectedGameSourceItem ? sourceItemLabel(selectedGameSourceItem) : "Manual" }}</strong>
            </div>
            <div class="wide">
              <span>Aliases</span>
              <strong>{{ selectedGame.aliases.length ? selectedGame.aliases.join(", ") : "None" }}</strong>
            </div>
            <div class="wide">
              <span>Source URL</span>
              <strong>{{ selectedGameSourceItem?.raw_url || "None" }}</strong>
            </div>
          </div>

          <div v-if="!selectedGameSourceItem" class="warning-box">
            <p>No adapter transcript is matched to this game yet.</p>
          </div>

          <section
            v-if="selectedGame.description || selectedGameTranscript.fields?.description || selectedGameTranscript.inferred?.description"
            class="transcript-section"
          >
            <h3>Overview</h3>
            <p>{{ selectedGame.description || selectedGameTranscript.fields?.description || selectedGameTranscript.inferred?.description }}</p>
          </section>

          <section v-if="selectedGameTranscript.fields?.developer_links?.length" class="transcript-section">
            <h3>Developer Links</h3>
            <div class="link-list">
              <a
                v-for="link in selectedGameTranscript.fields.developer_links"
                :key="`${link.name}-${link.url}`"
                :href="link.url"
                target="_blank"
              >
                {{ link.name }}
              </a>
            </div>
          </section>

          <section v-if="selectedGameTranscript.fields?.download_groups?.length" class="transcript-section">
            <h3>Downloads</h3>
            <dl>
              <template v-for="group in selectedGameTranscript.fields.download_groups" :key="group.platform">
                <dt>
                  {{ group.platform }}
                  <small v-if="group.note">{{ group.note }}</small>
                </dt>
                <dd>
                  <div v-if="group.links?.length" class="download-list">
                    <template v-for="(link, index) in group.links" :key="`${group.platform}-${link.name}-${index}`">
                      <a v-if="link.url" class="download-link" :href="link.url" target="_blank">
                        <strong>{{ link.name }}</strong>
                        <small>{{ link.url }}</small>
                      </a>
                      <span v-else class="download-link missing-url">
                        <strong>{{ link.name }}</strong>
                        <small>No URL captured</small>
                      </span>
                    </template>
                  </div>
                  <span v-else>Unknown</span>
                </dd>
              </template>
            </dl>
          </section>

          <section v-if="selectedGameTranscript.fields?.changelog" class="transcript-section">
            <h3>Changelog</h3>
            <div
              v-if="selectedGameTranscript.fields?.changelog_html"
              class="rich-html"
              v-html="selectedGameTranscript.fields.changelog_html"
            ></div>
            <p v-else>{{ selectedGameTranscript.fields.changelog }}</p>
          </section>

          <section v-if="selectedGameKeyValues.length" class="transcript-section">
            <h3>Raw Fields</h3>
            <dl>
              <template v-for="[key, values] in selectedGameKeyValues" :key="key">
                <dt>{{ key }}</dt>
                <dd>{{ values.join(", ") }}</dd>
              </template>
            </dl>
          </section>

          <section
            v-for="section in displayTranscriptSections(selectedGameTranscript)"
            :key="section.heading"
            class="transcript-section"
          >
            <h3>{{ section.heading }}</h3>
            <p>{{ section.body }}</p>
          </section>

          <div class="result-actions">
            <button v-if="selectedGameSourceItem" class="secondary" @click="selectItem(selectedGameSourceItem); view = 'items'">
              <Icon name="file-search" :size="17" />
              <span>Open item</span>
            </button>
            <a v-if="selectedGameSourceItem?.raw_url" class="open-link" :href="selectedGameSourceItem.raw_url" target="_blank">
              <Icon name="eye" :size="17" />
              <span>Open source</span>
            </a>
          </div>
        </div>

        <div v-else class="detail-pane">
          <div class="empty-detail">
            <Icon name="database" :size="24" />
            <strong>No game selected</strong>
            <button class="primary" @click="newGame">
              <Icon name="plus" :size="17" />
              <span>New game</span>
            </button>
          </div>
        </div>
      </section>

      <section v-else-if="view === 'adapters'" class="workspace adapters-workspace">
        <div class="detail-pane bridge-pane">
          <div class="pane-title">
            <h1>{{ browserBridge.name }}</h1>
            <div class="button-row">
              <a class="primary" :href="browserBridge.extensionPackage" target="_blank" rel="noreferrer">
                <Icon name="download" :size="17" />
                <span>Extension ZIP</span>
              </a>
            </div>
          </div>
          <dl class="meta-grid">
            <div>
              <span>Mode</span>
              <strong>{{ browserBridge.mode }}</strong>
            </div>
            <div>
              <span>Coverage</span>
              <strong>{{ browserBridge.coverage }}</strong>
            </div>
            <div>
              <span>Install</span>
              <strong>{{ browserBridge.install }}</strong>
            </div>
            <div>
              <span>Action</span>
              <strong>{{ browserBridge.action }}</strong>
            </div>
            <div>
              <span>Auth profiles</span>
              <strong>{{ syncedAuthProfiles.length }} synced</strong>
            </div>
            <div>
              <span>Extension</span>
              <strong>{{ browserBridge.extension }}</strong>
            </div>
            <div>
              <span>Extension package</span>
              <strong>{{ browserBridge.extensionPackage }}</strong>
            </div>
            <div>
              <span>Load unpacked folder</span>
              <strong>{{ browserBridge.extensionSource }}</strong>
            </div>
            <div class="wide">
              <span>Install note</span>
              <strong>{{ browserBridge.limitation }}</strong>
            </div>
            <div class="wide">
              <span>Global status</span>
              <strong>{{ globalAuthSummary }}</strong>
            </div>
          </dl>
        </div>

        <div class="adapters-layout">
          <div class="list-pane">
            <div class="pane-title">
              <h1>Adapters</h1>
            </div>
            <button
              v-for="adapter in builtInAdapters"
              :key="adapter.id"
              class="row-button"
              :class="{ selected: selectedAdapterId === adapter.id }"
              @click="selectedAdapterId = adapter.id"
            >
              <span class="source-dot"></span>
              <span class="row-main">
                <strong>{{ adapter.name }}</strong>
                <small>{{ adapter.kind }} · {{ adapter.status }}</small>
              </span>
              <Icon name="cable" :size="16" />
            </button>
          </div>

          <div class="detail-stack">
            <div class="detail-pane">
            <div class="pane-title">
              <h2>{{ selectedAdapter.name }}</h2>
              <span class="status-pill succeeded">{{ selectedAdapter.status }}</span>
            </div>
            <dl class="meta-grid">
              <div>
                <span>Adapter key</span>
                <strong>{{ selectedAdapter.id }}</strong>
              </div>
              <div>
                <span>Input</span>
                <strong>{{ selectedAdapter.input }}</strong>
              </div>
              <div>
                <span>Dedupe</span>
                <strong>{{ selectedAdapter.dedupe }}</strong>
              </div>
              <div>
                <span>Attachments</span>
                <strong>{{ selectedAdapter.attachments }}</strong>
              </div>
              <div>
                <span>Endpoint</span>
                <strong>{{ selectedAdapter.endpoint }}</strong>
              </div>
              <div class="wide">
                <span>Without Browser Bridge</span>
                <strong>{{ selectedAdapter.withoutBridge }}</strong>
              </div>
            </dl>
          </div>

            <div class="detail-pane auth-pane">
              <div class="pane-title">
                <h2>{{ selectedAdapter.name }} Cookies</h2>
                <span class="status-pill" :class="adapterAuthStatusClass(selectedAdapterAuthProfile, selectedAdapter)">
                  {{ adapterAuthStatusLabel(selectedAdapterAuthProfile, selectedAdapter) }}
                </span>
              </div>

              <template v-if="selectedAdapterAuthProfile">
                <dl class="meta-grid">
                  <div>
                    <span>Account</span>
                    <strong>{{ selectedAdapterAuthProfile.username || "Unknown" }}</strong>
                  </div>
                  <div>
                    <span>Cookies</span>
                    <strong>{{ selectedAdapterAuthProfile.cookie_count }} saved</strong>
                  </div>
                  <div>
                    <span>Expires</span>
                    <strong>{{ formatAuthExpiry(selectedAdapterAuthProfile) }}</strong>
                  </div>
                  <div>
                    <span>Imported</span>
                    <strong>{{ formatTimestamp(selectedAdapterAuthProfile.imported_at, "Not synced") }}</strong>
                  </div>
                  <div>
                    <span>Last used</span>
                    <strong>{{ formatTimestamp(selectedAdapterAuthProfile.last_used_at) }}</strong>
                  </div>
                  <div>
                    <span>User-Agent</span>
                    <strong>{{ selectedAdapterAuthProfile.user_agent ? "Saved" : "Not saved" }}</strong>
                  </div>
                  <div class="wide">
                    <span>Source page</span>
                    <strong>{{ selectedAdapterAuthProfile.source_url || "Unknown" }}</strong>
                  </div>
                </dl>
                <div v-if="selectedAdapterAuthProfile.cookies?.length" class="cookie-list">
                  <div v-for="cookie in selectedAdapterAuthProfile.cookies" :key="cookie.name" class="cookie-chip">
                    <strong>{{ cookie.name }}</strong>
                    <span>{{ formatCookieExpiry(cookie) }}</span>
                    <small v-if="formatCookieFlags(cookie)">{{ formatCookieFlags(cookie) }}</small>
                  </div>
                </div>
                <div v-if="!hasAdapterAuth(selectedAdapterAuthProfile, selectedAdapter)" class="auth-guide">
                  <strong>Auth profile looks incomplete</strong>
                  <p>
                    Ludex can see the account name and basic cookies, but it is still missing
                    {{ missingAdapterAuthCookies(selectedAdapterAuthProfile, selectedAdapter).join(", ") || "the login cookie" }}.
                    The saved cookies can identify the page session, but may not unlock login-only download links.
                  </p>
                  <p>Download the Ludex extension ZIP, load it unpacked in Chrome/Edge developer mode, then use the extension popup to sync auth.</p>
                </div>
              </template>

              <div v-else class="auth-guide">
                <strong>No F95zone auth profile yet</strong>
                <p>Download the global Browser Bridge extension, load it unpacked in Chrome/Edge developer mode, open F95zone while logged in, then run the extension popup action.</p>
                <a class="open-link" :href="browserBridge.extensionPackage" target="_blank" rel="noreferrer">
                  <Icon name="download" :size="17" />
                  <span>Download extension</span>
                </a>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section v-else-if="view === 'import'" class="workspace import-grid">
        <div class="detail-pane">
          <div class="pane-title">
            <h1>F95zone Import</h1>
            <div class="button-row">
              <button class="secondary" title="Clear import state" @click="clearImportState">
                <Icon name="x" :size="17" />
                <span>Clear</span>
              </button>
              <button class="primary" @click="runImport">
                <Icon name="download" :size="17" />
                <span>Import</span>
              </button>
            </div>
          </div>
          <div v-if="recentImportTask" class="inline-task-panel" :class="recentImportTask.status">
            <div class="inline-task-head">
              <span class="task-state" :class="recentImportTask.status"></span>
              <span class="row-main">
                <strong>{{ recentImportTask.title }}</strong>
                <small>{{ recentImportTask.message || recentImportTask.kind }}</small>
              </span>
              <span class="status-pill" :class="recentImportTask.status">{{ recentImportTask.status }}</span>
            </div>
            <div class="task-progress">
              <div class="progress-track">
                <span :style="{ width: `${taskProgress(recentImportTask)}%` }"></span>
              </div>
              <strong>{{ taskProgress(recentImportTask) }}%</strong>
            </div>
            <div v-if="recentImportMediaEntries.length" class="image-strip compact">
              <div
                v-for="entry in recentImportMediaEntries"
                :key="entry.key"
                class="image-card"
                :class="{ failed: entry.status === 'failed' }"
              >
                <button
                  v-if="entry.status === 'cached'"
                  class="image-thumb"
                  type="button"
                  @click="openImagePreview(entry.public_url, recentImportImages)"
                >
                  <img :src="entry.public_url" alt="" draggable="false" @dragstart.prevent />
                </button>
                <button
                  v-else
                  class="image-thumb image-placeholder"
                  type="button"
                  :disabled="!recentImportSourceItem || retryingMediaURL === entry.original_url"
                  @click="recentImportSourceItem && retryItemImage(recentImportSourceItem, entry)"
                >
                  <Icon name="refresh" :size="18" />
                  <span class="compact-placeholder-label">{{ retryingMediaURL === entry.original_url ? "Retrying" : "Retry" }}</span>
                </button>
              </div>
            </div>
            <div class="result-actions">
              <strong v-if="taskResultTitle(recentImportTask)" class="result-title">
                {{ taskResultTitle(recentImportTask) }}
              </strong>
              <button
                v-if="taskResultTitle(recentImportTask)"
                class="secondary"
                @click="openTaskResult(recentImportTask)"
              >
                <Icon name="eye" :size="17" />
                <span>Open result</span>
              </button>
              <button class="secondary" @click="openTask(recentImportTask)">
                <Icon name="activity" :size="17" />
                <span>Task</span>
              </button>
            </div>
          </div>
          <label>
            <span>Adapter</span>
            <select :value="selectedAdapter.id" disabled>
              <option value="f95zone">F95zone</option>
            </select>
          </label>
          <label class="wide">
            <span>Thread URL</span>
            <input v-model="importDraft.url" type="url" />
          </label>
          <label class="wide">
            <span>Proxy URL</span>
            <input v-model="importDraft.proxy_url" type="text" placeholder="socks5://127.0.0.1:7890" />
          </label>
          <label class="toggle-row">
            <input v-model="importDraft.create_game" type="checkbox" />
            <span>Create game</span>
          </label>
        </div>
      </section>

      <section v-else-if="view === 'tasks'" class="workspace two-column tasks-layout">
        <div class="list-pane">
          <div class="pane-title">
            <h1>Tasks</h1>
            <button class="icon-button" title="Refresh tasks" @click="loadTasks">
              <Icon name="refresh" :size="18" />
            </button>
          </div>

          <div class="task-list-section">
            <div class="task-section-title">
              <span>Active</span>
              <small>{{ activeTasks.length }}</small>
            </div>
            <button
              v-for="task in activeTasks"
              :key="task.id"
              class="row-button task-row"
              :class="{ selected: selectedTask?.id === task.id }"
              @click="selectTask(task)"
            >
              <span class="task-state" :class="task.status"></span>
              <span class="row-main">
                <strong>{{ task.title }}</strong>
                <small>{{ task.status }} · {{ task.message || task.kind }}</small>
              </span>
            </button>
            <div v-if="activeTasks.length === 0" class="task-empty-row">
              <Icon name="activity" :size="18" />
              <strong>No active tasks</strong>
            </div>
          </div>

          <div class="task-list-section task-history-section">
            <button
              class="task-history-toggle"
              :disabled="completedTasks.length === 0"
              @click="taskHistoryExpanded = !taskHistoryExpanded"
            >
              <Icon name="arrow-right" :class="{ expanded: taskHistoryExpanded }" :size="16" />
              <span>History</span>
              <small>{{ completedTasks.length }}</small>
            </button>
            <template v-if="taskHistoryExpanded">
              <button
                v-for="task in completedTasks"
                :key="task.id"
                class="row-button task-row completed"
                :class="{ selected: selectedTask?.id === task.id }"
                @click="selectTask(task)"
              >
                <span class="task-state" :class="task.status"></span>
                <span class="row-main">
                  <strong>{{ task.title }}</strong>
                  <small>{{ task.status }} · {{ task.message || task.kind }}</small>
                </span>
              </button>
            </template>
          </div>
        </div>

        <div v-if="selectedTask" class="detail-pane task-detail">
          <div class="pane-title">
            <h2>{{ selectedTask.title }}</h2>
            <span class="status-pill" :class="selectedTask.status">{{ selectedTask.status }}</span>
          </div>

          <div class="task-progress">
            <div class="progress-track">
              <span :style="{ width: `${taskProgress(selectedTask)}%` }"></span>
            </div>
            <strong>{{ taskProgress(selectedTask) }}%</strong>
          </div>

          <div class="meta-grid">
            <div>
              <span>Kind</span>
              <strong>{{ selectedTask.kind }}</strong>
            </div>
            <div>
              <span>Progress</span>
              <strong>{{ selectedTask.progress_current }} / {{ selectedTask.progress_total || "?" }}</strong>
            </div>
            <div>
              <span>Started</span>
              <strong>{{ selectedTask.started_at || "Pending" }}</strong>
            </div>
            <div>
              <span>Finished</span>
              <strong>{{ selectedTask.finished_at || "Running" }}</strong>
            </div>
          </div>

          <section class="transcript-section">
            <h3>Message</h3>
            <p>{{ selectedTask.message || "No message" }}</p>
          </section>

          <div v-if="selectedTask.error" class="warning-box">
            <p>{{ selectedTask.error }}</p>
          </div>

          <section v-if="taskResultTitle(selectedTask)" class="transcript-section">
            <h3>Result</h3>
            <p>{{ taskResultTitle(selectedTask) }}</p>
            <button class="secondary" @click="openTaskResult(selectedTask)">
              <Icon name="eye" :size="17" />
              <span>Open result</span>
            </button>
          </section>

          <section v-if="selectedTaskSourceItems.length" class="transcript-section source-records">
            <h3>Adapter Records</h3>
            <div class="subtab-row">
              <button
                v-for="item in selectedTaskSourceItems"
                :key="item.id"
                class="subtab-button"
                :class="{ active: selectedTaskSourceItem?.id === item.id }"
                @click="selectTaskSourceItem(item)"
              >
                <span>{{ sourceItemLabel(item) }}</span>
                <small>{{ item.external_id || item.status }}</small>
              </button>
            </div>

            <div v-if="selectedTaskSourceItem" class="source-record-panel">
              <div v-if="selectedTaskSourceImages.length" class="image-strip compact">
                <button
                  v-for="image in selectedTaskSourceImages"
                  :key="image"
                  class="image-thumb"
                  type="button"
                  @click="openImagePreview(image, selectedTaskSourceImages)"
                >
                  <img :src="image" alt="" draggable="false" @dragstart.prevent />
                </button>
              </div>
              <div class="meta-grid">
                <div>
                  <span>Adapter</span>
                  <strong>{{ sourceItemLabel(selectedTaskSourceItem) }}</strong>
                </div>
                <div>
                  <span>External</span>
                  <strong>{{ selectedTaskSourceItem.external_id || "None" }}</strong>
                </div>
                <div>
                  <span>Version</span>
                  <strong>{{ selectedTaskSourceTranscript.inferred?.version || "Unknown" }}</strong>
                </div>
                <div>
                  <span>Fetched</span>
                  <strong>{{ selectedTaskSourceItem.fetched_at || "Unknown" }}</strong>
                </div>
              </div>
              <section v-if="selectedTaskSourceKeyValues.length" class="transcript-section">
                <h3>Fields</h3>
                <dl>
                  <template v-for="[key, values] in selectedTaskSourceKeyValues" :key="key">
                    <dt>{{ key }}</dt>
                    <dd>{{ values.join(", ") }}</dd>
                  </template>
                </dl>
              </section>
              <div class="result-actions">
                <a v-if="selectedTaskSourceItem.raw_url" class="open-link" :href="selectedTaskSourceItem.raw_url" target="_blank">
                  <Icon name="eye" :size="17" />
                  <span>Open source</span>
                </a>
              </div>
            </div>
          </section>
        </div>
        <div v-else class="detail-pane">
          <div class="empty-detail">
            <Icon name="activity" :size="24" />
            <strong>No task selected</strong>
          </div>
        </div>
      </section>

      <section v-else class="workspace two-column items-layout">
        <div class="list-pane">
          <div class="pane-title">
            <h1>Items</h1>
          </div>
          <div
            v-for="item in sourceItems"
            :key="item.id"
            :class="{ selected: selectedItem?.id === item.id }"
            class="row-entry"
          >
            <button
              class="row-button"
              :class="{ selected: selectedItem?.id === item.id }"
              @click="selectItem(item)"
            >
              <span class="item-state" :class="item.status"></span>
              <span class="row-main">
                <strong>{{ item.title }}</strong>
                <small>{{ item.external_id || "no external id" }} · {{ item.status }}</small>
              </span>
            </button>
            <button class="row-delete" title="Delete item" @click.stop="requestDeleteSourceItem(item)">
              <Icon name="trash" :size="16" />
            </button>
          </div>
          <div v-if="sourceItems.length === 0" class="empty-state">
            <Icon name="file-search" :size="20" />
            <strong>No items</strong>
          </div>
        </div>

        <div v-if="selectedItem" class="detail-pane transcript-pane">
          <div class="pane-title">
            <h2>{{ selectedItem.title }}</h2>
            <div class="button-row">
              <button v-if="selectedMatchedGame" class="secondary" @click="openMatchedGame">
                <Icon name="eye" :size="17" />
                <span>Open game</span>
              </button>
              <button v-else class="secondary" @click="createGameFromItem(selectedItem)">
                <Icon name="plus" :size="17" />
                <span>Create game</span>
              </button>
              <button class="secondary" :disabled="mediaRetrying" @click="retryItemImages(selectedItem)">
                <Icon name="refresh" :size="17" />
                <span>{{ mediaRetrying ? "Retrying" : "Retry images" }}</span>
              </button>
            </div>
          </div>

          <div class="match-bar">
            <span v-if="selectedMatchedGame" class="match-current">
              Matched to <strong>{{ selectedMatchedGame.title }}</strong>
            </span>
            <template v-if="!selectedMatchedGame || matchEditing">
              <select v-if="availableMatchGames.length" v-model="matchGameId">
                <option value="">{{ selectedMatchedGame ? "Choose another game..." : "Select game..." }}</option>
                <option v-for="game in availableMatchGames" :key="game.id" :value="game.id">
                  {{ game.title }}
                </option>
              </select>
              <span v-else class="match-empty">
                {{ selectedMatchedGame ? "No other games available" : "No games available to match" }}
              </span>
              <button
                v-if="availableMatchGames.length"
                class="secondary"
                :disabled="!canApplyMatch"
                @click="matchItem(selectedItem)"
              >
                <Icon name="cable" :size="17" />
                <span>{{ selectedMatchedGame ? "Apply" : "Match" }}</span>
              </button>
              <button v-if="matchEditing" class="secondary" @click="cancelMatchEdit">
                <Icon name="x" :size="17" />
                <span>Cancel</span>
              </button>
            </template>
            <button v-else-if="availableMatchGames.length" class="secondary" @click="startMatchEdit">
              <Icon name="cable" :size="17" />
              <span>Change match</span>
            </button>
          </div>

          <div v-if="previewMediaEntries.length" class="image-strip">
            <div
              v-for="entry in previewMediaEntries"
              :key="entry.key"
              class="image-card"
              :class="{ failed: entry.status === 'failed' }"
            >
              <button
                v-if="entry.status === 'cached'"
                class="image-thumb"
                type="button"
                @click="openImagePreview(entry.public_url, previewImages)"
              >
                <img :src="entry.public_url" alt="" draggable="false" @dragstart.prevent />
                <span v-if="entry.public_url === activeCoverImage" class="image-badge">Cover</span>
              </button>
              <button
                v-else
                class="image-thumb image-placeholder"
                type="button"
                :disabled="retryingMediaURL === entry.original_url"
                @click="retryItemImage(selectedItem, entry)"
              >
                <Icon name="refresh" :size="20" />
                <strong>{{ entry.role === "cover" ? "Cover failed" : "Image failed" }}</strong>
                <small>{{ retryingMediaURL === entry.original_url ? "Retrying" : "Retry" }}</small>
              </button>
              <button
                v-if="entry.status === 'cached' && selectedMatchedGame && activeCoverImage !== entry.public_url"
                class="image-action"
                type="button"
                title="Set cover"
                @click="setGameCoverFromImage(entry.public_url)"
              >
                <Icon name="check" :size="14" />
                <span>Cover</span>
              </button>
              <button
                v-else-if="entry.status === 'failed'"
                class="image-action"
                type="button"
                title="Retry image"
                :disabled="retryingMediaURL === entry.original_url"
                @click="retryItemImage(selectedItem, entry)"
              >
                <Icon name="refresh" :size="14" />
                <span>Retry</span>
              </button>
            </div>
          </div>

          <div class="meta-grid">
            <div>
              <span>Game</span>
              <strong>{{ selectedTranscript.fields?.game_name || selectedTranscript.inferred?.game_title || selectedTranscript.title }}</strong>
            </div>
            <div>
              <span>Version</span>
              <strong>{{ selectedTranscript.fields?.version || selectedTranscript.inferred?.version || "Unknown" }}</strong>
            </div>
            <div>
              <span>Developer</span>
              <strong>{{ selectedTranscript.fields?.developer || selectedTranscript.inferred?.developer || "Unknown" }}</strong>
            </div>
            <div>
              <span>Engine</span>
              <strong>{{ selectedTranscript.fields?.engine || "Unknown" }}</strong>
            </div>
            <div>
              <span>Prefixes</span>
              <strong>{{ formatList(selectedTranscript.fields?.prefixes) }}</strong>
            </div>
            <div>
              <span>Censored</span>
              <strong>{{ formatBoolean(selectedTranscript.fields?.censored) }}</strong>
            </div>
            <div>
              <span>Updated</span>
              <strong>{{ selectedTranscript.fields?.thread_updated || "Unknown" }}</strong>
            </div>
            <div>
              <span>Released</span>
              <strong>{{ selectedTranscript.fields?.release_date || "Unknown" }}</strong>
            </div>
            <div>
              <span>OS</span>
              <strong>{{ formatList(selectedTranscript.fields?.operating_systems) }}</strong>
            </div>
            <div>
              <span>Language</span>
              <strong>{{ formatList(selectedTranscript.fields?.languages) }}</strong>
            </div>
            <div>
              <span>Genre</span>
              <strong>{{ formatList(selectedTranscript.fields?.genres) }}</strong>
            </div>
          </div>

          <div v-if="selectedTranscript.warnings?.length" class="warning-box">
            <p v-for="warning in selectedTranscript.warnings" :key="warning">{{ warning }}</p>
          </div>

          <section v-if="selectedTranscript.fields?.description" class="transcript-section">
            <h3>Overview</h3>
            <p>{{ selectedTranscript.fields.description }}</p>
          </section>

          <section v-if="selectedTranscript.fields?.developer_links?.length" class="transcript-section">
            <h3>Developer Links</h3>
            <div class="link-list">
              <a
                v-for="link in selectedTranscript.fields.developer_links"
                :key="`${link.name}-${link.url}`"
                :href="link.url"
                target="_blank"
              >
                {{ link.name }}
              </a>
            </div>
          </section>

          <section v-if="selectedTranscript.fields?.download_groups?.length" class="transcript-section">
            <h3>Downloads</h3>
            <dl>
              <template v-for="group in selectedTranscript.fields.download_groups" :key="group.platform">
                <dt>
                  {{ group.platform }}
                  <small v-if="group.note">{{ group.note }}</small>
                </dt>
                <dd>
                  <div v-if="group.links?.length" class="download-list">
                    <template v-for="(link, index) in group.links" :key="`${group.platform}-${link.name}-${index}`">
                      <a v-if="link.url" class="download-link" :href="link.url" target="_blank">
                        <strong>{{ link.name }}</strong>
                        <small>{{ link.url }}</small>
                      </a>
                      <span v-else class="download-link missing-url">
                        <strong>{{ link.name }}</strong>
                        <small>No URL captured</small>
                      </span>
                    </template>
                  </div>
                  <span v-else>Unknown</span>
                </dd>
              </template>
            </dl>
          </section>

          <section v-if="selectedTranscript.fields?.changelog" class="transcript-section">
            <h3>Changelog</h3>
            <div
              v-if="selectedTranscript.fields?.changelog_html"
              class="rich-html"
              v-html="selectedTranscript.fields.changelog_html"
            ></div>
            <p v-else>{{ selectedTranscript.fields.changelog }}</p>
          </section>

          <section v-if="keyValues.length" class="transcript-section">
            <h3>Raw Fields</h3>
            <dl>
              <template v-for="[key, values] in keyValues" :key="key">
                <dt>{{ key }}</dt>
                <dd>{{ values.join(", ") }}</dd>
              </template>
            </dl>
          </section>

          <section
            v-for="section in displayTranscriptSections(selectedTranscript)"
            :key="section.heading"
            class="transcript-section"
          >
            <h3>{{ section.heading }}</h3>
            <p>{{ section.body }}</p>
          </section>

          <div class="result-actions">
            <a v-if="selectedItem.raw_url" class="open-link" :href="selectedItem.raw_url" target="_blank">
              <Icon name="eye" :size="17" />
              <span>Open source</span>
            </a>
          </div>
        </div>
      </section>
    </main>

    <div v-if="previewImage" class="preview-layer" @click.self="closeImagePreview">
      <div class="preview-popover" role="dialog" aria-modal="true" aria-label="Image preview">
        <button class="icon-button preview-close" title="Close preview" @click="closeImagePreview">
          <Icon name="x" :size="18" />
        </button>
        <button v-if="canStepPreview" class="icon-button preview-nav previous" title="Previous image" @click="stepPreview(-1)">
          <Icon name="arrow-left" :size="20" />
        </button>
        <img :src="previewImage" alt="" draggable="false" @dragstart.prevent />
        <button v-if="canStepPreview" class="icon-button preview-nav next" title="Next image" @click="stepPreview(1)">
          <Icon name="arrow-right" :size="20" />
        </button>
        <span v-if="canStepPreview" class="preview-count">
          {{ previewImageIndex + 1 }} / {{ previewSequence.length }}
        </span>
      </div>
    </div>

    <Transition name="toast">
      <div v-if="notice || error" class="toast-layer" role="status" aria-live="polite">
        <div class="toast" :class="{ error: !notice && Boolean(error) }">
          <Icon :name="notice ? 'check' : 'x'" :size="17" />
          <span>{{ notice || error }}</span>
        </div>
      </div>
    </Transition>

    <div v-if="pendingDelete" class="confirm-layer" @click.self="cancelDelete">
      <div class="confirm-popover" role="dialog" aria-modal="true" aria-labelledby="delete-confirm-title">
        <div class="confirm-icon">
          <Icon name="trash" :size="20" />
        </div>
        <div class="confirm-copy">
          <h2 id="delete-confirm-title">{{ pendingDelete.firstMessage }}</h2>
          <p>
            <strong>{{ pendingDelete.title }}</strong>
            <span>{{ pendingDelete.secondMessage }}</span>
          </p>
        </div>
        <div class="confirm-actions">
          <button class="secondary" :disabled="deleting" @click="cancelDelete">
            <Icon name="x" :size="17" />
            <span>Cancel</span>
          </button>
          <button class="danger" :disabled="deleting" @click="confirmDelete">
            <Icon name="trash" :size="17" />
            <span>{{ deleting ? "Deleting" : "Delete" }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
