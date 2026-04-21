const tokenKey = "ttc-client-token";
const PIECE_CODES = {
  white: { pawn: "WP", rook: "WR", bishop: "WB", knight: "WN" },
  black: { pawn: "BP", rook: "BR", bishop: "BB", knight: "BN" },
};

// SVG chess pieces by Colin M.L. Burnett (Wikimedia Commons, CC BY-SA 3.0)
const PIECE_SVGS = {
  pawn: '<svg viewBox="0 0 45 45"><path d="m 22.5,9 c -2.21,0 -4,1.79 -4,4 0,0.89 0.29,1.71 0.78,2.38 C 17.33,16.5 16,18.59 16,21 c 0,2.03 0.94,3.84 2.41,5.03 C 15.41,27.09 11,31.58 11,39.5 H 34 C 34,31.58 29.59,27.09 26.59,26.03 28.06,24.84 29,23.03 29,21 29,18.59 27.67,16.5 25.72,15.38 26.21,14.71 26.5,13.89 26.5,13 c 0,-2.21 -1.79,-4 -4,-4 z" style="fill:currentColor;stroke:var(--piece-stroke);stroke-width:1;stroke-linecap:round;stroke-linejoin:miter"/></svg>',
  rook: '<svg viewBox="0 0 45 45"><g style="fill:currentColor;stroke:var(--piece-stroke);stroke-width:1;stroke-linecap:round;stroke-linejoin:round" transform="translate(0,0.3)"><path d="M 9,39 L 36,39 L 36,36 L 9,36 L 9,39 z" style="stroke-linecap:butt"/><path d="M 12,36 L 12,32 L 33,32 L 33,36 L 12,36 z" style="stroke-linecap:butt"/><path d="M 11,14 L 11,9 L 15,9 L 15,11 L 20,11 L 20,9 L 25,9 L 25,11 L 30,11 L 30,9 L 34,9 L 34,14" style="stroke-linecap:butt"/><path d="M 34,14 L 31,17 L 14,17 L 11,14"/><path d="M 31,17 L 31,29.5 L 14,29.5 L 14,17" style="stroke-linecap:butt;stroke-linejoin:miter"/><path d="M 31,29.5 L 32.5,32 L 12.5,32 L 14,29.5"/><path d="M 11,14 L 34,14" style="fill:none;stroke-linejoin:miter"/></g></svg>',
  bishop: '<svg viewBox="0 0 45 45"><g style="fill:currentColor;stroke:var(--piece-stroke);stroke-width:1;stroke-linecap:round;stroke-linejoin:round" transform="translate(0,0.6)"><path d="M 9,36 C 12.39,35.03 19.11,36.43 22.5,34 C 25.89,36.43 32.61,35.03 36,36 C 36,36 37.65,36.54 39,38 C 38.32,38.97 37.35,38.99 36,38.5 C 32.61,37.53 25.89,38.96 22.5,37.5 C 19.11,38.96 12.39,37.53 9,38.5 C 7.65,38.99 6.68,38.97 6,38 C 7.35,36.54 9,36 9,36 z"/><path d="M 15,32 C 17.5,34.5 27.5,34.5 30,32 C 30.5,30.5 30,30 30,30 C 30,27.5 27.5,26 27.5,26 C 33,24.5 33.5,14.5 22.5,10.5 C 11.5,14.5 12,24.5 17.5,26 C 17.5,26 15,27.5 15,30 C 15,30 14.5,30.5 15,32 z"/><path d="M 25 8 A 2.5 2.5 0 1 1 20,8 A 2.5 2.5 0 1 1 25 8 z"/><path d="M 17.5,26 L 27.5,26 M 15,30 L 30,30 M 22.5,15.5 L 22.5,20.5 M 20,18 L 25,18" style="fill:none;stroke-linejoin:miter"/></g></svg>',
  knight: '<svg viewBox="0 0 45 45"><g style="fill:currentColor;stroke:var(--piece-stroke);stroke-width:1;stroke-linecap:round;stroke-linejoin:round" transform="translate(0,0.3)"><path d="M 22,10 C 32.5,11 38.5,18 38,39 L 15,39 C 15,30 25,32.5 23,18"/><path d="M 24,18 C 24.38,20.91 18.45,25.37 16,27 C 13,29 13.18,31.34 11,31 C 9.958,30.06 12.41,27.96 11,28 C 10,28 11.19,29.23 10,30 C 9,30 5.997,31 6,26 C 6,24 12,14 12,14 C 12,14 13.89,12.1 14,10.5 C 13.27,9.506 13.5,8.5 13.5,7.5 C 14.5,6.5 16.5,10 16.5,10 L 18.5,10 C 18.5,10 19.28,8.008 21,7 C 22,7 22,10 22,10"/><path d="M 9.5 25.5 A 0.5 0.5 0 1 1 8.5,25.5 A 0.5 0.5 0 1 1 9.5 25.5 z" style="fill:var(--piece-stroke);stroke:var(--piece-stroke)"/><path d="M 15 15.5 A 0.5 1.5 0 1 1 14,15.5 A 0.5 1.5 0 1 1 15 15.5 z" transform="matrix(0.866,0.5,-0.5,0.866,9.693,-5.173)" style="fill:var(--piece-stroke);stroke:var(--piece-stroke)"/></g></svg>',
};

const KINDS = ["pawn", "rook", "bishop", "knight"];

const HOME_BOARD = {
  cells: [
    { kind: "rook",   color: "black" }, null, { kind: "bishop", color: "white" }, { kind: "knight", color: "black" },
    null, null, { kind: "bishop", color: "black" }, null,
    null, { kind: "pawn",   color: "black" }, { kind: "rook",   color: "white" }, null,
    { kind: "pawn",   color: "white" }, null, { kind: "knight", color: "white" }, null,
  ],
  selected: 0,
  dots: [1, 4, 8],
  captures: [2, 12],
};

const HOME_DEMO = {
  from: 0,
  to: 12,
  color: "black",
  winLine: [
    { row: 3, col: 0 },
    { row: 2, col: 1 },
    { row: 1, col: 2 },
    { row: 0, col: 3 },
  ],
  moveDurationMs: 700,
  lineDelayMs: 420,
};

let homeDemoPlayed = false;

const DIFFICULTIES = ["easy", "medium", "hard"];
const DIFFICULTY_STORAGE_KEY = "ttc-bot-difficulty";

const state = {
  phase: "connecting",
  route: "home",
  token: null,
  lobbyId: null,
  lobbyShareStatus: null,
  roomId: null,
  roomReady: false,
  roomEverReady: false,
  myColor: null,
  board: null,
  turn: null,
  status: null,
  winner: null,
  winLineShown: false,
  selectedPiece: null,
  pawnDirections: null,
  prev: null,
  error: null,
  rematchSent: false,
  opponentWantsRematch: false,
  opponentStatus: null,
  installMessage: null,
  score: { me: 0, opponent: 0 },
  botDifficulty: "medium",
};

let ws = null;
let reconnectAttempt = 0;
let reconnectTimer = null;
let deferredInstallPrompt = null;

