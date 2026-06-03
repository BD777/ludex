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

type AdapterBrowseCapabilities = {
  custom_url: boolean;
  pagination: boolean;
  search: boolean;
  filter: boolean;
  sort: boolean;
  import: boolean;
  search_note: string;
  filter_note: string;
  sort_note: string;
};

type AdapterBrowsePreset = {
  id: string;
  label: string;
  description: string;
  url: string;
};

type AdapterBrowseManifest = {
  enabled: boolean;
  description: string;
  presets: AdapterBrowsePreset[];
  capabilities: AdapterBrowseCapabilities;
};

type Adapter = {
  id: string;
  name: string;
  kind: string;
  status: string;
  input: string;
  dedupe: string;
  endpoint: string;
  attachments: string;
  auth_domain: string;
  auth_cookie_names: string[];
  without_bridge: string;
  browse: AdapterBrowseManifest;
};

type AdapterBrowseFilter = {
  id: string;
  label: string;
  count: string;
  url: string;
};

type AdapterListItem = {
  adapter_id: string;
  external_id: string;
  title: string;
  url: string;
  preview_url: string;
  cover_image: string;
  author: string;
  summary: string;
  started_at: string;
  latest_at: string;
  latest_by: string;
  prefixes: string[];
  tags: string[];
  replies: string;
  views: string;
  rating: string;
  votes: string;
  importable: boolean;
};

