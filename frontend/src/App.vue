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
  key_values?: Record<string, string[]>;
  sections?: Array<{ heading: string; body: string }>;
  images?: string[];
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

type ViewName = "games" | "sources" | "import" | "items" | "tasks";

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
const sourceItems = ref<SourceItem[]>([]);
const tasks = ref<Task[]>([]);
const selectedGame = ref<Game | null>(null);
const selectedItem = ref<SourceItem | null>(null);
const selectedTask = ref<Task | null>(null);
const lastImportTaskId = ref<number | null>(null);
const selectedTaskSourceItemId = ref<number | null>(null);
const pendingDelete = ref<DeleteTarget | null>(null);
const deleting = ref(false);
let noticeTimer: number | undefined;

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

const sourceDraft = reactive({
  name: "F95zone",
  type: "f95zone",
  url: "",
  proxy_url: "",
  enabled: true,
  trust_level: 50,
  config_json: "{}"
});

const importDraftDefaults = {
  source_id: "",
  url: "",
  proxy_url: "",
  html: "",
  create_game: true
};

const importDraft = reactive({ ...importDraftDefaults });

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

const previewImages = computed(() => {
  return selectedTranscript.value.images?.slice(0, 8) ?? [];
});

const activeTasks = computed(() => tasks.value.filter(isTaskActive));

const recentImportTask = computed(() => {
  return lastImportTaskId.value ? tasks.value.find((task) => task.id === lastImportTaskId.value) ?? null : null;
});

const recentImportImages = computed(() => {
  const result = recentImportTask.value?.result_json;
  const transcript = result?.transcript ?? result?.item?.parsed_json;
  return transcript?.images?.slice(0, 5) ?? [];
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
  return selectedTaskSourceTranscript.value.images?.slice(0, 8) ?? [];
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
  const [nextGames, nextSources, nextItems] = await Promise.all([
    api<Game[]>("/api/games"),
    api<Source[]>("/api/sources"),
    api<SourceItem[]>("/api/source-items")
  ]);
  games.value = nextGames;
  sources.value = nextSources;
  sourceItems.value = nextItems;
  if (!selectedGame.value && nextGames.length > 0) {
    editGame(nextGames[0]);
  }
  if (!selectedItem.value && nextItems.length > 0) {
    selectedItem.value = nextItems[0];
    matchGameId.value = selectedItem.value.matched_game_id?.toString() ?? "";
  }
}

async function loadTasks() {
  const hadActiveTasks = activeTasks.value.length > 0;
  const nextTasks = await api<Task[]>("/api/tasks");
  tasks.value = nextTasks;
  const lastImportTask = lastImportTaskId.value
    ? nextTasks.find((task) => task.id === lastImportTaskId.value)
    : null;
  if (!selectedTask.value && lastImportTask) {
    selectedTask.value = lastImportTask;
  } else if (!selectedTask.value && nextTasks.length > 0) {
    selectedTask.value = nextTasks[0];
  } else if (selectedTask.value) {
    selectedTask.value = nextTasks.find((task) => task.id === selectedTask.value?.id) ?? selectedTask.value;
  }
  if (hadActiveTasks && !nextTasks.some(isTaskActive)) {
    await loadLibrary();
  }
}

function newGame() {
  selectedGame.value = null;
  Object.assign(gameDraft, {
    id: 0,
    title: "",
    aliases: "",
    description: "",
    current_version: "",
    cover_image: ""
  });
}

function editGame(game: Game) {
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
    editGame(saved);
  } catch (err) {
    error.value = toMessage(err);
  }
}

async function saveSource() {
  error.value = "";
  clearNotice();
  try {
    await api<Source>("/api/sources", {
      method: "POST",
      body: JSON.stringify(sourceDraft)
    });
    showNotice("Source saved");
    await loadAll();
  } catch (err) {
    error.value = toMessage(err);
  }
}