const gameArea = document.getElementById("game-area");
const turnIndicator = document.getElementById("turn-indicator");
const errorMessage = document.getElementById("error-message");
const overlay = document.getElementById("overlay");
const exitBtn = document.getElementById("exit-btn");
const themeToggle = document.getElementById("theme-toggle");
const homeView = document.getElementById("home-view");
const rulesView = document.getElementById("rules-view");
const openRulesBtn = document.getElementById("open-rules-btn");
const homeBoardEl = document.getElementById("home-board");
const joinLobbyBtn = document.getElementById("join-lobby-btn");
const inviteFriendBtn = document.getElementById("invite-friend-btn");
const playBotBtn = document.getElementById("play-bot-btn");
const inviteStatus = document.getElementById("invite-status");
const installAppBtn = document.getElementById("install-app-btn");
const installStatus = document.getElementById("install-status");
const difficultyButtons = Array.from(document.querySelectorAll(".difficulty-option"));
const titleLink = document.querySelector(".title-link");
const themeColorMeta = document.querySelector('meta[name="theme-color"]');

const sunSVG =
  '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="5"/><path d="M12 1v3M12 20v3M4.22 4.22l2.12 2.12M17.66 17.66l2.12 2.12M1 12h3M20 12h3M4.22 19.78l2.12-2.12M17.66 6.34l2.12-2.12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>';
const moonSVG =
  '<svg viewBox="0 0 24 24"><path d="M21 12.79A9 9 0 1 1 11.21 3a7 7 0 0 0 9.79 9.79z"/></svg>';

const SOUND_URLS = {
  place:   "/sounds/move.mp3",
  move:    "/sounds/move.mp3",
  capture: "/sounds/capture.mp3",
  win:     "/sounds/win.mp3",
  lose:    "/sounds/lose.mp3",
};
const SOUND_VOLUME = 0.7;

let audioCtx = null;
let masterGain = null;
const soundBuffers = new Map();
const soundLoading = new Map();

function getAudioCtx() {
  if (!audioCtx) {
    const Ctor = window.AudioContext || window.webkitAudioContext;
    if (!Ctor) return null;
    audioCtx = new Ctor();
    masterGain = audioCtx.createGain();
    masterGain.gain.value = SOUND_VOLUME;
    masterGain.connect(audioCtx.destination);
    audioCtx.addEventListener("statechange", () => {
      if (audioCtx.state === "running") syncLobbySonar();
    });
  }
  if (audioCtx.state === "suspended") audioCtx.resume().catch(() => {});
  return audioCtx;
}

function loadSound(name) {
  if (soundBuffers.has(name)) return Promise.resolve(soundBuffers.get(name));
  if (soundLoading.has(name)) return soundLoading.get(name);
  const ctx = getAudioCtx();
  const url = SOUND_URLS[name];
  if (!ctx || !url) return Promise.resolve(null);
  const promise = fetch(url)
    .then((res) => (res.ok ? res.arrayBuffer() : null))
    .then((data) => (data ? ctx.decodeAudioData(data) : null))
    .then((buf) => {
      if (buf) soundBuffers.set(name, buf);
      soundLoading.delete(name);
      return buf;
    })
    .catch((e) => {
      soundLoading.delete(name);
      console.debug("sound load failed", name, e);
      return null;
    });
  soundLoading.set(name, promise);
  return promise;
}

function playSound(name) {
  const ctx = getAudioCtx();
  if (!ctx) return;
  const buf = soundBuffers.get(name);
  if (buf) {
    playBuffer(ctx, buf);
    return;
  }
  loadSound(name).then((loaded) => {
    if (loaded) playBuffer(ctx, loaded);
  });
}

function playBuffer(ctx, buf) {
  const src = ctx.createBufferSource();
  src.buffer = buf;
  src.connect(masterGain);
  try {
    src.start();
  } catch {
    // ignore — happens if the context just suspended between state check and start
  }
}

// Sonar ping: 900 → 420 Hz sine sweep with exponential decay, ~0.9s tail.
// Matches the .pulse ring animation cadence (1.2s per ring).
const SONAR_PERIOD_MS = 1200;

function playSonarPing(ctx, when) {
  const t = when ?? ctx.currentTime;
  const osc = ctx.createOscillator();
  osc.type = "sine";
  osc.frequency.setValueAtTime(900, t);
  osc.frequency.exponentialRampToValueAtTime(420, t + 0.35);
  const gain = ctx.createGain();
  gain.gain.setValueAtTime(0.0001, t);
  gain.gain.exponentialRampToValueAtTime(0.25, t + 0.015);
  gain.gain.exponentialRampToValueAtTime(0.0001, t + 0.9);
  osc.connect(gain).connect(masterGain);
  osc.start(t);
  osc.stop(t + 1.0);
}

let sonarIntervalId = null;