type AdapterBrowsePage = {
  adapter_id: string;
  title: string;
  url: string;
  page: number;
  total_pages: number;
  prev_url: string;
  next_url: string;
  items: AdapterListItem[];
  filters: AdapterBrowseFilter[];
  warnings: string[];
  capabilities: AdapterBrowseCapabilities;
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

type TelegramStatus = {
  configured: boolean;
  authorized: boolean;
  user_id?: number;
  username?: string;
  phone?: string;
  first_name?: string;
  last_name?: string;
  error?: string;
};

type TelegramDialog = {
  peer_id: string;
  type: string;
  title: string;
  username?: string;
  access_hash?: string;
  participants?: number;
  date?: string;
  source_url: string;
  config_json: string;
};

type TelegramSendCodeResult = {
  code_sent: boolean;
  phone: string;
  code_type: string;
  timeout?: number;
  status: TelegramStatus;
};

type TelegramSignInResult = {
  password_required: boolean;
  status: TelegramStatus;
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
    import_request?: {
      source_id?: number;
      message_id?: number;
      url?: string;
      proxy_url?: string;
      create_game?: boolean;
      has_html?: boolean;
    };
    retried_by_task_id?: number;
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

type ViewName = "games" | "adapters" | "import" | "review" | "tasks";

type ImportMode = "browse" | "direct";

type GamePanelMode = "detail" | "edit" | "new";

type GameDetailTab = "details" | "sources";

type CoverLoadState = "loading" | "loaded" | "failed";

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
const importMode = ref<ImportMode>("browse");

const games = ref<Game[]>([]);
const adapters = ref<Adapter[]>([]);
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
const selectedGameTab = ref<GameDetailTab>("details");
const taskHistoryExpanded = ref(false);
const matchEditing = ref(false);
const savingGame = ref(false);
const creatingGameItemId = ref<number | null>(null);
const matchingItemId = ref<number | null>(null);
const unlinkingItemId = ref<number | null>(null);
const mediaRetrying = ref(false);
const retryingMediaURL = ref("");
const retryingTaskId = ref<number | null>(null);
const sourceLinkOpen = ref(false);
let noticeTimer: number | undefined;
let errorTimer: number | undefined;

const importDraftStorageKey = "gmb.importDraft.v1";
const adapterBrowseStorageKey = "gmb.adapterBrowse.v1";
const lastImportTaskStorageKey = "gmb.lastImportTaskId.v1";
let restoringAdapterBrowseState = false;

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

const adapterBrowseDraftDefaults = {
  preset_id: "trending",
  url: "",
  filter_url: "",
  page: 1,
  limit: 30,
  offset_id: 0,
  search: "",
  sort: "",
  proxy_url: ""
};

const adapterBrowseDraft = reactive({ ...adapterBrowseDraftDefaults });
const adapterBrowsePage = ref<AdapterBrowsePage | null>(null);
const adapterBrowsing = ref(false);
const adapterCoverStates = reactive<Record<string, CoverLoadState>>({});
const telegramStatus = ref<TelegramStatus>({ configured: false, authorized: false });
const telegramDialogs = ref<TelegramDialog[]>([]);
const telegramLoading = ref(false);
const telegramCodeSent = ref(false);
const telegramPasswordRequired = ref(false);
const telegramSavingSourceURL = ref("");
const telegramDialogQuery = ref("");
const selectedTelegramSourceIds = ref<number[]>([]);
const telegramAuthDraft = reactive({
  api_id: "",
  api_hash: "",
  phone: "",
  code: "",
  password: ""
});

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

const fallbackAdapters: Adapter[] = [
  {
    id: "f95zone",
    name: "F95zone",
    kind: "Forum thread",
    status: "built-in",
    input: "Thread URL",
    dedupe: "Thread ID",
    endpoint: "/api/import/f95zone",
    attachments: "Cached images",
    auth_domain: "f95zone.to",
    auth_cookie_names: ["xf_user"],
    without_bridge: "Download links, login-only spoilers/changelog, and some developer/social links may be unavailable.",
    browse: {
      enabled: true,
      description: "Browse XenForo thread lists and import selected F95zone game threads.",
      presets: [
        {
          id: "trending",
          label: "Trending games",
          description: "F95zone trending game threads.",
          url: "https://f95zone.to/trending/threads.1/"
        },
        {
          id: "games",
          label: "Games forum",
          description: "The main Games forum thread list.",
          url: "https://f95zone.to/forums/games.2/"
        }
      ],
      capabilities: {
        custom_url: true,
        pagination: true,
        search: false,
        filter: true,
        sort: false,
        import: true,
        search_note: "F95zone list search is not enabled yet; XenForo search requires a separate adapter flow.",
        filter_note: "F95zone browse supports source-provided prefix filters from the current list page.",
        sort_note: "F95zone list sorting is not enabled yet."
      }
    }
  },
  {
    id: "telegram",
    name: "Telegram",
    kind: "MTProto account",
    status: "built-in",
    input: "Authorized account and selected group/channel",
    dedupe: "Peer ID",
    endpoint: "/api/telegram/dialogs",
    attachments: "Message media later",
    auth_domain: "telegram.org",
    auth_cookie_names: [],
    without_bridge: "Requires a Telegram API ID/hash and phone-code login; message field extraction is configured per group later.",
    browse: {
      enabled: true,
      description: "Browse selected Telegram groups/channels and import individual messages as source items.",
      presets: [],
      capabilities: {
        custom_url: false,
        pagination: true,
        search: true,
        filter: false,
        sort: false,
        import: true,
        search_note: "Telegram searches inside one selected group or channel.",
        filter_note: "Telegram group-specific filters will be added after the target groups are connected.",
        sort_note: "Telegram message browsing uses Telegram's default newest-first order."
      }
    }
  }
];

const matchGameId = ref("");
const matchGameQuery = ref("");
const matchPickerOpen = ref(false);
const matchGameLimit = 30;
let matchPickerCloseTimer: number | undefined;

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

const unmatchedSourceItems = computed(() => {
  return sourceItems.value
    .filter((item) => !item.matched_game_id)
    .sort(compareSourceItemsByUpdatedAt);
});

const reviewSourceItems = computed(() => {
  const items = [...unmatchedSourceItems.value];
  if (selectedItem.value && !items.some((item) => item.id === selectedItem.value?.id)) {
    items.unshift(selectedItem.value);
  }
  return items;
});

const reviewItemCount = computed(() => unmatchedSourceItems.value.length);

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
    .sort(compareSourceItemsByUpdatedAt);
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

const completedTasks = computed(() => tasks.value.filter((task) => !isTaskActive(task) && !isTaskRetried(task)));

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

const availableAdapters = computed(() => (adapters.value.length ? adapters.value : fallbackAdapters));

const selectedAdapter = computed(() => {
  return availableAdapters.value.find((adapter) => adapter.id === selectedAdapterId.value) ?? fallbackAdapters[0];
});

const selectedAdapterAuthProfile = computed(() => {
  return (
    authProfiles.value.find(
      (profile) => profile.adapter_id === selectedAdapter.value.id && profile.domain === selectedAdapter.value.auth_domain
    ) ?? null
  );
});

const adapterBrowseCapabilities = computed(() => selectedAdapter.value.browse.capabilities);

const adapterBrowsePresets = computed(() => selectedAdapter.value.browse.presets ?? []);

const adapterBrowseFilters = computed(() => adapterBrowsePage.value?.filters ?? []);

const adapterBrowseLoadingCovers = computed(() => {
  return adapterBrowsePage.value?.items.filter((item) => adapterBrowseCoverStatus(item) === "loading").length ?? 0;
});

const telegramSources = computed(() => {
  return sources.value.filter((source) => source.type === "telegram");
});

const selectedTelegramSources = computed(() => {
  const selected = new Set(selectedTelegramSourceIds.value);
  return telegramSources.value.filter((source) => selected.has(source.id));
});

const telegramSearchDisabled = computed(() => selectedTelegramSourceIds.value.length !== 1);

const canLoadTelegramMessages = computed(() => {
  return selectedAdapter.value.id === "telegram" && selectedTelegramSourceIds.value.length > 0 && !adapterBrowsing.value;
});

const telegramOlderOffsetID = computed(() => {
  if (selectedAdapter.value.id !== "telegram" || !adapterBrowsePage.value?.next_url) {
    return 0;
  }
  try {
    const parsed = new URL(adapterBrowsePage.value.next_url);
    return Number(parsed.searchParams.get("offset_id") ?? "0");
  } catch {
    const match = /offset_id=(\d+)/.exec(adapterBrowsePage.value.next_url);
    return match ? Number(match[1]) : 0;
  }
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

const telegramAuthLabel = computed(() => {
  if (telegramStatus.value.authorized) {
    return "authorized";
  }
  if (telegramStatus.value.configured) {
    return "configured";
  }
  return "missing";
});

const telegramAuthClass = computed(() => {
  if (telegramStatus.value.authorized) return "succeeded";
  if (telegramStatus.value.configured) return "warning";
  return "failed";
});

const telegramAccountName = computed(() => {
  const status = telegramStatus.value;
  if (status.username) return `@${status.username}`;
  const fullName = [status.first_name, status.last_name].filter(Boolean).join(" ");
  return fullName || "Not signed in";
});

const canSubmitTelegramSignIn = computed(() => {
  return (
    telegramCodeSent.value ||
    telegramPasswordRequired.value ||
    telegramAuthDraft.code.trim().length > 0 ||
    telegramAuthDraft.password.trim().length > 0
  );
});

const filteredTelegramDialogs = computed(() => {
  const needle = telegramDialogQuery.value.trim().toLowerCase();
  if (!needle) {
    return telegramDialogs.value;
  }
  return telegramDialogs.value.filter((dialog) => {
    return (
      dialog.title.toLowerCase().includes(needle) ||
      dialog.type.toLowerCase().includes(needle) ||
      (dialog.username ?? "").toLowerCase().includes(needle)
    );
  });
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

const selectedMatchGame = computed(() => {
  const id = Number(matchGameId.value);
  if (!Number.isFinite(id)) {
    return null;
  }
  return availableMatchGames.value.find((game) => game.id === id) ?? null;
});

const matchingMatchGames = computed(() => {
  const needle = matchGameQuery.value.trim().toLowerCase();
  if (!needle) {
    return availableMatchGames.value;
  }
  if (selectedMatchGame.value && needle === formatMatchGameOption(selectedMatchGame.value).toLowerCase()) {
    return availableMatchGames.value;
  }
  return availableMatchGames.value.filter((game) => gameMatchesQuery(game, needle));
});

const limitedMatchGames = computed(() => matchingMatchGames.value.slice(0, matchGameLimit));

const hiddenMatchGameCount = computed(() => Math.max(0, matchingMatchGames.value.length - limitedMatchGames.value.length));

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
  const [nextAdapters, nextGames, nextSources, nextAuthProfiles, nextItems] = await Promise.all([
    api<Adapter[]>("/api/adapters"),
    api<Game[]>("/api/games"),
    api<Source[]>("/api/sources"),
    api<AuthProfile[]>("/api/auth-profiles"),
    api<SourceItem[]>("/api/source-items")
  ]);
  adapters.value = nextAdapters.length ? nextAdapters : fallbackAdapters;
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
      resetMatchPicker();
      matchEditing.value = false;
    }
  }
  if (!selectedItem.value && unmatchedSourceItems.value.length > 0) {
    selectedItem.value = unmatchedSourceItems.value[0];
    resetMatchPicker();
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

function refreshLibraryInBackground() {
  void loadLibrary().catch((err) => {
    error.value = toMessage(err);
  });
}

function upsertGameInState(game: Game) {
  const index = games.value.findIndex((entry) => entry.id === game.id);
  if (index >= 0) {
    games.value = games.value.map((entry) => (entry.id === game.id ? game : entry));
  } else {
    games.value = [game, ...games.value];
  }
}

function upsertSourceItemInState(item: SourceItem) {
  const index = sourceItems.value.findIndex((entry) => entry.id === item.id);
  if (index >= 0) {
    sourceItems.value = sourceItems.value.map((entry) => (entry.id === item.id ? item : entry));
  } else {
    sourceItems.value = [item, ...sourceItems.value];
  }
}

function newGame() {
  selectedGame.value = null;
  gamePanelMode.value = "new";
  selectedGameTab.value = "details";
  sourceLinkOpen.value = false;
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
  sourceLinkOpen.value = false;
}

function editGame(game: Game) {
  assignGameDraft(game);
  gamePanelMode.value = "edit";
}

function startGameEdit() {
  if (selectedGame.value) {
    selectedGameTab.value = "details";
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
  selectedGameTab.value = "details";
}

async function saveGame() {
  if (savingGame.value) {
    return;
  }
  error.value = "";
  clearNotice();
  savingGame.value = true;
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
    upsertGameInState(saved);
    selectGame(saved);
    refreshLibraryInBackground();
  } catch (err) {
    error.value = toMessage(err);
  } finally {
    savingGame.value = false;
  }
}

async function runImport() {
  await queueF95zoneImport(importDraft.url.trim());
}

async function queueF95zoneImport(url: string) {
  error.value = "";
  clearNotice();
  loading.value = true;
  const payload = {
    url,
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

async function retryTask(task: Task) {
  if (!canRetryTask(task)) {
    return;
  }
  error.value = "";
  clearNotice();
  retryingTaskId.value = task.id;
  try {
    const result = await api<{ task: Task; duplicate?: boolean }>(`/api/tasks/${task.id}/retry`, {
      method: "POST",
      body: JSON.stringify({
        proxy_url: importDraft.proxy_url.trim() || adapterBrowseDraft.proxy_url.trim()
      })
    });
    showNotice(result.duplicate ? "Import task already running" : "Import retry started");
    lastImportTaskId.value = result.task.id;
    persistLastImportTaskId();
    selectedTask.value = result.task;
    await loadTasks();
  } catch (err) {
    error.value = toMessage(err);
  } finally {
    retryingTaskId.value = null;
  }
}

async function browseAdapterList(resetOffset = false) {
  if (!selectedAdapter.value.browse.enabled) {
    return;
  }
  if (selectedAdapter.value.id === "telegram") {
    if (selectedTelegramSourceIds.value.length === 0) {
      error.value = "Choose at least one Telegram group";
      return;
    }
    if (adapterBrowseDraft.search.trim() && selectedTelegramSourceIds.value.length !== 1) {
      error.value = "Telegram source search supports one group at a time";
      return;
    }
    if (resetOffset) {
      adapterBrowseDraft.offset_id = 0;
    }
  }
  error.value = "";
  clearNotice();
  adapterBrowsing.value = true;
  try {
    const page = await api<AdapterBrowsePage>(`/api/adapters/${selectedAdapter.value.id}/browse`, {
      method: "POST",
      body: JSON.stringify({
        preset_id: adapterBrowseDraft.preset_id,
        url: adapterBrowseDraft.url.trim(),
        filter_url: adapterBrowseDraft.filter_url.trim(),
        page: adapterBrowseDraft.page,
        limit: adapterBrowseDraft.limit,
        offset_id: adapterBrowseDraft.offset_id,
        source_ids: selectedAdapter.value.id === "telegram" ? selectedTelegramSourceIds.value : [],
        search: adapterBrowseDraft.search.trim(),
        sort: adapterBrowseDraft.sort,
        proxy_url: adapterBrowseDraft.proxy_url.trim() || importDraft.proxy_url.trim()
      })
    });
    adapterBrowsePage.value = page;
    prepareAdapterCoverStates(page);
    adapterBrowseDraft.page = page.page || adapterBrowseDraft.page || 1;
    adapterBrowseDraft.url = page.url;
    showNotice(selectedAdapter.value.id === "telegram" ? "Telegram messages loaded" : "Adapter list loaded");
  } catch (err) {
    error.value = toMessage(err);
  } finally {
    adapterBrowsing.value = false;
  }
}

function selectBrowsePreset(presetID: string) {
  adapterBrowseDraft.preset_id = presetID;
  adapterBrowseDraft.url = "";
  adapterBrowseDraft.filter_url = "";
  adapterBrowseDraft.page = 1;
  adapterBrowseDraft.offset_id = 0;
  adapterBrowsePage.value = null;
}

function resetAdapterBrowse() {
  const firstPreset = selectedAdapter.value.browse.presets?.[0];
  adapterBrowseDraft.preset_id = firstPreset?.id ?? "";
  adapterBrowseDraft.url = "";
  adapterBrowseDraft.filter_url = "";
  adapterBrowseDraft.page = 1;
  adapterBrowseDraft.offset_id = 0;
  adapterBrowseDraft.search = "";
  adapterBrowseDraft.sort = "";
  adapterBrowseDraft.proxy_url = "";
  adapterBrowsePage.value = null;
}

function resetImportFormsForAdapter() {
  Object.assign(importDraft, importDraftDefaults);
  resetAdapterBrowse();
  selectedTelegramSourceIds.value = [];
}

function changeImportAdapter(event: Event) {
  const target = event.target;
  if (!(target instanceof HTMLSelectElement)) {
    return;
  }
  selectedAdapterId.value = target.value;
  resetImportFormsForAdapter();
  if (selectedAdapter.value.id === "telegram") {
    importMode.value = "browse";
    void loadTelegramStatus();
  }
}

function setImportMode(nextMode: ImportMode) {
  if (nextMode === "direct" && selectedAdapter.value.id !== "f95zone") {
    return;
  }
  importMode.value = nextMode;
}

function selectBrowseFilter(filterURL: string) {
  adapterBrowseDraft.filter_url = filterURL;
  if (!filterURL) {
    adapterBrowseDraft.url = "";
  }
  adapterBrowseDraft.page = 1;
  void browseAdapterList();
}

function selectBrowseFilterFromEvent(event: Event) {
  const target = event.target;
  if (target instanceof HTMLSelectElement) {
    selectBrowseFilter(target.value);
  }
}

async function goBrowsePage(direction: 1 | -1) {
  if (selectedAdapter.value.id === "telegram") {
    if (direction > 0) {
      await loadOlderTelegramMessages();
    }
    return;
  }
  const targetURL = direction > 0 ? adapterBrowsePage.value?.next_url : adapterBrowsePage.value?.prev_url;
  if (!targetURL) {
    return;
  }
  adapterBrowseDraft.url = targetURL;
  adapterBrowseDraft.filter_url = "";
  adapterBrowseDraft.page = Math.max(1, (adapterBrowsePage.value?.page ?? adapterBrowseDraft.page) + direction);
  await browseAdapterList();
}

async function importBrowseItem(item: AdapterListItem) {
  if (!item.importable || !item.url) {
    return;
  }
  if (item.adapter_id === "telegram") {
    await queueTelegramImport(item);
    return;
  }
  importDraft.url = item.url;
  await queueF95zoneImport(item.url);
}

async function loadOlderTelegramMessages() {
  const offsetID = telegramOlderOffsetID.value;
  if (!offsetID) {
    return;
  }
  adapterBrowseDraft.offset_id = offsetID;
  await browseAdapterList();
}

async function queueTelegramImport(item: AdapterListItem) {
  const messageID = telegramMessageID(item);
  const source = telegramSourceForBrowseItem(item);
  if (!source || !messageID) {
    error.value = "Could not resolve the Telegram source for this message";
    return;
  }
  error.value = "";
  clearNotice();
  loading.value = true;
  try {
    const result = await api<{ task: Task; duplicate?: boolean }>("/api/import/telegram", {
      method: "POST",
      body: JSON.stringify({
        source_id: source.id,
        message_id: messageID,
        create_game: false
      })
    });
    showNotice(result.duplicate ? "Telegram import already running" : "Telegram import started");
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

function telegramSourceForBrowseItem(item: AdapterListItem) {
  const peerID = item.external_id.split(":")[0] ?? "";
  return telegramSources.value.find((source) => telegramSourcePeerID(source) === peerID) ?? null;
}

function telegramSourcePeerID(source: Source) {
  try {
    const parsed = JSON.parse(source.config_json || "{}") as { peer_id?: unknown };
    return typeof parsed.peer_id === "string" ? parsed.peer_id : "";
  } catch {
    return "";
  }
}

function telegramMessageID(item: AdapterListItem) {
  const raw = item.external_id.split(":").at(-1) ?? "";
  const value = Number(raw);
  return Number.isFinite(value) && value > 0 ? value : 0;
}

function telegramConfigPayload() {
  const apiID = Number(telegramAuthDraft.api_id);
  return {
    api_id: Number.isFinite(apiID) ? apiID : 0,
    api_hash: telegramAuthDraft.api_hash.trim(),
    phone: telegramAuthDraft.phone.trim()
  };
}

async function loadTelegramStatus() {
  telegramLoading.value = true;
  try {
    const status = await api<TelegramStatus>("/api/telegram/status");
    telegramStatus.value = status;
    if (status.phone && !telegramAuthDraft.phone) {
      telegramAuthDraft.phone = status.phone.startsWith("+") ? status.phone : `+${status.phone}`;
    }
  } catch (err) {
    telegramStatus.value = { configured: false, authorized: false, error: toMessage(err) };
  } finally {
    telegramLoading.value = false;
  }
}

async function sendTelegramCode() {
  error.value = "";
  clearNotice();
  telegramLoading.value = true;
  try {
    const result = await api<TelegramSendCodeResult>("/api/telegram/auth/send-code", {
      method: "POST",
      body: JSON.stringify(telegramConfigPayload())
    });
    telegramCodeSent.value = result.code_sent;
    telegramStatus.value = result.status;
    showNotice(`Telegram code sent${result.timeout ? `, expires in ${result.timeout}s` : ""}`);
  } catch (err) {
    error.value = toMessage(err);
  } finally {
    telegramLoading.value = false;
  }
}

async function signInTelegram() {
  error.value = "";
  clearNotice();
  telegramLoading.value = true;
  try {
    const result = await api<TelegramSignInResult>("/api/telegram/auth/sign-in", {
      method: "POST",
      body: JSON.stringify({
        ...telegramConfigPayload(),
        code: telegramAuthDraft.code.trim(),
        password: telegramAuthDraft.password
      })
    });
    telegramPasswordRequired.value = result.password_required;
    telegramStatus.value = result.status;
    if (result.password_required) {
      telegramAuthDraft.code = "";
      showNotice("Telegram 2FA password required");
    } else {
      telegramAuthDraft.code = "";
      telegramAuthDraft.password = "";
      telegramCodeSent.value = false;
      showNotice("Telegram account authorized");
      await loadTelegramDialogs();
    }
  } catch (err) {
    error.value = toMessage(err);
  } finally {
    telegramLoading.value = false;
  }
}

async function loadTelegramDialogs() {
  error.value = "";
  clearNotice();
  telegramLoading.value = true;
  try {
    telegramDialogs.value = await api<TelegramDialog[]>("/api/telegram/dialogs?limit=100");
    showNotice("Telegram dialogs loaded");
  } catch (err) {
    error.value = toMessage(err);
  } finally {
    telegramLoading.value = false;
  }
}

function telegramDialogSaved(dialog: TelegramDialog) {
  return sources.value.some((source) => source.type === "telegram" && source.url === dialog.source_url);
}

async function saveTelegramDialog(dialog: TelegramDialog) {
  if (telegramDialogSaved(dialog)) {
    return;
  }
  error.value = "";
  clearNotice();
  telegramSavingSourceURL.value = dialog.source_url;
  try {
    await api<Source>("/api/telegram/sources", {
      method: "POST",
      body: JSON.stringify({ dialog })
    });
    showNotice("Telegram source saved");
    await loadLibrary();
  } catch (err) {
    error.value = toMessage(err);
  } finally {
    telegramSavingSourceURL.value = "";
  }
}

function adapterBrowseCoverSrc(item: AdapterListItem) {
  if (item.cover_image && !isExternalURL(item.cover_image) && !item.cover_image.startsWith("telegram://")) {
    return item.cover_image;
  }
  if (!item.preview_url || !["f95zone", "telegram"].includes(item.adapter_id)) {
    return "";
  }
  const params = new URLSearchParams({ preview_url: item.preview_url });
  const proxyURL = adapterBrowseDraft.proxy_url.trim() || importDraft.proxy_url.trim();
  if (proxyURL) {
    params.set("proxy_url", proxyURL);
  }
  return `/api/adapters/${item.adapter_id}/browse-cover?${params.toString()}`;
}

function prepareAdapterCoverStates(page: AdapterBrowsePage | null) {
  if (!page) {
    return;
  }
  for (const item of page.items) {
    const src = adapterBrowseCoverSrc(item);
    if (!src || adapterCoverStates[src] === "loaded") {
      continue;
    }
    adapterCoverStates[src] = "loading";
  }
}

function adapterBrowseCoverStatus(item: AdapterListItem): CoverLoadState | "missing" {
  const src = adapterBrowseCoverSrc(item);
  if (!src) {
    return "missing";
  }
  return adapterCoverStates[src] ?? "loading";
}

function markAdapterCoverLoaded(src: string) {
  if (src) {
    adapterCoverStates[src] = "loaded";
  }
}

function markAdapterCoverFailed(src: string) {
  if (src) {
    adapterCoverStates[src] = "failed";
  }
}

function adapterBrowsePreviewImages() {
  return (
    adapterBrowsePage.value?.items
      .map((item) => adapterBrowseCoverSrc(item))
      .filter((src) => src && adapterCoverStates[src] === "loaded") ?? []
  );
}

function openAdapterBrowseCover(item: AdapterListItem) {
  const src = adapterBrowseCoverSrc(item);
  if (!src || adapterCoverStates[src] !== "loaded") {
    return;
  }
  openImagePreview(src, adapterBrowsePreviewImages());
}

function isExternalURL(value: string) {
  return /^https?:\/\//i.test(value);
}

async function createGameFromItem(item: SourceItem) {
  if (creatingGameItemId.value === item.id) {
    return;
  }
  error.value = "";
  clearNotice();
  creatingGameItemId.value = item.id;
  try {
    const result = await api<{ game: Game; item: SourceItem }>(
      `/api/source-items/${item.id}/create-game`,
      { method: "POST", body: "{}" }
    );
    upsertGameInState(result.game);
    upsertSourceItemInState(result.item);
    selectedItem.value = result.item;
    selectGame(result.game);
    view.value = "games";
    showNotice("Game created");
    refreshLibraryInBackground();
  } catch (err) {
    error.value = toMessage(err);
  } finally {
    creatingGameItemId.value = null;
  }
}

async function matchItem(item: SourceItem) {
  if (!canApplyMatch.value) {
    return;
  }
  const gameId = Number(matchGameId.value);
  const wasMatched = Boolean(item.matched_game_id);
  await matchSourceItemToGame(item, gameId, wasMatched ? "Match updated" : "Matched");
}

async function matchSourceItemToGame(item: SourceItem, gameId: number, message = "Matched") {
  if (matchingItemId.value === item.id) {
    return;
  }
  error.value = "";
  clearNotice();
  matchingItemId.value = item.id;
  try {
    const updated = await api<SourceItem>(`/api/source-items/${item.id}/match`, {
      method: "POST",
      body: JSON.stringify({ game_id: gameId })
    });
    upsertSourceItemInState(updated);
    selectedItem.value = updated;
    resetMatchPicker();
    matchEditing.value = false;
    showNotice(message);
    refreshLibraryInBackground();
  } catch (err) {
    error.value = toMessage(err);
  } finally {
    matchingItemId.value = null;
  }
}

async function linkSourceItemToSelectedGame(item: SourceItem) {
  if (!selectedGame.value) {
    return;
  }
  await matchSourceItemToGame(item, selectedGame.value.id, "Source linked");
  selectedGameTab.value = "sources";
}

async function unlinkSourceItem(item: SourceItem) {
  if (unlinkingItemId.value === item.id) {
    return;
  }
  error.value = "";
  clearNotice();
  unlinkingItemId.value = item.id;
  try {
    const updated = await api<SourceItem>(`/api/source-items/${item.id}/match`, {
      method: "POST",
      body: JSON.stringify({ game_id: null })
    });
    upsertSourceItemInState(updated);
    if (selectedItem.value?.id === item.id) {
      selectedItem.value = updated;
    }
    showNotice("Source unlinked");
    refreshLibraryInBackground();
  } catch (err) {
    error.value = toMessage(err);
  } finally {
    unlinkingItemId.value = null;
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

function resetMatchPicker() {
  matchGameId.value = "";
  matchGameQuery.value = "";
  matchPickerOpen.value = false;
  if (matchPickerCloseTimer !== undefined) {
    window.clearTimeout(matchPickerCloseTimer);
    matchPickerCloseTimer = undefined;
  }
}

function openMatchPicker() {
  if (matchPickerCloseTimer !== undefined) {
    window.clearTimeout(matchPickerCloseTimer);
    matchPickerCloseTimer = undefined;
  }
  matchPickerOpen.value = true;
}

function closeMatchPickerSoon() {
  if (matchPickerCloseTimer !== undefined) {
    window.clearTimeout(matchPickerCloseTimer);
  }
  matchPickerCloseTimer = window.setTimeout(() => {
    matchPickerOpen.value = false;
    matchPickerCloseTimer = undefined;
  }, 120);
}

function selectMatchGame(game: Game) {
  matchGameId.value = String(game.id);
  matchGameQuery.value = formatMatchGameOption(game);
  matchPickerOpen.value = false;
}

function selectFirstMatchGame() {
  if (selectedMatchGame.value && matchGameQuery.value === formatMatchGameOption(selectedMatchGame.value)) {
    matchPickerOpen.value = false;
    return;
  }
  const game = limitedMatchGames.value[0];
  if (game) {
    selectMatchGame(game);
  }
}

function clearMatchGameSelection() {
  matchGameId.value = "";
  matchGameQuery.value = "";
  openMatchPicker();
}

function handleMatchGameInput() {
  if (selectedMatchGame.value && matchGameQuery.value !== formatMatchGameOption(selectedMatchGame.value)) {
    matchGameId.value = "";
  }
  openMatchPicker();
}

function startMatchEdit() {
  matchEditing.value = true;
  resetMatchPicker();
}

function cancelMatchEdit() {
  matchEditing.value = false;
  resetMatchPicker();
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
    title: sourceItemTitle(item),
    firstMessage: "Delete this source record?",
    secondMessage: "This removes the raw capture and unreferenced attachments from disk."
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
      showNotice("Source record deleted");
      selectedItem.value = null;
      resetMatchPicker();
    }
    pendingDelete.value = null;
    await loadAll();
    if (target.kind === "item" && unmatchedSourceItems.value.length > 0) {
      selectItem(unmatchedSourceItems.value[0]);
    }
  } catch (err) {
    error.value = toMessage(err);
  } finally {
    deleting.value = false;
  }
}

function selectItem(item: SourceItem) {
  selectedItem.value = item;
  resetMatchPicker();
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
    view.value = "review";
  }
}

function isTaskActive(task: Task) {
  return task.status === "queued" || task.status === "running";
}

function canRetryTask(task: Task) {
  return task.status === "failed" && (task.kind === "import:f95zone" || task.kind === "import:telegram");
}

function isTaskRetried(task: Task) {
  return Number.isFinite(task.result_json?.retried_by_task_id);
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

function compareSourceItemsByUpdatedAt(left: SourceItem, right: SourceItem) {
  const leftTime = Date.parse(left.updated_at || left.fetched_at || left.created_at);
  const rightTime = Date.parse(right.updated_at || right.fetched_at || right.created_at);
  return rightTime - leftTime;
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
  return items.sort(compareSourceItemsByUpdatedAt);
}

function sourceItemLabel(item: SourceItem) {
  const source = item.source_id ? sources.value.find((entry) => entry.id === item.source_id) : null;
  return source?.name || item.source_type || "Unknown adapter";
}

function sourceItemTitle(item: SourceItem) {
  return item.title || item.parsed_json?.fields?.game_name || item.parsed_json?.inferred?.game_title || "Untitled source";
}

function sourceItemSubtitle(item: SourceItem) {
  const parts = [sourceItemLabel(item), item.external_id || "no external id", item.status].filter(Boolean);
  return parts.join(" · ");
}

function sourceItemRawHref(item: SourceItem) {
  return item.raw_content_path ? `/api/source-items/${item.id}/raw` : "";
}

function openSourceRecord(item: SourceItem) {
  selectItem(item);
  view.value = "review";
}

function openReviewQueue() {
  if (unmatchedSourceItems.value.length > 0) {
    selectItem(unmatchedSourceItems.value[0]);
  } else if (selectedItem.value?.matched_game_id) {
    selectedItem.value = null;
    resetMatchPicker();
    matchEditing.value = false;
  }
  view.value = "review";
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
  const hidden = new Set([
    "overview",
    "story",
    "description",
    "changelog",
    "change log",
    "download",
    "downloads",
    "游戏介绍",
    "介绍",
    "故事梗概",
    "剧情梗概",
    "更新介绍",
    "更新内容",
    "更新日志"
  ]);
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
  return availableAdapters.value.find((adapter) => adapter.id === adapterID)?.name ?? adapterID;
}

function formatMatchGameOption(game: Game) {
  return `${game.title}${game.current_version ? ` · ${game.current_version}` : ""}`;
}

function gameMatchesQuery(game: Game, needle: string) {
  return (
    game.title.toLowerCase().includes(needle) ||
    game.current_version.toLowerCase().includes(needle) ||
    game.aliases.some((alias) => alias.toLowerCase().includes(needle)) ||
    String(game.id).includes(needle)
  );
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

function hasAdapterAuth(profile: AuthProfile | null | undefined, adapter: Adapter) {
  if (!profile) {
    return false;
  }
  const requiredCookies: readonly string[] = adapter.auth_cookie_names ?? [];
  if (requiredCookies.length === 0) {
    return profile.cookie_count > 0;
  }
  const names = new Set((profile.cookies ?? []).map((cookie) => cookie.name));
  return requiredCookies.some((name) => names.has(name));
}

function missingAdapterAuthCookies(profile: AuthProfile | null | undefined, adapter: Adapter) {
  const requiredCookies: readonly string[] = adapter.auth_cookie_names ?? [];
  if (!profile || requiredCookies.length === 0) {
    return [];
  }
  const names = new Set((profile.cookies ?? []).map((cookie) => cookie.name));
  return requiredCookies.filter((name) => !names.has(name));
}

function adapterAuthState(profile: AuthProfile | null | undefined, adapter: Adapter) {
  if (!profile) {
    return "missing";
  }
  return hasAdapterAuth(profile, adapter) ? "authorized" : "incomplete";
}

function adapterAuthStatusClass(profile: AuthProfile | null | undefined, adapter: Adapter) {
  const state = adapterAuthState(profile, adapter);
  if (state === "authorized") return "succeeded";
  if (state === "incomplete") return "warning";
  return "failed";
}

function adapterAuthStatusLabel(profile: AuthProfile | null | undefined, adapter: Adapter) {
  return adapterAuthState(profile, adapter);
}

function clearImportState() {
  Object.assign(importDraft, importDraftDefaults);
  localStorage.removeItem(importDraftStorageKey);
  lastImportTaskId.value = null;
  persistLastImportTaskId();
  showNotice("Import cleared");
}

function dismissRecentImportTask() {
  lastImportTaskId.value = null;
  persistLastImportTaskId();
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

function restoreAdapterBrowseState() {
  const savedState = localStorage.getItem(adapterBrowseStorageKey);
  if (!savedState) {
    return;
  }
  restoringAdapterBrowseState = true;
  try {
    const state = JSON.parse(savedState) as {
      selected_adapter_id?: unknown;
      import_mode?: unknown;
      draft?: Partial<typeof adapterBrowseDraftDefaults>;
      page?: AdapterBrowsePage | null;
      telegram_source_ids?: unknown;
    };
    if (typeof state.selected_adapter_id === "string" && state.selected_adapter_id) {
      selectedAdapterId.value = state.selected_adapter_id;
    }
    if (state.import_mode === "browse" || state.import_mode === "direct") {
      importMode.value = state.import_mode;
    }
    const draft = state.draft ?? {};
    Object.assign(adapterBrowseDraft, {
      preset_id: typeof draft.preset_id === "string" ? draft.preset_id : adapterBrowseDraftDefaults.preset_id,
      url: typeof draft.url === "string" ? draft.url : adapterBrowseDraftDefaults.url,
      filter_url: typeof draft.filter_url === "string" ? draft.filter_url : adapterBrowseDraftDefaults.filter_url,
      page: typeof draft.page === "number" && Number.isFinite(draft.page) ? draft.page : adapterBrowseDraftDefaults.page,
      limit: typeof draft.limit === "number" && Number.isFinite(draft.limit) ? draft.limit : adapterBrowseDraftDefaults.limit,
      offset_id: typeof draft.offset_id === "number" && Number.isFinite(draft.offset_id) ? draft.offset_id : adapterBrowseDraftDefaults.offset_id,
      search: typeof draft.search === "string" ? draft.search : adapterBrowseDraftDefaults.search,
      sort: typeof draft.sort === "string" ? draft.sort : adapterBrowseDraftDefaults.sort,
      proxy_url: typeof draft.proxy_url === "string" ? draft.proxy_url : adapterBrowseDraftDefaults.proxy_url
    });
    if (Array.isArray(state.telegram_source_ids)) {
      selectedTelegramSourceIds.value = state.telegram_source_ids
        .map((value) => Number(value))
        .filter((value) => Number.isFinite(value) && value > 0);
    }
    adapterBrowsePage.value = state.page && Array.isArray(state.page.items) ? state.page : null;
    prepareAdapterCoverStates(adapterBrowsePage.value);
  } catch {
    localStorage.removeItem(adapterBrowseStorageKey);
  } finally {
    restoringAdapterBrowseState = false;
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

function persistAdapterBrowseState() {
  if (restoringAdapterBrowseState) {
    return;
  }
  try {
    localStorage.setItem(
      adapterBrowseStorageKey,
      JSON.stringify({
        selected_adapter_id: selectedAdapterId.value,
        import_mode: importMode.value,
        draft: adapterBrowseDraft,
        telegram_source_ids: selectedTelegramSourceIds.value,
        page: adapterBrowsePage.value
      })
    );
  } catch (err) {
    error.value = `Could not save browse state: ${toMessage(err)}`;
  }
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

watch(adapterBrowseDraft, persistAdapterBrowseState, { deep: true });

watch(adapterBrowsePage, persistAdapterBrowseState, { deep: true });

watch(selectedAdapterId, persistAdapterBrowseState);

watch(matchGameQuery, () => {
  if (selectedMatchGame.value && matchGameQuery.value !== formatMatchGameOption(selectedMatchGame.value)) {
    matchGameId.value = "";
  }
});

watch(selectedTelegramSourceIds, () => {
  adapterBrowseDraft.offset_id = 0;
  if (selectedTelegramSourceIds.value.length !== 1) {
    adapterBrowseDraft.search = "";
  }
  if (selectedAdapterId.value === "telegram") {
    adapterBrowsePage.value = null;
  }
  persistAdapterBrowseState();
}, { deep: true });

watch(selectedAdapterId, (adapterID) => {
  if (adapterID === "telegram") {
    if (importMode.value === "direct") {
      importMode.value = "browse";
    }
    void loadTelegramStatus();
  }
});

watch(importMode, persistAdapterBrowseState);

watch(
  () => adapterBrowseDraft.preset_id,
  (next, previous) => {
    if (!restoringAdapterBrowseState && next !== previous) {
      adapterBrowseDraft.url = "";
      adapterBrowseDraft.filter_url = "";
      adapterBrowseDraft.page = 1;
      adapterBrowsePage.value = null;
    }
  }
);

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
  restoreAdapterBrowseState();
  void loadAll();
  if (selectedAdapterId.value === "telegram") {
    void loadTelegramStatus();
  }
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
  if (matchPickerCloseTimer !== undefined) {
    window.clearTimeout(matchPickerCloseTimer);
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
        <button :class="{ active: view === 'review' }" title="Review" @click="openReviewQueue">
          <Icon name="file-search" :size="18" />
          <span>Review</span>
          <small v-if="reviewItemCount" class="nav-badge">{{ reviewItemCount }}</small>
        </button>
        <button :class="{ active: view === 'tasks' }" title="Tasks" @click="view = 'tasks'">
          <Icon name="activity" :size="18" />
          <span>Tasks</span>
          <small v-if="activeTasks.length" class="nav-badge">{{ activeTasks.length }}</small>
        </button>
      </nav>
    </aside>

    <main class="main">
      <section v-if="view === 'games'" class="workspace two-column">
        <div class="list-pane">
          <div class="pane-title">
            <h1>Games</h1>
            <button class="icon-button" title="New game" @click="newGame">
              <Icon name="plus" :size="18" />
            </button>
          </div>
          <div class="list-filter-row">
            <div class="searchbox">
              <Icon name="search" :size="17" />
              <input v-model="query" type="search" placeholder="Search games" />
            </div>
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
              <button class="primary" :disabled="savingGame" @click="saveGame">
                <Icon name="save" :size="17" />
                <span>{{ savingGame ? "Saving" : "Save" }}</span>
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

          <div class="subtab-row game-detail-tabs" role="tablist" aria-label="Game detail">
            <button
              type="button"
              class="subtab-button"
              :class="{ active: selectedGameTab === 'details' }"
              role="tab"
              :aria-selected="selectedGameTab === 'details'"
              @click="selectedGameTab = 'details'"
            >
              <span>Details</span>
              <small>Merged fields</small>
            </button>
            <button
              type="button"
              class="subtab-button"
              :class="{ active: selectedGameTab === 'sources' }"
              role="tab"
              :aria-selected="selectedGameTab === 'sources'"
              @click="selectedGameTab = 'sources'"
            >
              <span>Sources</span>
              <small>{{ selectedGameSourceItems.length }} linked</small>
            </button>
          </div>

          <template v-if="selectedGameTab === 'details'">
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
                <small>Open source record to retry</small>
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
            <button v-if="selectedGameSourceItem" class="secondary" @click="openSourceRecord(selectedGameSourceItem)">
              <Icon name="file-search" :size="17" />
              <span>Open source record</span>
            </button>
            <a
              v-if="selectedGameSourceItem && sourceItemRawHref(selectedGameSourceItem)"
              class="open-link"
              :href="sourceItemRawHref(selectedGameSourceItem)"
              target="_blank"
            >
              <Icon name="file-search" :size="17" />
              <span>Raw capture</span>
            </a>
            <a v-if="selectedGameSourceItem?.raw_url" class="open-link" :href="selectedGameSourceItem.raw_url" target="_blank">
              <Icon name="eye" :size="17" />
              <span>Open source</span>
            </a>
          </div>
          </template>

          <template v-else>
            <div class="source-tab-header">
              <div>
                <h3>Linked Source Records</h3>
                <p>Adapter transcripts stay attached here; merged fields are shown in Details.</p>
              </div>
              <div class="button-row">
                <button class="secondary" @click="sourceLinkOpen = !sourceLinkOpen">
                  <Icon name="cable" :size="17" />
                  <span>{{ sourceLinkOpen ? "Close linker" : "Link source" }}</span>
                </button>
                <button class="primary" @click="view = 'import'">
                  <Icon name="download" :size="17" />
                  <span>Import</span>
                </button>
              </div>
            </div>

            <div v-if="sourceLinkOpen" class="source-link-panel">
              <div class="adapter-browser-summary">
                <div>
                  <strong>Review queue</strong>
                  <small>{{ unmatchedSourceItems.length }} unlinked source records</small>
                </div>
                <button class="secondary" @click="openReviewQueue">
                  <Icon name="file-search" :size="17" />
                  <span>Open Review</span>
                </button>
              </div>
              <div v-if="unmatchedSourceItems.length" class="source-link-list">
                <div v-for="item in unmatchedSourceItems" :key="item.id" class="source-record-row">
                  <span class="item-state" :class="item.status"></span>
                  <span class="row-main">
                    <strong>{{ sourceItemTitle(item) }}</strong>
                    <small>{{ sourceItemSubtitle(item) }}</small>
                  </span>
                  <div class="row-actions">
                    <button class="secondary" @click="openSourceRecord(item)">
                      <Icon name="eye" :size="16" />
                      <span>Inspect</span>
                    </button>
                    <button
                      class="primary"
                      :disabled="matchingItemId === item.id"
                      @click="linkSourceItemToSelectedGame(item)"
                    >
                      <Icon name="cable" :size="16" />
                      <span>{{ matchingItemId === item.id ? "Linking" : "Link" }}</span>
                    </button>
                  </div>
                </div>
              </div>
              <div v-else class="task-empty-row">
                <Icon name="check" :size="18" />
                <strong>No unlinked source records</strong>
              </div>
            </div>

            <div v-if="selectedGameSourceItems.length" class="source-record-list">
              <div v-for="item in selectedGameSourceItems" :key="item.id" class="source-record-card">
                <div class="source-record-card-head">
                  <span class="item-state" :class="item.status"></span>
                  <span class="row-main">
                    <strong>{{ sourceItemTitle(item) }}</strong>
                    <small>{{ sourceItemSubtitle(item) }}</small>
                  </span>
                  <span class="status-pill succeeded">{{ sourceItemLabel(item) }}</span>
                </div>
                <div class="meta-grid compact-meta-grid">
                  <div>
                    <span>Fetched</span>
                    <strong>{{ formatTimestamp(item.fetched_at, "Unknown") }}</strong>
                  </div>
                  <div>
                    <span>Updated</span>
                    <strong>{{ formatTimestamp(item.updated_at, "Unknown") }}</strong>
                  </div>
                  <div>
                    <span>External ID</span>
                    <strong>{{ item.external_id || "None" }}</strong>
                  </div>
                  <div>
                    <span>Raw capture</span>
                    <strong>{{ item.raw_content_path ? "Saved" : "Not saved" }}</strong>
                  </div>
                </div>
                <div class="result-actions">
                  <button class="secondary" @click="openSourceRecord(item)">
                    <Icon name="file-search" :size="17" />
                    <span>Inspect</span>
                  </button>
                  <a v-if="sourceItemRawHref(item)" class="open-link" :href="sourceItemRawHref(item)" target="_blank">
                    <Icon name="file-search" :size="17" />
                    <span>Raw capture</span>
                  </a>
                  <a v-if="item.raw_url" class="open-link" :href="item.raw_url" target="_blank">
                    <Icon name="eye" :size="17" />
                    <span>Open source</span>
                  </a>
                  <button
                    class="secondary"
                    :disabled="unlinkingItemId === item.id"
                    @click="unlinkSourceItem(item)"
                  >
                    <Icon name="x" :size="17" />
                    <span>{{ unlinkingItemId === item.id ? "Unlinking" : "Unlink" }}</span>
                  </button>
                  <button class="danger" @click="requestDeleteSourceItem(item)">
                    <Icon name="trash" :size="17" />
                    <span>Delete</span>
                  </button>
                </div>
              </div>
            </div>

            <div v-else class="empty-detail source-empty-detail">
              <Icon name="file-search" :size="24" />
              <strong>No sources linked</strong>
              <div class="button-row">
                <button class="secondary" @click="sourceLinkOpen = true">
                  <Icon name="cable" :size="17" />
                  <span>Link from Review</span>
                </button>
                <button class="primary" @click="view = 'import'">
                  <Icon name="download" :size="17" />
                  <span>Import source</span>
                </button>
              </div>
            </div>
          </template>
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
              v-for="adapter in availableAdapters"
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
                <strong>{{ selectedAdapter.without_bridge }}</strong>
              </div>
            </dl>
          </div>

            <div v-if="selectedAdapter.id === 'telegram'" class="detail-pane auth-pane telegram-auth-pane">
              <div class="pane-title">
                <h2>Telegram Account</h2>
                <span class="status-pill" :class="telegramAuthClass">{{ telegramAuthLabel }}</span>
              </div>

              <dl class="meta-grid">
                <div>
                  <span>Account</span>
                  <strong>{{ telegramAccountName }}</strong>
                </div>
                <div>
                  <span>Phone</span>
                  <strong>{{ telegramStatus.phone || telegramAuthDraft.phone || "Not saved" }}</strong>
                </div>
                <div>
                  <span>User ID</span>
                  <strong>{{ telegramStatus.user_id || "Unknown" }}</strong>
                </div>
                <div>
                  <span>Groups loaded</span>
                  <strong>{{ telegramDialogs.length }}</strong>
                </div>
                <div class="wide">
                  <span>API setup</span>
                  <strong>Use your own Telegram API ID/hash from my.telegram.org, then sign in with a phone code.</strong>
                </div>
                <div v-if="telegramStatus.error" class="wide">
                  <span>Status error</span>
                  <strong>{{ telegramStatus.error }}</strong>
                </div>
              </dl>

              <div class="telegram-auth-grid">
                <label>
                  <span>API ID</span>
                  <input v-model="telegramAuthDraft.api_id" inputmode="numeric" placeholder="Stored or env if blank" />
                </label>
                <label>
                  <span>API Hash</span>
                  <input v-model="telegramAuthDraft.api_hash" type="password" placeholder="Stored or env if blank" />
                </label>
                <label>
                  <span>Phone</span>
                  <input v-model="telegramAuthDraft.phone" type="tel" placeholder="+1..." />
                </label>
                <div class="form-action-cell">
                  <button class="secondary" :disabled="telegramLoading" @click="sendTelegramCode">
                    <Icon name="refresh" :size="17" />
                    <span>Send code</span>
                  </button>
                </div>
                <label>
                  <span>Login code</span>
                  <input v-model="telegramAuthDraft.code" inputmode="numeric" autocomplete="one-time-code" />
                </label>
                <label>
                  <span>2FA password</span>
                  <input v-model="telegramAuthDraft.password" type="password" :placeholder="telegramPasswordRequired ? 'Required' : 'If enabled'" />
                </label>
                <div class="form-action-cell">
                  <button class="primary" :disabled="telegramLoading || !canSubmitTelegramSignIn" @click="signInTelegram">
                    <Icon name="check" :size="17" />
                    <span>Sign in</span>
                  </button>
                </div>
              </div>

              <div class="telegram-dialog-toolbar">
                <div class="searchbox">
                  <Icon name="search" :size="17" />
                  <input v-model="telegramDialogQuery" type="search" placeholder="Filter loaded groups" />
                </div>
                <button class="primary" :disabled="telegramLoading || !telegramStatus.authorized" @click="loadTelegramDialogs">
                  <Icon name="refresh" :size="17" />
                  <span>{{ telegramLoading ? "Loading" : "Load groups" }}</span>
                </button>
              </div>

              <div v-if="filteredTelegramDialogs.length" class="telegram-dialog-list">
                <div v-for="dialog in filteredTelegramDialogs" :key="dialog.source_url" class="telegram-dialog-row">
                  <span class="source-dot"></span>
                  <span class="row-main">
                    <strong>{{ dialog.title }}</strong>
                    <small>
                      {{ dialog.type }} · {{ dialog.username ? `@${dialog.username}` : dialog.peer_id }}
                      <template v-if="dialog.participants"> · {{ dialog.participants }} members</template>
                    </small>
                  </span>
                  <button
                    class="secondary"
                    :disabled="telegramDialogSaved(dialog) || telegramSavingSourceURL === dialog.source_url"
                    @click="saveTelegramDialog(dialog)"
                  >
                    <Icon :name="telegramDialogSaved(dialog) ? 'check' : 'save'" :size="17" />
                    <span>{{ telegramDialogSaved(dialog) ? "Saved" : telegramSavingSourceURL === dialog.source_url ? "Saving" : "Save source" }}</span>
                  </button>
                </div>
              </div>

              <div v-else class="auth-guide">
                <strong>{{ telegramStatus.authorized ? "No groups loaded" : "Telegram account is not authorized" }}</strong>
                <p>{{ telegramStatus.authorized ? "Load groups to choose Telegram sources." : "Send a login code, sign in, then load the groups and channels visible to your account." }}</p>
              </div>
            </div>

            <div v-else class="detail-pane auth-pane">
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

      <section v-else-if="view === 'import'" class="workspace import-workspace">
        <div class="import-mode-row" role="tablist" aria-label="Import mode">
          <button
            type="button"
            class="import-mode-button"
            :class="{ active: importMode === 'browse' }"
            role="tab"
            :aria-selected="importMode === 'browse'"
            @click="setImportMode('browse')"
          >
            <Icon name="globe" :size="17" />
            <span>Browse Source</span>
          </button>
          <button
            type="button"
            class="import-mode-button"
            :class="{ active: importMode === 'direct' }"
            role="tab"
            :aria-selected="importMode === 'direct'"
            :disabled="selectedAdapter.id !== 'f95zone'"
            @click="setImportMode('direct')"
          >
            <Icon name="link" :size="17" />
            <span>Direct URL</span>
          </button>
        </div>

        <div
          v-if="recentImportTask && isTaskActive(recentImportTask)"
          class="inline-task-panel import-task-panel"
          :class="recentImportTask.status"
        >
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
        <div v-else-if="recentImportTask" class="import-result-strip" :class="recentImportTask.status">
          <span class="task-state" :class="recentImportTask.status"></span>
          <span class="row-main">
            <strong>{{ taskResultTitle(recentImportTask) || recentImportTask.title }}</strong>
            <small>{{ recentImportTask.error || recentImportTask.message || recentImportTask.status }}</small>
          </span>
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
          <button class="icon-button" title="Dismiss import result" @click="dismissRecentImportTask">
            <Icon name="x" :size="17" />
          </button>
        </div>

        <div v-if="importMode === 'browse'" class="detail-pane adapter-browser-pane import-browser-pane">
          <div class="adapter-browser-controls">
            <label>
              <span>Adapter</span>
              <select :value="selectedAdapterId" @change="changeImportAdapter">
                <option v-for="adapter in availableAdapters" :key="adapter.id" :value="adapter.id">
                  {{ adapter.name }}
                </option>
              </select>
            </label>
            <template v-if="selectedAdapter.id !== 'telegram'">
              <label>
                <span>Source list</span>
                <select v-model="adapterBrowseDraft.preset_id" :disabled="!adapterBrowsePresets.length" @change="selectBrowsePreset(adapterBrowseDraft.preset_id)">
                  <option v-for="preset in adapterBrowsePresets" :key="preset.id" :value="preset.id">
                    {{ preset.label }}
                  </option>
                </select>
              </label>
              <label>
                <span>Page</span>
                <input
                  v-model.number="adapterBrowseDraft.page"
                  type="number"
                  min="1"
                  :disabled="!adapterBrowseCapabilities.pagination"
                  @keydown.enter.prevent="browseAdapterList()"
                />
              </label>
              <label>
                <span>Source filter</span>
                <select
                  :value="adapterBrowseDraft.filter_url"
                  :disabled="!adapterBrowseCapabilities.filter || adapterBrowseFilters.length === 0"
                  @change="selectBrowseFilterFromEvent"
                >
                  <option value="">None</option>
                  <option v-for="filter in adapterBrowseFilters" :key="filter.id" :value="filter.url">
                    {{ filter.label }}{{ filter.count ? ` (${filter.count})` : "" }}
                  </option>
                </select>
              </label>
              <label>
                <span>Source search</span>
                <input type="search" :disabled="!adapterBrowseCapabilities.search" :placeholder="adapterBrowseCapabilities.search ? 'Search source' : 'Disabled for this adapter'" />
              </label>
              <label>
                <span>Sort by</span>
                <select v-model="adapterBrowseDraft.sort" :disabled="!adapterBrowseCapabilities.sort">
                  <option value="">Source default</option>
                </select>
              </label>
              <label class="wide">
                <span>List URL</span>
                <input v-model="adapterBrowseDraft.url" type="url" :disabled="!adapterBrowseCapabilities.custom_url" placeholder="Use source list, source filter, or paste a list URL" />
              </label>
              <div class="form-action-cell">
                <button class="primary" :disabled="adapterBrowsing || !selectedAdapter.browse.enabled" @click="browseAdapterList()">
                  <Icon name="refresh" :size="17" />
                  <span>{{ adapterBrowsing ? "Loading" : "Load" }}</span>
                </button>
              </div>
            </template>
            <template v-else>
              <label>
                <span>Source search</span>
                <input
                  v-model="adapterBrowseDraft.search"
                  type="search"
                  :disabled="telegramSearchDisabled"
                  :placeholder="telegramSearchDisabled ? 'Select one group to search' : 'Search messages in selected group'"
                  @keydown.enter.prevent="browseAdapterList(true)"
                />
              </label>
              <label>
                <span>Messages</span>
                <input v-model.number="adapterBrowseDraft.limit" type="number" min="1" max="100" @keydown.enter.prevent="browseAdapterList(true)" />
              </label>
              <div class="form-action-cell">
                <button class="primary" :disabled="!canLoadTelegramMessages" @click="browseAdapterList(true)">
                  <Icon name="refresh" :size="17" />
                  <span>{{ adapterBrowsing ? "Loading" : "Load messages" }}</span>
                </button>
              </div>
            </template>
          </div>

          <div v-if="selectedAdapter.id === 'telegram'" class="telegram-source-picker">
            <div class="adapter-browser-summary">
              <div>
                <strong>Telegram groups</strong>
                <small>{{ selectedTelegramSources.length }} selected · {{ telegramSources.length }} saved</small>
              </div>
              <button class="secondary" @click="view = 'adapters'; selectedAdapterId = 'telegram'">
                <Icon name="cable" :size="17" />
                <span>Manage</span>
              </button>
            </div>
            <div v-if="telegramSources.length" class="telegram-source-list">
              <label v-for="source in telegramSources" :key="source.id" class="telegram-source-option">
                <input v-model="selectedTelegramSourceIds" type="checkbox" :value="source.id" />
                <span class="row-main">
                  <strong>{{ source.name }}</strong>
                  <small>{{ source.url }}</small>
                </span>
              </label>
            </div>
            <div v-else class="empty-detail">
              <strong>No Telegram sources saved</strong>
              <button class="secondary" @click="view = 'adapters'; selectedAdapterId = 'telegram'">
                <Icon name="cable" :size="17" />
                <span>Choose groups</span>
              </button>
            </div>
          </div>

          <div v-if="selectedAdapter.id !== 'telegram'" class="capability-note-row">
            <span v-if="!adapterBrowseCapabilities.search">{{ adapterBrowseCapabilities.search_note }}</span>
            <span v-if="!adapterBrowseCapabilities.sort">{{ adapterBrowseCapabilities.sort_note }}</span>
          </div>

          <div v-if="adapterBrowsePage" class="adapter-browser-summary">
            <div>
              <strong>{{ adapterBrowsePage.title || selectedAdapter.name }}</strong>
              <small>
                <template v-if="selectedAdapter.id === 'telegram'">
                  {{ adapterBrowsePage.items.length }} shown
                  <template v-if="adapterBrowseDraft.search"> · search "{{ adapterBrowseDraft.search }}"</template>
                </template>
                <template v-else>
                  Page {{ adapterBrowsePage.page }}{{ adapterBrowsePage.total_pages ? ` / ${adapterBrowsePage.total_pages}` : "" }} · {{ adapterBrowsePage.items.length }} shown
                </template>
                <template v-if="adapterBrowseLoadingCovers"> · {{ adapterBrowseLoadingCovers }} covers loading</template>
              </small>
            </div>
            <div class="button-row">
              <button v-if="selectedAdapter.id !== 'telegram'" class="secondary" :disabled="!adapterBrowsePage.prev_url || adapterBrowsing" @click="goBrowsePage(-1)">
                <Icon name="arrow-left" :size="17" />
                <span>Prev</span>
              </button>
              <button class="secondary" :disabled="!(selectedAdapter.id === 'telegram' ? telegramOlderOffsetID : adapterBrowsePage.next_url) || adapterBrowsing" @click="goBrowsePage(1)">
                <span>{{ selectedAdapter.id === "telegram" ? "Older" : "Next" }}</span>
                <Icon name="arrow-right" :size="17" />
              </button>
            </div>
          </div>

          <div v-if="adapterBrowsePage?.warnings?.length" class="warning-box">
            <p v-for="warning in adapterBrowsePage.warnings" :key="warning">{{ warning }}</p>
          </div>

          <div v-if="adapterBrowsePage" class="adapter-result-list">
            <div v-for="item in adapterBrowsePage.items" :key="item.url" class="adapter-result-row">
              <button
                class="adapter-result-cover"
                :class="[item.adapter_id === 'telegram' && !adapterBrowseCoverSrc(item) ? 'is-message' : '', `is-${adapterBrowseCoverStatus(item)}`]"
                type="button"
                title="Preview cover"
                :disabled="adapterBrowseCoverStatus(item) !== 'loaded'"
                @click="openAdapterBrowseCover(item)"
              >
                <img
                  v-if="adapterBrowseCoverSrc(item)"
                  :src="adapterBrowseCoverSrc(item)"
                  alt=""
                  draggable="false"
                  @dragstart.prevent
                  @load="markAdapterCoverLoaded(adapterBrowseCoverSrc(item))"
                  @error="markAdapterCoverFailed(adapterBrowseCoverSrc(item))"
                />
                <span v-if="adapterBrowseCoverStatus(item) === 'loading'" class="cover-loading">
                  <span class="cover-spinner" aria-hidden="true"></span>
                  <small>Loading</small>
                </span>
                <span v-else-if="adapterBrowseCoverStatus(item) === 'failed'" class="cover-loading">
                  <Icon name="image-off" :size="18" />
                  <small>Failed</small>
                </span>
                <Icon v-else-if="adapterBrowseCoverStatus(item) === 'missing'" name="image-off" :size="20" />
              </button>
              <div class="row-main">
                <strong>{{ item.title }}</strong>
                <small>
                  {{ item.author || "Unknown author" }}
                  <template v-if="item.latest_at"> · updated {{ formatTimestamp(item.latest_at, item.latest_at) }}</template>
                </small>
                <p v-if="item.summary" class="adapter-result-summary">{{ item.summary }}</p>
                <div v-if="item.prefixes?.length" class="tag-row">
                  <span v-for="tag in item.prefixes" :key="`${item.url}-${tag}`">{{ tag }}</span>
                </div>
                <small class="adapter-result-stats">
                  ID {{ item.external_id || "Unknown" }}
                  <template v-if="item.replies"> · {{ item.replies }} replies</template>
                  <template v-if="item.views"> · {{ item.views }} views</template>
                  <template v-if="item.rating"> · {{ item.rating }}★</template>
                </small>
              </div>
              <div class="row-actions">
                <a class="secondary" :href="item.url" target="_blank" rel="noreferrer">
                  <Icon name="external-link" :size="16" />
                  <span>Open</span>
                </a>
                <button class="primary" :disabled="!adapterBrowseCapabilities.import || !item.importable || loading" @click="importBrowseItem(item)">
                  <Icon name="download" :size="16" />
                  <span>Import</span>
                </button>
              </div>
            </div>
            <div v-if="adapterBrowsePage.items.length === 0" class="empty-detail">
              <strong>No rows returned by the source</strong>
            </div>
          </div>
          <div v-else class="empty-detail">
            <strong>{{ selectedAdapter.id === "telegram" ? "Choose sources and load Telegram messages" : "Load a source list to browse import candidates" }}</strong>
          </div>
        </div>

        <div v-else class="detail-pane direct-import-pane">
          <div class="pane-title">
            <h1>Direct URL Import</h1>
            <div class="button-row">
              <button class="secondary" title="Clear import state" @click="clearImportState">
                <Icon name="x" :size="17" />
                <span>Clear</span>
              </button>
              <button class="primary" :disabled="loading || !importDraft.url.trim()" @click="runImport">
                <Icon name="download" :size="17" />
                <span>{{ loading ? "Importing" : "Import" }}</span>
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
              <div
                v-for="task in completedTasks"
                :key="task.id"
                class="task-row-entry"
              >
                <button
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
                <button
                  v-if="canRetryTask(task)"
                  class="task-retry-button"
                  title="Retry import"
                  :disabled="retryingTaskId === task.id"
                  @click.stop="retryTask(task)"
                >
                  <Icon name="refresh" :size="16" />
                  <span>Retry</span>
                </button>
              </div>
            </template>
          </div>
        </div>

        <div v-if="selectedTask" class="detail-pane task-detail">
          <div class="pane-title">
            <h2>{{ selectedTask.title }}</h2>
            <div class="title-actions">
              <button
                v-if="canRetryTask(selectedTask)"
                class="secondary"
                :disabled="retryingTaskId === selectedTask.id"
                @click="retryTask(selectedTask)"
              >
                <Icon name="refresh" :size="17" />
                <span>Retry import</span>
              </button>
              <span class="status-pill" :class="selectedTask.status">{{ selectedTask.status }}</span>
            </div>
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
                <button class="secondary" @click="openSourceRecord(selectedTaskSourceItem)">
                  <Icon name="file-search" :size="17" />
                  <span>Inspect</span>
                </button>
                <a
                  v-if="sourceItemRawHref(selectedTaskSourceItem)"
                  class="open-link"
                  :href="sourceItemRawHref(selectedTaskSourceItem)"
                  target="_blank"
                >
                  <Icon name="file-search" :size="17" />
                  <span>Raw capture</span>
                </a>
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

      <section v-else class="workspace two-column review-layout">
        <div class="list-pane">
          <div class="pane-title">
            <h1>Review</h1>
            <small class="section-count">{{ reviewItemCount }}</small>
          </div>
          <div
            v-for="item in reviewSourceItems"
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
                <strong>{{ sourceItemTitle(item) }}</strong>
                <small>{{ sourceItemSubtitle(item) }}</small>
              </span>
            </button>
            <button class="row-delete" title="Delete source record" @click.stop="requestDeleteSourceItem(item)">
              <Icon name="trash" :size="16" />
            </button>
          </div>
          <div v-if="reviewSourceItems.length === 0" class="empty-state">
            <Icon name="file-search" :size="20" />
            <strong>No records to review</strong>
          </div>
        </div>

        <div v-if="selectedItem" class="detail-pane transcript-pane">
          <div class="pane-title">
            <h2>{{ sourceItemTitle(selectedItem) }}</h2>
            <div class="button-row">
              <button v-if="selectedMatchedGame" class="secondary" @click="openMatchedGame">
                <Icon name="eye" :size="17" />
                <span>Open game</span>
              </button>
              <button
                v-else
                class="secondary"
                :disabled="creatingGameItemId === selectedItem.id"
                @click="createGameFromItem(selectedItem)"
              >
                <Icon name="plus" :size="17" />
                <span>{{ creatingGameItemId === selectedItem.id ? "Creating" : "Create game" }}</span>
              </button>
              <button
                v-if="selectedMatchedGame"
                class="secondary"
                :disabled="unlinkingItemId === selectedItem.id"
                @click="unlinkSourceItem(selectedItem)"
              >
                <Icon name="x" :size="17" />
                <span>{{ unlinkingItemId === selectedItem.id ? "Unlinking" : "Unlink" }}</span>
              </button>
              <button class="secondary" :disabled="mediaRetrying" @click="retryItemImages(selectedItem)">
                <Icon name="refresh" :size="17" />
                <span>{{ mediaRetrying ? "Retrying" : "Retry images" }}</span>
              </button>
            </div>
          </div>

          <div class="match-bar">
            <span v-if="selectedMatchedGame" class="match-current">
              Linked to <strong>{{ selectedMatchedGame.title }}</strong>
            </span>
            <template v-if="!selectedMatchedGame || matchEditing">
              <div v-if="availableMatchGames.length" class="match-picker">
                <div class="match-combobox" @focusin="openMatchPicker" @focusout="closeMatchPickerSoon">
                  <div
                    class="match-combobox-field"
                    :class="{ selected: Boolean(selectedMatchGame) }"
                    @mousedown="openMatchPicker"
                  >
                    <Icon name="search" :size="16" />
                    <input
                      v-model="matchGameQuery"
                      type="text"
                      role="combobox"
                      autocomplete="off"
                      :aria-expanded="matchPickerOpen"
                      placeholder="Search or select game"
                      @focus="openMatchPicker"
                      @input="handleMatchGameInput"
                      @keydown.enter.prevent="selectFirstMatchGame"
                      @keydown.escape.prevent="matchPickerOpen = false"
                    />
                    <button
                      v-if="matchGameQuery"
                      class="match-combobox-clear"
                      type="button"
                      title="Clear selection"
                      @mousedown.prevent
                      @click="clearMatchGameSelection"
                    >
                      <Icon name="x" :size="14" />
                    </button>
                  </div>

                  <div v-if="matchPickerOpen" class="match-combobox-menu" role="listbox">
                    <button
                      v-for="game in limitedMatchGames"
                      :key="game.id"
                      class="match-combobox-option"
                      :class="{ selected: selectedMatchGame?.id === game.id }"
                      type="button"
                      role="option"
                      :aria-selected="selectedMatchGame?.id === game.id"
                      @mousedown.prevent="selectMatchGame(game)"
                    >
                      <span class="row-main">
                        <strong>{{ game.title }}</strong>
                        <small>
                          {{ game.current_version || "No version" }}
                          <template v-if="game.aliases.length"> · {{ game.aliases.slice(0, 2).join(", ") }}</template>
                          · ID {{ game.id }}
                        </small>
                      </span>
                      <Icon v-if="selectedMatchGame?.id === game.id" name="check" :size="16" />
                    </button>
                    <span v-if="limitedMatchGames.length === 0" class="match-combobox-empty">
                      No games match this search.
                    </span>
                    <small class="match-combobox-footer">
                      Showing {{ limitedMatchGames.length }} of {{ matchingMatchGames.length }}
                      <template v-if="hiddenMatchGameCount"> · {{ hiddenMatchGameCount }} more, keep typing</template>
                    </small>
                  </div>
                </div>
              </div>
              <span v-else class="match-empty">
                {{ selectedMatchedGame ? "No other games available" : "No games available to match" }}
              </span>
              <button
                v-if="availableMatchGames.length"
                class="secondary"
                :disabled="!canApplyMatch || matchingItemId === selectedItem.id"
                @click="matchItem(selectedItem)"
              >
                <Icon name="cable" :size="17" />
                <span>{{ matchingItemId === selectedItem.id ? "Linking" : selectedMatchedGame ? "Apply" : "Link" }}</span>
              </button>
              <button v-if="matchEditing" class="secondary" @click="cancelMatchEdit">
                <Icon name="x" :size="17" />
                <span>Cancel</span>
              </button>
            </template>
            <button v-else-if="availableMatchGames.length" class="secondary" @click="startMatchEdit">
              <Icon name="cable" :size="17" />
              <span>Change link</span>
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
              <span>Adapter</span>
              <strong>{{ sourceItemLabel(selectedItem) }}</strong>
            </div>
            <div>
              <span>Status</span>
              <strong>{{ selectedItem.status }}</strong>
            </div>
            <div>
              <span>External ID</span>
              <strong>{{ selectedItem.external_id || "None" }}</strong>
            </div>
            <div>
              <span>Fetched</span>
              <strong>{{ formatTimestamp(selectedItem.fetched_at, "Unknown") }}</strong>
            </div>
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
            <a v-if="sourceItemRawHref(selectedItem)" class="open-link" :href="sourceItemRawHref(selectedItem)" target="_blank">
              <Icon name="file-search" :size="17" />
              <span>Raw capture</span>
            </a>
            <a v-if="selectedItem.raw_url" class="open-link" :href="selectedItem.raw_url" target="_blank">
              <Icon name="eye" :size="17" />
              <span>Open source</span>
            </a>
          </div>
        </div>
        <div v-else class="detail-pane">
          <div class="empty-detail">
            <Icon name="file-search" :size="24" />
            <strong>No source record selected</strong>
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