async function runImport() {
  error.value = "";
  clearNotice();
  loading.value = true;
  const payload = {
    source_id: importDraft.source_id ? Number(importDraft.source_id) : undefined,
    url: importDraft.url.trim(),
    proxy_url: importDraft.proxy_url.trim(),
    html: importDraft.html,
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
    editGame(result.game);
  } catch (err) {
    error.value = toMessage(err);
  }
}

async function matchItem(item: SourceItem) {
  error.value = "";
  clearNotice();
  const gameId = matchGameId.value ? Number(matchGameId.value) : undefined;
  try {
    const updated = await api<SourceItem>(`/api/source-items/${item.id}/match`, {
      method: "POST",
      body: JSON.stringify({ game_id: gameId })
    });
    selectedItem.value = updated;
    showNotice("Matched");
    await loadAll();
  } catch (err) {
    error.value = toMessage(err);
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
      newGame();
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
  matchGameId.value = item.matched_game_id?.toString() ?? "";
}

function selectTask(task: Task) {
  selectedTask.value = task;
  selectedTaskSourceItemId.value = null;
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
    editGame(task.result_json.game);
    view.value = "games";
    return;
  }
  if (task.result_json?.item) {
    selectItem(task.result_json.item);
    view.value = "items";
  }
}

function applySourceToImport(source: Source) {
  importDraft.source_id = source.id.toString();
  importDraft.url = source.url;
  importDraft.proxy_url = source.proxy_url;
  view.value = "import";
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
  return source?.name || item.source_type || "Standalone";
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
        source_id: typeof draft.source_id === "string" ? draft.source_id : "",
        url: typeof draft.url === "string" ? draft.url : "",
        proxy_url: typeof draft.proxy_url === "string" ? draft.proxy_url : "",
        html: typeof draft.html === "string" ? draft.html : "",
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
    importDraft.source_id === importDraftDefaults.source_id &&
    importDraft.url === importDraftDefaults.url &&
    importDraft.proxy_url === importDraftDefaults.proxy_url &&
    importDraft.html === importDraftDefaults.html &&
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

onMounted(() => {
  restoreImportState();
  void loadAll();
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
        <button :class="{ active: view === 'sources' }" title="Sources" @click="view = 'sources'">
          <Icon name="globe" :size="18" />
          <span>Sources</span>
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
        <div class="status-line" aria-live="polite">
          <span v-if="loading">Loading</span>
          <span v-else-if="error" class="error">{{ error }}</span>
          <span v-else-if="activeTasks.length">{{ activeTasks.length }} active task{{ activeTasks.length === 1 ? "" : "s" }}</span>
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
              @click="editGame(game)"
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

        <div class="detail-pane">
          <div class="pane-title">
            <h2>{{ gameDraft.id ? "Edit Game" : "New Game" }}</h2>
            <button class="primary" @click="saveGame">
              <Icon name="save" :size="17" />
              <span>Save</span>
            </button>
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
      </section>

      <section v-else-if="view === 'sources'" class="workspace two-column">
        <div class="list-pane">
          <div class="pane-title">
            <h1>Sources</h1>
          </div>
          <button
            v-for="source in sources"
            :key="source.id"
            class="row-button"
            @click="applySourceToImport(source)"
          >
            <span class="source-dot"></span>
            <span class="row-main">
              <strong>{{ source.name }}</strong>
              <small>{{ source.type }} · {{ source.proxy_url || "direct" }}</small>
            </span>
            <Icon name="link" :size="16" />
          </button>
        </div>

        <div class="detail-pane">
          <div class="pane-title">
            <h2>New Source</h2>
            <button class="primary" @click="saveSource">
              <Icon name="save" :size="17" />
              <span>Save</span>
            </button>
          </div>
          <label>
            <span>Name</span>
            <input v-model="sourceDraft.name" type="text" />
          </label>
          <label>
            <span>Type</span>
            <select v-model="sourceDraft.type">
              <option value="f95zone">F95zone</option>
            </select>
          </label>
          <label class="wide">
            <span>Thread URL</span>
            <input v-model="sourceDraft.url" type="url" />
          </label>
          <label class="wide">
            <span>Proxy URL</span>
            <input v-model="sourceDraft.proxy_url" type="text" placeholder="socks5://127.0.0.1:7890" />
          </label>
          <label>
            <span>Trust</span>
            <input v-model.number="sourceDraft.trust_level" min="0" max="100" type="number" />
          </label>
          <label class="toggle-row">
            <input v-model="sourceDraft.enabled" type="checkbox" />
            <span>Enabled</span>
          </label>
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
            <div v-if="recentImportImages.length" class="image-strip compact">
              <img v-for="image in recentImportImages" :key="image" :src="image" alt="" />
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
            <span>Source</span>
            <select v-model="importDraft.source_id">
              <option value="">Standalone</option>
              <option v-for="source in sources" :key="source.id" :value="source.id">
                {{ source.name }}
              </option>
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
          <label class="wide">
            <span>Raw HTML</span>
            <textarea v-model="importDraft.html" rows="16"></textarea>
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
          <button
            v-for="task in tasks"
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
            <h3>Source Records</h3>
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
                <img v-for="image in selectedTaskSourceImages" :key="image" :src="image" alt="" />
              </div>
              <div class="meta-grid">
                <div>
                  <span>Source</span>
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
              <a v-if="selectedTaskSourceItem.raw_url" class="open-link" :href="selectedTaskSourceItem.raw_url" target="_blank">
                <Icon name="eye" :size="17" />
                <span>Open source</span>
              </a>
            </div>
          </section>
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
            <button class="secondary" @click="createGameFromItem(selectedItem)">
              <Icon name="plus" :size="17" />
              <span>Game</span>
            </button>
          </div>

          <div class="match-bar">
            <select v-model="matchGameId">
              <option value="">Unmatched</option>
              <option v-for="game in games" :key="game.id" :value="game.id">
                {{ game.title }}
              </option>
            </select>
            <button class="secondary" @click="matchItem(selectedItem)">
              <Icon name="cable" :size="17" />
              <span>Match</span>
            </button>
          </div>

          <div v-if="previewImages.length" class="image-strip">
            <img v-for="image in previewImages" :key="image" :src="image" alt="" />
          </div>

          <div class="meta-grid">
            <div>
              <span>Game</span>
              <strong>{{ selectedTranscript.inferred?.game_title || selectedTranscript.title }}</strong>
            </div>
            <div>
              <span>Version</span>
              <strong>{{ selectedTranscript.inferred?.version || "Unknown" }}</strong>
            </div>
            <div>
              <span>Developer</span>
              <strong>{{ selectedTranscript.inferred?.developer || "Unknown" }}</strong>
            </div>
            <div>
              <span>Raw</span>
              <strong>{{ selectedItem.raw_content_path ? "Saved" : "None" }}</strong>
            </div>
          </div>

          <div v-if="selectedTranscript.warnings?.length" class="warning-box">
            <p v-for="warning in selectedTranscript.warnings" :key="warning">{{ warning }}</p>
          </div>

          <section v-if="keyValues.length" class="transcript-section">
            <h3>Fields</h3>
            <dl>
              <template v-for="[key, values] in keyValues" :key="key">
                <dt>{{ key }}</dt>
                <dd>{{ values.join(", ") }}</dd>
              </template>
            </dl>
          </section>

          <section
            v-for="section in selectedTranscript.sections"
            :key="section.heading"
            class="transcript-section"
          >
            <h3>{{ section.heading }}</h3>
            <p>{{ section.body }}</p>
          </section>

          <a v-if="selectedItem.raw_url" class="open-link" :href="selectedItem.raw_url" target="_blank">
            <Icon name="eye" :size="17" />
            <span>Open source</span>
          </a>
        </div>
      </section>
    </main>

    <Transition name="toast">
      <div v-if="notice" class="toast-layer" role="status" aria-live="polite">
        <div class="toast">
          <Icon name="check" :size="17" />
          <span>{{ notice }}</span>
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