function startSonar() {
  if (sonarIntervalId) return;
  if (window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
  const ctx = getAudioCtx();
  if (!ctx || ctx.state !== "running") return;
  playSonarPing(ctx, ctx.currentTime + 0.05);
  sonarIntervalId = setInterval(() => {
    const c = getAudioCtx();
    if (!c || c.state !== "running") return;
    playSonarPing(c, c.currentTime + 0.02);
  }, SONAR_PERIOD_MS);
}

function stopSonar() {
  if (!sonarIntervalId) return;
  clearInterval(sonarIntervalId);
  sonarIntervalId = null;
}

function syncLobbySonar() {
  const shouldPlay = state.route === "lobby" && !state.lobbyId;
  if (shouldPlay) startSonar();
  else stopSonar();
}

function preloadSounds() {
  for (const name of Object.keys(SOUND_URLS)) loadSound(name);
}

function warmSoundsOnce() {
  const once = () => {
    getAudioCtx();
    preloadSounds();
    document.removeEventListener("pointerdown", once);
    document.removeEventListener("keydown", once);
  };
  document.addEventListener("pointerdown", once, { once: true });
  document.addEventListener("keydown", once, { once: true });
}

function playMoveSound(prev, next) {
  if (!prev || !next) return;
  if (!boardChanged(prev, next)) return;
  const prevCount = countPieces(prev);
  const nextCount = countPieces(next);
  if (nextCount > prevCount) playSound("place");
  else if (nextCount < prevCount) playSound("capture");
  else playSound("move");
}

function boardChanged(a, b) {
  for (let r = 0; r < 4; r++) {
    for (let c = 0; c < 4; c++) {
      const pa = a[r][c];
      const pb = b[r][c];
      if (!pa && !pb) continue;
      if (!pa || !pb) return true;
      if (pa.kind !== pb.kind || pa.color !== pb.color) return true;
    }
  }
  return false;
}

function countPieces(board) {
  let n = 0;
  for (const row of board) {
    for (const cell of row) {
      if (cell) n++;
    }
  }
  return n;
}

document.addEventListener("DOMContentLoaded", () => {
  init().catch((error) => {
    console.error("init failed", error);
    state.phase = "connectionLost";
    state.error = error.message;
    render();
  });
});

async function init() {
  bindTheme();
  bindExit();
  bindHistory();
  bindInstall();
  bindVisibility();
  warmSoundsOnce();
  renderHomeBoard();
  initDifficulty();

  state.token = await ensureClientToken();
  syncRoute();
}

function renderHomeBoard() {
  const dotSet = new Set(HOME_BOARD.dots);
  const captureSet = new Set(HOME_BOARD.captures);

  homeBoardEl.innerHTML = "";
  for (let i = 0; i < 16; i++) {
    const cell = document.createElement("div");
    cell.className = "home-cell";
    cell.dataset.idx = i;
    cell.dataset.row = Math.floor(i / 4);
    cell.dataset.col = i % 4;
    const piece = HOME_BOARD.cells[i];
    if (piece) {
      const glyph = document.createElement("span");
      glyph.className = `piece-glyph piece-${piece.color}`;
      glyph.innerHTML = PIECE_SVGS[piece.kind];
      cell.appendChild(glyph);
      if (i === HOME_BOARD.selected) {
        cell.classList.add("selected");
      }
    }
    if (dotSet.has(i)) {
      const dot = document.createElement("span");
      dot.className = "dot";
      cell.appendChild(dot);
    } else if (captureSet.has(i)) {
      const ring = document.createElement("span");
      ring.className = "capture-ring";
      cell.appendChild(ring);
    }
    if (i === HOME_DEMO.to) {
      cell.classList.add("home-target");
    }
    homeBoardEl.appendChild(cell);
  }

  homeBoardEl.addEventListener("click", handleHomeDemoClick);
}

function handleHomeDemoClick(event) {
  if (homeDemoPlayed) return;
  const cell = event.target.closest(".home-cell");
  if (!cell) return;
  if (Number(cell.dataset.idx) !== HOME_DEMO.to) return;
  homeDemoPlayed = true;
  runHomeDemo();
}

function runHomeDemo() {
  const fromCell = homeBoardEl.querySelector(`[data-idx="${HOME_DEMO.from}"]`);
  const toCell = homeBoardEl.querySelector(`[data-idx="${HOME_DEMO.to}"]`);
  if (!fromCell || !toCell) return;
  const rook = fromCell.querySelector(".piece-glyph");
  if (!rook) return;

  for (const el of homeBoardEl.querySelectorAll(".dot, .capture-ring")) {
    el.remove();
  }
  fromCell.classList.remove("selected");
  toCell.classList.remove("home-target");

  const boardRect = homeBoardEl.getBoundingClientRect();
  const fromRect = fromCell.getBoundingClientRect();
  const toRect = toCell.getBoundingClientRect();

  rook.classList.add("home-floating-glyph");
  rook.style.left = `${fromRect.left - boardRect.left + fromRect.width * 0.12}px`;
  rook.style.top = `${fromRect.top - boardRect.top + fromRect.height * 0.12}px`;
  rook.style.width = `${fromRect.width * 0.76}px`;
  rook.style.height = `${fromRect.height * 0.76}px`;
  homeBoardEl.appendChild(rook);

  void rook.offsetWidth;

  rook.style.transition = `transform ${HOME_DEMO.moveDurationMs}ms cubic-bezier(0.4, 0.1, 0.2, 1)`;
  rook.style.transform = `translate(${toRect.left - fromRect.left}px, ${toRect.top - fromRect.top}px)`;

  // Play the capture sound as the rook visually impacts the pawn. The ease-out
  // curve has the rook reach the target well before the 700ms transition
  // formally ends, and relying on `transitionend` pushed the sound ~200–400ms
  // past the visual landing. A deterministic timer matches the impact moment.
  setTimeout(() => playSound("capture"), HOME_DEMO.moveDurationMs - 120);

  rook.addEventListener(
    "transitionend",
    () => {
      toCell.querySelector(".piece-glyph")?.remove();
      rook.remove();
      const finalRook = document.createElement("span");
      finalRook.className = "piece-glyph piece-black";
      finalRook.innerHTML = PIECE_SVGS.rook;
      toCell.appendChild(finalRook);
      setTimeout(() => {
        drawWinLine(homeBoardEl, HOME_DEMO.winLine, HOME_DEMO.color);
      }, HOME_DEMO.lineDelayMs);
    },
    { once: true }
  );
}

function initDifficulty() {
  const stored = localStorage.getItem(DIFFICULTY_STORAGE_KEY);
  state.botDifficulty = DIFFICULTIES.includes(stored) ? stored : "medium";
  syncDifficultyUI();
  for (const btn of difficultyButtons) {
    btn.addEventListener("click", () => {
      const next = btn.dataset.difficulty;
      if (!DIFFICULTIES.includes(next)) return;
      state.botDifficulty = next;
      localStorage.setItem(DIFFICULTY_STORAGE_KEY, next);
      syncDifficultyUI();
    });
  }
}

function syncDifficultyUI() {
  for (const btn of difficultyButtons) {
    btn.setAttribute("aria-checked", String(btn.dataset.difficulty === state.botDifficulty));
  }
}

function detectRoute() {
  const roomMatch = location.pathname.match(/^\/room\/([^/]+)$/);
  if (roomMatch) {
    state.route = "room";
    const newRoomId = decodeURIComponent(roomMatch[1]);
    if (newRoomId !== state.roomId) {
      state.score = { me: 0, opponent: 0 };
      state.roomId = newRoomId;
      state.roomEverReady = false;
      loadScore();
    }
    state.lobbyId = null;
    state.lobbyShareStatus = null;
    return;
  }

  const namedLobbyMatch = location.pathname.match(/^\/lobby\/([^/]+)$/);
  if (namedLobbyMatch) {
    state.route = "lobby";
    state.lobbyId = decodeURIComponent(namedLobbyMatch[1]);
    state.lobbyShareStatus = null;
    state.roomId = null;
    return;
  }

  if (location.pathname === "/lobby") {
    state.route = "lobby";
    state.lobbyId = null;
    state.lobbyShareStatus = null;
    state.roomId = null;
    return;
  }

  if (location.pathname === "/rules") {
    state.route = "rules";
    state.lobbyId = null;
    state.lobbyShareStatus = null;
    state.roomId = null;
    return;
  }

  state.route = "home";
  state.lobbyId = null;
  state.lobbyShareStatus = null;
  state.roomId = null;
}

function syncRoute() {
  detectRoute();

  if (state.route === "room") {
    connectRoom();
    return;
  }

  if (state.route === "lobby") {
    connectLobby();
    return;
  }

  disconnectSocket();
  resetBoardState();
  state.error = null;
  state.phase = "idle";
  render();
}

async function ensureClientToken() {
  const hashParams = new URLSearchParams(location.hash.slice(1));
  const clientId = hashParams.get("clientId");

  if (clientId) {
    // Network errors during hash validation propagate up — don't silently
    // fall through to creating a new client when the server is down.
    if (await tokenIsValid(clientId)) {
      removeFromHashParams("clientId");
      localStorage.setItem(tokenKey, clientId);
      return clientId;
    }
    // Server explicitly rejected the hash clientId; fall through to stored.
  }

  const stored = localStorage.getItem(tokenKey);
  if (stored) {
    try {
      if (await tokenIsValid(stored)) {
        return stored;
      }
      // Server rejected stored token: clear and issue a new one.
    } catch (error) {
      // Network error: keep the stored token and bubble up so callers can
      // retry later instead of losing identity during an outage.
      console.warn("token validation unreachable, keeping stored token", error);
      throw error;
    }
  }

  localStorage.removeItem(tokenKey);

  const response = await fetch("/api/clients", { method: "POST" });
  if (!response.ok) {
    throw new Error("failed to create client token");
  }

  const payload = await response.json();
  localStorage.setItem(tokenKey, payload.token);
  return payload.token;
}

function removeFromHashParams(paramName) {
  const hashParams = new URLSearchParams(location.hash.slice(1));
  hashParams.delete(paramName);

  const nextHash = hashParams.toString();
  const nextURL = location.pathname +
    location.search +
    (nextHash ? `#${nextHash}` : "");

  window.history.replaceState({}, "", nextURL);
}

// Returns true if the server confirmed the token is valid, false if the server
// explicitly rejected it (401/403). Throws on network errors so callers can
// distinguish "server is down" from "my token is bad" — network errors must
// NOT clear the stored token.
async function tokenIsValid(token) {
  const response = await fetch("/api/me", {
    headers: { Authorization: `Bearer ${token}` },
  });
  return response.ok;
}

function connectLobby() {
  disconnectSocket();
  resetBoardState();
  if (state.phase !== "connectionLost") {
    state.phase = "connecting";
  }
  state.error = null;
  render();

  const lobbyPath = state.lobbyId
    ? `/ws/lobby/${encodeURIComponent(state.lobbyId)}`
    : "/ws/lobby";
  const socket = new WebSocket(wsURL(`${lobbyPath}?token=${encodeURIComponent(state.token)}`));
  ws = socket;

  socket.addEventListener("open", () => {
    if (ws !== socket) {
      return;
    }
    console.debug("lobby ws open");
    resetReconnect();
  });

  socket.addEventListener("message", (event) => {
    if (ws !== socket) {
      return;
    }
    const data = JSON.parse(event.data);

    switch (data.type) {
      case "waiting":
        state.phase = "waiting";
        render();
        break;
      case "paired":
        if (data.roomId) {
          disconnectSocket();
          navigateToRoom(data.roomId);
        }
        break;
      case "error":
        showError(data.error || "lobby error");
        break;
      default:
        console.warn("unknown lobby message", data);
    }
  });

  socket.addEventListener("close", (event) => {
    if (ws !== socket) {
      return;
    }

    ws = null;
    console.debug("lobby ws close", event.code, event.reason);
    if (state.route === "lobby") {
      state.phase = "connectionLost";
      render();
      scheduleReconnect();
    }
  });

  socket.addEventListener("error", (event) => {
    if (ws !== socket) {
      return;
    }
    console.error("lobby ws error", event);
  });
}

function connectRoom() {
  if (!state.roomId) {
    navigateToLobby();
    return;
  }

  disconnectSocket();
  if (state.phase !== "connectionLost") {
    state.phase = "connecting";
  }
  state.roomReady = false;
  state.error = null;
  render();

  const socket = new WebSocket(
    wsURL(`/ws/room/${encodeURIComponent(state.roomId)}?token=${encodeURIComponent(state.token)}`),
  );
  ws = socket;

  socket.addEventListener("open", () => {
    if (ws !== socket) {
      return;
    }
    console.debug("room ws open", state.roomId);
    resetReconnect();
  });

  socket.addEventListener("message", (event) => {
    if (ws !== socket) {
      return;
    }
    const data = JSON.parse(event.data);
    console.debug("room message", data);
    handleRoomMessage(data);
  });

  socket.addEventListener("close", (event) => {
    if (ws !== socket) {
      return;
    }

    ws = null;
    console.debug("room ws close", event.code, event.reason);

    if (state.route !== "room") {
      return;
    }

    if (!state.roomEverReady) {
      navigateToLobby();
      return;
    }

    if (state.phase !== "gameOver") {
      state.phase = "connectionLost";
      render();
      scheduleReconnect();
    }
  });

  socket.addEventListener("error", (event) => {
    if (ws !== socket) {
      return;
    }
    console.error("room ws error", event);
  });
}

function handleRoomMessage(data) {
  switch (data.type) {
    case "roomJoined":
      state.myColor = data.color;
      render();
      break;
    case "gameState":
      state.prev = {
        board: state.board,
        turn: state.turn,
        status: state.status,
      };
      state.board = data.state.board;
      state.turn = data.state.turn;
      state.status = data.state.status;
      state.winner = data.state.winner;
      state.pawnDirections = data.state.pawnDirections;
      reconcileSelectedPiece();
      state.roomReady = true;
      state.roomEverReady = true;

      playMoveSound(state.prev.board, state.board);

      if (state.status === "over") {
        state.phase = "gameOver";
        if (state.winner) {
          playSound(state.winner === state.myColor ? "win" : "lose");
        }
        const scoredKey = state.roomId ? `ttc-scored-${state.roomId}` : null;
        const alreadyScored = scoredKey && localStorage.getItem(scoredKey);
        if (state.winner && !alreadyScored) {
          if (state.winner === state.myColor) {
            state.score.me++;
          } else {
            state.score.opponent++;
          }
          saveScore();
          if (scoredKey) localStorage.setItem(scoredKey, "1");
        }
      } else {
        state.phase = "playing";
      }

      render();
      break;
    case "rematchStarted":
      state.phase = "playing";
      state.myColor = data.color;
      if (state.roomId) localStorage.removeItem(`ttc-scored-${state.roomId}`);
      resetBoardState();
      render();
      break;
    case "rematchRequested":
      state.opponentWantsRematch = true;
      render();
      break;
    case "opponentAway":
      state.opponentStatus = "away";
      render();
      break;
    case "opponentDisconnected":
      state.opponentStatus = "disconnected";
      render();
      break;
    case "opponentReconnected":
      state.opponentStatus = null;
      render();
      break;
    case "reaction":
      showEmojiReaction(data.reaction, data.from);
      break;
    case "error":
      showError(data.error || "server error");
      break;
    default:
      console.warn("unknown room message", data);
  }
}

function render() {
  renderRoute();
  renderTurnIndicator();
  renderError();
  renderOverlay();
  renderGameArea();
  renderInstallCTA();
  syncLobbySonar();

  if (state.route !== "home") {
    exitBtn.classList.add("visible");
  } else {
    exitBtn.classList.remove("visible");
  }

  state.prev = null;
}

function renderRoute() {
  const showHome = state.route === "home";
  const showRules = state.route === "rules";
  const showLobby = state.route === "lobby";
  const hideGame = showHome || showRules;
  homeView.classList.toggle("hidden", !showHome);
  rulesView.classList.toggle("hidden", !showRules);
  inviteStatus.textContent = showHome ? inviteStatus.textContent : "";
  installStatus.textContent = showHome ? state.installMessage || "" : "";
  installStatus.classList.toggle("hidden", !showHome || !state.installMessage);
  turnIndicator.classList.toggle("hidden", hideGame || showLobby);
  gameArea.classList.toggle("hidden", hideGame);
  errorMessage.classList.toggle("hidden", hideGame);
}

function renderInstallCTA() {
  const showButton = state.route === "home" && !isStandalone() && !!deferredInstallPrompt;
  installAppBtn.classList.toggle("hidden", !showButton);

  if (state.route !== "home" || state.installMessage) {
    return;
  }

  if (isStandalone()) {
    installStatus.textContent = "Installed on home screen.";
    installStatus.classList.remove("hidden");
    return;
  }

  if (isIOS()) {
    installStatus.textContent = 'On iPhone, tap Share and choose "Add to Home Screen".';
    installStatus.classList.remove("hidden");
    return;
  }

  installStatus.textContent = "";
  installStatus.classList.add("hidden");
}

function renderOverlay() {
  if (state.route === "home" || state.route === "rules") {
    hideOverlay();
    return;
  }

  switch (state.phase) {
    case "connecting":
      showOverlay(state.route === "room" ? "Joining room..." : "Connecting...");
      break;
    case "waiting":
      if (state.lobbyId) {
        hideOverlay();
        break;
      }
      showOverlay("Waiting for opponent...");
      break;
    case "connectionLost":
      showOverlay("Connection lost. Reconnecting\u2026", "error");
      break;
    default:
      hideOverlay();
  }
}

function showOverlay(message, mode = "busy") {
  overlay.textContent = message;
  overlay.dataset.mode = mode;
  overlay.classList.remove("hidden");
}

function hideOverlay() {
  overlay.textContent = "";
  overlay.dataset.mode = "";
  overlay.classList.add("hidden");
}

function renderTurnIndicator() {
  if (state.route === "home") {
    turnIndicator.textContent = "";
    return;
  }

  if (state.route === "lobby") {
    turnIndicator.textContent = "";
    turnIndicator.className = "";
    return;
  }

  if (state.route !== "room") {
    turnIndicator.textContent = "";
    return;
  }

  if (state.status === "over") {
    turnIndicator.innerHTML = "";
    turnIndicator.className = "game-result";

    const row = document.createElement("div");
    row.className = "turn-row";
    const result = document.createElement("span");
    if (state.winner) {
      result.textContent = state.winner === state.myColor ? "You win!" : "You lose!";
    } else {
      result.textContent = "Draw!";
    }
    row.appendChild(result);
    turnIndicator.appendChild(row);
    const scoreEl = createScoreEl();
    if (scoreEl) turnIndicator.appendChild(scoreEl);
    return;
  }

  turnIndicator.className = "";
  if (!state.turn) {
    turnIndicator.textContent = "";
    return;
  }

  if (state.opponentStatus) {
    turnIndicator.textContent =
      state.opponentStatus === "away" ? "Opponent away..." : "Opponent disconnected";
    return;
  }

  const isMyTurn = state.turn === state.myColor;
  turnIndicator.innerHTML = "";
  turnIndicator.className = "";

  const row = document.createElement("div");
  row.className = "turn-row";
  const turnText = document.createElement("span");
  turnText.className = `turn-chip active turn-${state.turn}`;
  turnText.textContent = isMyTurn ? "your turn" : "opponent's turn";
  row.appendChild(turnText);
  turnIndicator.appendChild(row);
  const scoreEl = createScoreEl();
  if (scoreEl) turnIndicator.appendChild(scoreEl);
}

function scoreKey() {
  return state.roomId ? `ttc-score-${state.roomId}` : null;
}

function saveScore() {
  const key = scoreKey();
  if (key) localStorage.setItem(key, JSON.stringify(state.score));
}

function loadScore() {
  const key = scoreKey();
  if (!key) return;
  try {
    const saved = JSON.parse(localStorage.getItem(key));
    if (saved && typeof saved.me === "number" && typeof saved.opponent === "number") {
      state.score = saved;
    }
  } catch (_) {}
}

function clearScore() {
  const key = scoreKey();
  if (key) localStorage.removeItem(key);
  if (state.roomId) localStorage.removeItem(`ttc-scored-${state.roomId}`);
}

function createScoreEl() {
  const { me, opponent } = state.score;
  const el = document.createElement("div");
  el.className = "score-strip";
  el.innerHTML =
    `<span class="score-strip-name">Them</span>` +
    `<span class="score-strip-score">` +
      `<span class="score-num">${opponent}</span>` +
      `<span class="score-sep">\u2013</span>` +
      `<span class="score-num">${me}</span>` +
    `</span>` +
    `<span class="score-strip-name">You</span>`;
  return el;
}

function renderError() {
  errorMessage.textContent = state.error || "";
}

function renderGameArea() {
  if (state.route === "lobby" && state.lobbyId) {
    gameArea.innerHTML = "";
    gameArea.appendChild(renderInviteLobby());
    return;
  }

  if (state.route === "lobby" && !state.lobbyId) {
    gameArea.innerHTML = "";
    gameArea.appendChild(renderMatchmakingLobby());
    return;
  }

  if (state.route !== "room" || !state.board) {
    gameArea.innerHTML = "";
    return;
  }

  const flipped = state.myColor === "black";
  const topColor = flipped ? "white" : "black";
  const bottomColor = flipped ? "black" : "white";

  gameArea.innerHTML = "";
  gameArea.classList.toggle("game-over", state.status === "over");
  gameArea.appendChild(renderHand(topColor));
  gameArea.appendChild(renderColLabels());
  const boardEl = renderBoard(flipped);
  gameArea.appendChild(boardEl);
  gameArea.appendChild(renderColLabels());
  gameArea.appendChild(renderHand(bottomColor));
  if (state.status === "over") {
    gameArea.appendChild(renderRematchActions());
  }
  gameArea.appendChild(renderEmojiButton());

  if (state.winner) {
    const winLine = findWinLine(state.board, state.winner);
    if (winLine) {
      const animate = !state.winLineShown;
      state.winLineShown = true;
      requestAnimationFrame(() => drawWinLine(boardEl, winLine, state.winner, animate));
    }
  }
}

function renderRematchActions() {
  const wrap = document.createElement("div");
  wrap.className = "rematch-area";

  const homeBtn = document.createElement("button");
  homeBtn.className = "rematch-btn rematch-btn-ghost";
  homeBtn.textContent = "Home";
  homeBtn.addEventListener("click", leaveCurrentPage);
  wrap.appendChild(homeBtn);

  const rematchBtn = document.createElement("button");
  rematchBtn.className = "rematch-btn rematch-btn-primary";
  if (state.rematchSent) {
    rematchBtn.textContent = "Waiting\u2026";
    rematchBtn.disabled = true;
  } else if (state.opponentWantsRematch) {
    rematchBtn.textContent = "Accept rematch";
    rematchBtn.addEventListener("click", sendRematch);
  } else {
    rematchBtn.textContent = "Rematch";
    rematchBtn.addEventListener("click", sendRematch);
  }
  wrap.appendChild(rematchBtn);

  return wrap;
}

function renderInviteLobby() {
  const card = document.createElement("div");
  card.className = "invite-card";

  const title = document.createElement("h2");
  title.className = "invite-card-title";
  title.textContent = state.phase === "connecting" ? "Setting up\u2026" : "Play with friend";
  card.appendChild(title);

  const howTo = document.createElement("p");
  howTo.className = "invite-card-text";
  howTo.textContent = "Send this link to your friend. Once they open it, the game will start automatically.";
  card.appendChild(howTo);

  const linkBox = document.createElement("div");
  linkBox.className = "invite-link-box";

  const link = document.createElement("code");
  link.className = "invite-link";
  link.textContent = inviteLobbyURL();
  linkBox.appendChild(link);

  const copyButton = document.createElement("button");
  copyButton.className = "secondary-action";
  copyButton.textContent = "Copy Link";
  copyButton.addEventListener("click", async () => {
    try {
      const copyMethod = await copyInviteLink(inviteLobbyURL());
      state.lobbyShareStatus =
        copyMethod === "manual" ? "Clipboard unavailable. Copy the link from the dialog." : "Link copied to clipboard.";
      render();
    } catch (error) {
      console.error("copy invite link failed", error);
      state.lobbyShareStatus = "Could not copy link.";
      render();
    }
  });
  linkBox.appendChild(copyButton);

  card.appendChild(linkBox);

  const waitingText = document.createElement("p");
  waitingText.className = "invite-card-text invite-card-note";
  waitingText.textContent =
    state.phase === "connecting" ? "Connecting you to the private lobby..." : "Waiting for your friend to join...";
  card.appendChild(waitingText);

  const status = document.createElement("p");
  status.className = "invite-copy-status";
  status.textContent = state.lobbyShareStatus || "";
  card.appendChild(status);

  return card;
}

function renderMatchmakingLobby() {
  const wrap = document.createElement("div");

  const pulse = document.createElement("div");
  pulse.className = "pulse";

  const logo = document.createElement("div");
  logo.className = "avatar";
  logo.setAttribute("role", "presentation");
  logo.addEventListener("click", () => {
    const burst = document.createElement("div");
    burst.className = "pulse-burst";
    pulse.appendChild(burst);
    burst.addEventListener("animationend", () => burst.remove());
    const ctx = getAudioCtx();
    if (ctx) playSonarPing(ctx, ctx.currentTime + 0.01);
  });
  pulse.appendChild(logo);
  wrap.appendChild(pulse);

  const label = document.createElement("p");
  label.className = "lobby-label";
  label.textContent = "Game starts when another player joins.";
  wrap.appendChild(label);

  return wrap;
}

function renderHand(color) {
  const panel = document.createElement("div");
  panel.className = "hand-panel";

  panel.appendChild(document.createElement("span"));

  for (const kind of KINDS) {
    const cell = document.createElement("div");
    cell.className = "hand-cell";

    const inHand = !isPieceOnBoard(color, kind);

    if (inHand) {
      const span = document.createElement("span");
      span.className = `piece-glyph piece-${color}`;
      span.innerHTML = PIECE_SVGS[kind];
      if (state.prev && state.prev.board && wasPieceOnBoard(state.prev.board, color, kind)) {
        span.classList.add("animate-place");
      }
      if (state.selectedPiece && state.selectedPiece.code === PIECE_CODES[color][kind]) {
        cell.classList.add("selected");
      }
      cell.appendChild(span);

      cell.addEventListener("click", () => {
        if (color !== state.myColor) {
          return;
        }

        state.selectedPiece = {
          code: PIECE_CODES[color][kind],
          kind,
          color,
          source: "hand",
        };
        state.prev = null;

        render();
      });
    } else {
      cell.classList.add("empty");
    }

    panel.appendChild(cell);
  }

  panel.appendChild(document.createElement("span"));
  return panel;
}

function renderBoard(flipped) {
  const board = document.createElement("div");
  board.className = "board";

  let moves = [];
  if (state.selectedPiece && state.selectedPiece.source === "board" && state.pawnDirections) {
    const pos = findPiecePosition(state.board, state.selectedPiece.code);
    if (pos) {
      moves = computeMoves(state.board, state.selectedPiece, pos.row, pos.col, state.pawnDirections);
    }
  }

  const handPlacement = state.selectedPiece && state.selectedPiece.source === "hand";

  for (let i = 0; i < 4; i += 1) {
    const engineRow = flipped ? 3 - i : i;
    const rankNum = 4 - engineRow;

    const leftLabel = document.createElement("span");
    leftLabel.className = "row-label";
    leftLabel.textContent = rankNum;
    board.appendChild(leftLabel);

    for (let col = 0; col < 4; col += 1) {
      const cell = document.createElement("div");
      cell.className = "board-cell";
      cell.dataset.row = engineRow;
      cell.dataset.col = col;

      const move = moves.find((m) => m.row === engineRow && m.col === col);
      if (move) {
        cell.classList.add(move.capture ? "target-capture" : "target");
      } else if (handPlacement && !state.board[engineRow][col]) {
        cell.classList.add("target");
      }

      const piece = state.board[engineRow][col];
      if (piece) {
        const span = document.createElement("span");
        span.className = `piece-glyph piece-${piece.color}`;
        span.innerHTML = PIECE_SVGS[piece.kind];

        if (state.prev && state.prev.board) {
          const prev = state.prev.board[engineRow][col];
          if (!prev || prev.color !== piece.color || prev.kind !== piece.kind) {
            span.classList.add("animate-place");
          }
        }

        if (
          state.selectedPiece &&
          state.selectedPiece.code === PIECE_CODES[piece.color][piece.kind]
        ) {
          cell.classList.add("selected");
        }

        cell.appendChild(span);
      }

      cell.addEventListener("click", () => {
        const isMyTurn = state.turn === state.myColor;

        if (state.selectedPiece) {
          if (piece && piece.color === state.myColor) {
            state.selectedPiece = {
              code: PIECE_CODES[piece.color][piece.kind],
              kind: piece.kind,
              color: piece.color,
              source: "board",
            };
            state.prev = null;
            render();
            return;
          }

          if (!isMyTurn) {
            return;
          }

          send({
            type: "move",
            piece: state.selectedPiece.code,
            cell: cellNotation(engineRow, col),
          });
          return;
        }

        if (piece && piece.color === state.myColor) {
          state.selectedPiece = {
            code: PIECE_CODES[piece.color][piece.kind],
            kind: piece.kind,
            color: piece.color,
            source: "board",
          };
          state.prev = null;
          render();
          return;
        }

        state.selectedPiece = null;
        state.prev = null;
        render();
      });

      board.appendChild(cell);
    }

    const rightLabel = document.createElement("span");
    rightLabel.className = "row-label";
    rightLabel.textContent = rankNum;
    board.appendChild(rightLabel);
  }

  return board;
}

function renderColLabels() {
  const labels = document.createElement("div");
  labels.className = "col-labels";
  labels.innerHTML =
    "<span></span><span>a</span><span>b</span><span>c</span><span>d</span><span></span>";
  return labels;
}

function drawWinLine(boardEl, winLine, color, animate = true) {
  const first = winLine[0];
  const last = winLine[winLine.length - 1];
  const firstCell = boardEl.querySelector(`[data-row="${first.row}"][data-col="${first.col}"]`);
  const lastCell = boardEl.querySelector(`[data-row="${last.row}"][data-col="${last.col}"]`);
  if (!firstCell || !lastCell) {
    return;
  }

  const boardRect = boardEl.getBoundingClientRect();
  const r1 = firstCell.getBoundingClientRect();
  const r2 = lastCell.getBoundingClientRect();

  let x1 = r1.left + r1.width / 2 - boardRect.left;
  let y1 = r1.top + r1.height / 2 - boardRect.top;
  let x2 = r2.left + r2.width / 2 - boardRect.left;
  let y2 = r2.top + r2.height / 2 - boardRect.top;

  const dx = x2 - x1;
  const dy = y2 - y1;
  const len = Math.sqrt(dx * dx + dy * dy);
  const extend = r1.width * 0.4;
  x1 -= (dx / len) * extend;
  y1 -= (dy / len) * extend;
  x2 += (dx / len) * extend;
  y2 += (dy / len) * extend;
  const totalLen = len + 2 * extend;

  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("class", `win-line piece-${color}`);
  svg.setAttribute("width", boardRect.width);
  svg.setAttribute("height", boardRect.height);

  const shadow = document.createElementNS("http://www.w3.org/2000/svg", "line");
  shadow.setAttribute("class", "shadow");
  shadow.setAttribute("x1", x1);
  shadow.setAttribute("y1", y1);
  shadow.setAttribute("x2", x2);
  shadow.setAttribute("y2", y2);
  svg.appendChild(shadow);

  const line = document.createElementNS("http://www.w3.org/2000/svg", "line");
  line.setAttribute("class", "main");
  line.setAttribute("x1", x1);
  line.setAttribute("y1", y1);
  line.setAttribute("x2", x2);
  line.setAttribute("y2", y2);
  svg.appendChild(line);

  if (animate) {
    for (const ln of [shadow, line]) {
      ln.style.setProperty("--dash-len", String(totalLen));
    }
  }

  boardEl.appendChild(svg);
}

function findWinLine(board, color) {
  const line = [];

  board.forEach((row, rowIndex) => {
    row.forEach((piece, colIndex) => {
      if (piece && piece.color === color) {
        line.push({ row: rowIndex, col: colIndex });
      }
    });
  });

  if (line.length < 4) {
    return null;
  }

  return line;
}

function sendRematch() {
  send({ type: "rematch" });
  state.rematchSent = true;
  render();
}

function leaveCurrentPage() {
  resetReconnect();
  disconnectSocket();
  resetBoardState();
  state.phase = "idle";
  navigateToHome();
}


function scheduleReconnect() {
  cancelReconnect();
  const base = Math.min(1000 * Math.pow(2, reconnectAttempt), 30000);
  const jitter = Math.random() * base * 0.3;
  const delay = base + jitter;
  reconnectAttempt++;
  console.debug("reconnect in", Math.round(delay), "ms (attempt", reconnectAttempt + ")");
  reconnectTimer = setTimeout(async () => {
    reconnectTimer = null;
    if (state.phase !== "connectionLost") return;
    try {
      state.token = await ensureClientToken();
    } catch (e) {
      console.warn("token refresh failed, will retry", e);
    }
    syncRoute();
  }, delay);
}

function cancelReconnect() {
  if (reconnectTimer !== null) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
}

function resetReconnect() {
  cancelReconnect();
  reconnectAttempt = 0;
}

function disconnectSocket() {
  if (ws) {
    const socket = ws;
    ws = null;
    socket.close();
  }
}

function navigateToHome() {
  navigate("/");
}

function navigateToRules() {
  navigate("/rules");
}

function navigateToLobby() {
  navigate("/lobby");
}

function navigateToNamedLobby(lobbyId) {
  navigate(`/lobby/${encodeURIComponent(lobbyId)}`);
}

function navigateToRoom(roomId) {
  navigate(`/room/${encodeURIComponent(roomId)}#clientId=${encodeURIComponent(state.token)}`);
}

function navigate(path, { replace = false } = {}) {
  if (location.pathname === path) {
    syncRoute();
    return;
  }

  if (replace) {
    window.history.replaceState({}, "", path);
  } else {
    window.history.pushState({}, "", path);
  }

  syncRoute();
}

async function createInviteLobby() {
  inviteStatus.textContent = "Creating invite link...";

  try {
    const response = await fetch("/api/lobbies", { method: "POST" });
    if (!response.ok) {
      throw new Error("failed to create invite link");
    }

    const payload = await response.json();
    if (!payload.id) {
      throw new Error("invite link is missing lobby id");
    }

    navigateToNamedLobby(payload.id);
  } catch (error) {
    console.error("create invite lobby failed", error);
    inviteStatus.textContent = "Could not create invite link.";
  }
}

async function startBotGame() {
  inviteStatus.textContent = "Starting bot game...";

  try {
    const response = await fetch(
      `/api/bot-game?difficulty=${encodeURIComponent(state.botDifficulty)}`,
      {
        method: "POST",
        headers: { Authorization: `Bearer ${state.token}` },
      },
    );

    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || "failed to start bot game");
    }

    const payload = await response.json();
    if (!payload.roomId) {
      throw new Error("bot game response missing room ID");
    }

    navigateToRoom(payload.roomId);
  } catch (error) {
    console.error("start bot game failed", error);
    inviteStatus.textContent = "Could not start bot game.";
  }
}

function inviteLobbyURL() {
  return new URL(`/lobby/${encodeURIComponent(state.lobbyId)}`, location.origin).toString();
}

async function copyInviteLink(inviteURL) {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(inviteURL);
    return "clipboard";
  }

  const promptMessage = "Copy this invite link:";
  if (typeof window.prompt !== "function") {
    throw new Error("clipboard copy is unavailable");
  }

  window.prompt(promptMessage, inviteURL);
  return "manual";
}

function resetBoardState() {
  state.board = null;
  state.prev = null;
  state.turn = null;
  state.status = null;
  state.winner = null;
  state.winLineShown = false;
  state.pawnDirections = null;
  state.selectedPiece = null;
  state.rematchSent = false;
  state.opponentWantsRematch = false;
  state.opponentStatus = null;
}

function reconcileSelectedPiece() {
  if (!state.selectedPiece) return;

  const justMovedMyself =
    state.prev && state.prev.turn === state.myColor && state.turn !== state.myColor;
  if (justMovedMyself || state.status === "over") {
    state.selectedPiece = null;
    return;
  }

  const sel = state.selectedPiece;
  const onBoard = findPiecePosition(state.board, sel.code) !== null;
  const shouldBeOnBoard = sel.source === "board";
  if (onBoard !== shouldBeOnBoard) {
    state.selectedPiece = null;
  }
}

function isPieceOnBoard(color, kind) {
  if (!state.board) {
    return false;
  }

  for (const row of state.board) {
    for (const cell of row) {
      if (cell && cell.color === color && cell.kind === kind) {
        return true;
      }
    }
  }

  return false;
}

function wasPieceOnBoard(board, color, kind) {
  for (const row of board) {
    for (const cell of row) {
      if (cell && cell.color === color && cell.kind === kind) {
        return true;
      }
    }
  }
  return false;
}

const REACTION_EMOJIS = window.__reactionEmojis || [];

function renderEmojiButton() {
  const wrapper = document.createElement("div");
  wrapper.className = "emoji-btn-wrapper";

  const btn = document.createElement("button");
  btn.className = "emoji-btn";
  btn.innerHTML =
    '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M8 14s1.5 2 4 2 4-2 4-2"/><line x1="9" y1="9" x2="9.01" y2="9"/><line x1="15" y1="9" x2="15.01" y2="9"/></svg>' +
    '<span class="emoji-btn-label">react</span>';
  btn.title = "Send reaction";

  btn.addEventListener("click", () => {
    toggleEmojiPicker(wrapper);
  });

  wrapper.appendChild(btn);
  return wrapper;
}

function toggleEmojiPicker(wrapper) {
  const existing = wrapper.querySelector(".emoji-picker");
  if (existing) {
    existing.remove();
    return;
  }

  const picker = document.createElement("div");
  picker.className = "emoji-picker";

  for (const emoji of REACTION_EMOJIS) {
    const btn = document.createElement("button");
    btn.className = "emoji-picker-item";
    btn.textContent = emoji;
    btn.addEventListener("click", () => {
      sendReaction(emoji);
    });
    picker.appendChild(btn);
  }

  wrapper.appendChild(picker);

  function dismissOnClickOutside(e) {
    if (!wrapper.contains(e.target)) {
      picker.remove();
      document.removeEventListener("click", dismissOnClickOutside);
    }
  }
  setTimeout(() => document.addEventListener("click", dismissOnClickOutside), 0);

  function dismissOnEscape(e) {
    if (e.key === "Escape") {
      picker.remove();
      document.removeEventListener("keydown", dismissOnEscape);
    }
  }
  document.addEventListener("keydown", dismissOnEscape);
}

function sendReaction(emoji) {
  send({ type: "reaction", reaction: emoji });
}

function showEmojiReaction(emoji, fromColor) {
  const el = document.createElement("div");
  el.className = "emoji-bubble";
  el.textContent = emoji;

  el.style.left = (10 + Math.random() * 80) + "%";
  el.style.animationDuration = (2.5 + Math.random() * 1.0) + "s";

  document.body.appendChild(el);

  el.addEventListener("animationend", (e) => {
    if (e.animationName === "bubble-up") el.remove();
  });
}

function send(payload) {
  if (!ws || ws.readyState !== WebSocket.OPEN) {
    console.warn("socket is not open", payload);
    if (state.phase !== "connectionLost") {
      state.phase = "connectionLost";
      render();
      scheduleReconnect();
    }
    return;
  }

  console.debug("send", payload);
  ws.send(JSON.stringify(payload));
}

function showError(message) {
  state.error = message;
  render();

  window.clearTimeout(showError.timeoutID);
  showError.timeoutID = window.setTimeout(() => {
    state.error = null;
    render();
  }, 2000);
}

function rematchButton(label, onClick, extraClass = "") {
  const btn = document.createElement("button");
  btn.className = `rematch-btn ${extraClass}`.trim();
  btn.textContent = label;
  btn.addEventListener("click", onClick);
  return btn;
}

function canMoveTo(board, color, row, col) {
  if (row < 0 || row > 3 || col < 0 || col > 3) {
    return { allowed: false, capture: false };
  }
  const piece = board[row][col];
  if (!piece) {
    return { allowed: true, capture: false };
  }
  if (piece.color !== color) {
    return { allowed: true, capture: true };
  }
  return { allowed: false, capture: false };
}

function slideMoves(board, color, row, col, directions) {
  const moves = [];
  for (const [dr, dc] of directions) {
    let r = row;
    let c = col;
    for (let i = 0; i < 3; i++) {
      r += dr;
      c += dc;
      const result = canMoveTo(board, color, r, c);
      if (result.allowed) {
        moves.push({ row: r, col: c, capture: result.capture });
        if (result.capture) break;
      } else {
        break;
      }
    }
  }
  return moves;
}

function rookMoves(board, color, row, col) {
  return slideMoves(board, color, row, col, [
    [0, 1], [0, -1], [-1, 0], [1, 0],
  ]);
}

function bishopMoves(board, color, row, col) {
  return slideMoves(board, color, row, col, [
    [-1, 1], [1, 1], [1, -1], [-1, -1],
  ]);
}

function knightMoves(board, color, row, col) {
  const moves = [];
  const jumps = [
    [-2, -1], [-2, 1], [2, -1], [2, 1],
    [-1, -2], [-1, 2], [1, -2], [1, 2],
  ];
  for (const [dr, dc] of jumps) {
    const result = canMoveTo(board, color, row + dr, col + dc);
    if (result.allowed) {
      moves.push({ row: row + dr, col: col + dc, capture: result.capture });
    }
  }
  return moves;
}

function pawnMoves(board, color, row, col, direction) {
  const moves = [];
  const forwardRow = row + direction;

  if (forwardRow >= 0 && forwardRow <= 3 && !board[forwardRow][col]) {
    moves.push({ row: forwardRow, col: col, capture: false });
  }

  for (const dc of [-1, 1]) {
    const captureCol = col + dc;
    if (forwardRow < 0 || forwardRow > 3 || captureCol < 0 || captureCol > 3) continue;
    const target = board[forwardRow][captureCol];
    if (target && target.color !== color) {
      moves.push({ row: forwardRow, col: captureCol, capture: true });
    }
  }

  return moves;
}

function pawnDirection(pawnDirections, color) {
  const dir = pawnDirections[color];
  return dir === "toBlackSide" ? -1 : 1;
}

function findPiecePosition(board, code) {
  for (let row = 0; row < 4; row++) {
    for (let col = 0; col < 4; col++) {
      const piece = board[row][col];
      if (piece && PIECE_CODES[piece.color][piece.kind] === code) {
        return { row, col };
      }
    }
  }
  return null;
}

function computeMoves(board, piece, row, col, pawnDirections) {
  switch (piece.kind) {
    case "rook":   return rookMoves(board, piece.color, row, col);
    case "bishop": return bishopMoves(board, piece.color, row, col);
    case "knight": return knightMoves(board, piece.color, row, col);
    case "pawn":   return pawnMoves(board, piece.color, row, col, pawnDirection(pawnDirections, piece.color));
    default:       return [];
  }
}

function cellNotation(row, col) {
  return "abcd"[col] + (4 - row);
}

function wsURL(path) {
  const protocol = location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${location.host}${path}`;
}

function isDark() {
  return document.documentElement.getAttribute("data-theme") === "dark";
}

function applyTheme(dark) {
  if (dark) {
    document.documentElement.setAttribute("data-theme", "dark");
  } else {
    document.documentElement.removeAttribute("data-theme");
  }

  themeToggle.innerHTML = dark ? sunSVG : moonSVG;
  themeColorMeta?.setAttribute("content", dark ? "#0a0a12" : "#f0ebe0");
}

function bindTheme() {
  const saved = localStorage.getItem("theme");
  if (saved) {
    applyTheme(saved === "dark");
  } else {
    applyTheme(window.matchMedia("(prefers-color-scheme: dark)").matches);
  }

  themeToggle.addEventListener("click", () => {
    const dark = !isDark();
    applyTheme(dark);
    localStorage.setItem("theme", dark ? "dark" : "light");
  });
}

function bindVisibility() {
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState !== "visible") return;
    if (state.phase === "connectionLost") return;
    if (state.route !== "room" && state.route !== "lobby") return;
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      console.debug("zombie socket detected on visibility change");
      disconnectSocket();
      state.phase = "connectionLost";
      render();
      scheduleReconnect();
    }
  });
}

function bindHistory() {
  window.addEventListener("popstate", () => {
    syncRoute();
  });
}

function bindInstall() {
  installAppBtn.addEventListener("click", () => {
    promptInstall();
  });

  window.addEventListener("beforeinstallprompt", (event) => {
    event.preventDefault();
    deferredInstallPrompt = event;
    state.installMessage = null;
    render();
  });

  window.addEventListener("appinstalled", () => {
    deferredInstallPrompt = null;
    state.installMessage = "App added to your home screen.";
    render();
  });

  const standaloneQuery = window.matchMedia("(display-mode: standalone)");
  const handleStandaloneChange = () => {
    render();
  };
  if (standaloneQuery.addEventListener) {
    standaloneQuery.addEventListener("change", handleStandaloneChange);
  } else if (standaloneQuery.addListener) {
    standaloneQuery.addListener(handleStandaloneChange);
  }
}

async function promptInstall() {
  if (!deferredInstallPrompt) {
    return;
  }

  deferredInstallPrompt.prompt();
  const { outcome } = await deferredInstallPrompt.userChoice;
  deferredInstallPrompt = null;

  if (outcome === "accepted") {
    state.installMessage = "App added to your home screen.";
  } else {
    state.installMessage = "Install dismissed. You can still use the browser menu later.";
  }

  render();
}

function isStandalone() {
  return window.matchMedia("(display-mode: standalone)").matches || window.navigator.standalone === true;
}

function isIOS() {
  const ua = window.navigator.userAgent.toLowerCase();
  const touchMac = navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1;
  return /iphone|ipad|ipod/.test(ua) || touchMac;
}

function bindExit() {
  exitBtn.addEventListener("click", leaveCurrentPage);
  joinLobbyBtn.addEventListener("click", navigateToLobby);
  inviteFriendBtn.addEventListener("click", () => {
    createInviteLobby();
  });
  playBotBtn.addEventListener("click", () => {
    startBotGame();
  });
  openRulesBtn.addEventListener("click", (event) => {
    event.preventDefault();
    navigateToRules();
  });
  titleLink.addEventListener("click", (event) => {
    event.preventDefault();
    disconnectSocket();
    navigateToHome();
  });
}
