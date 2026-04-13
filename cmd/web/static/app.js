const tokenKey = "ttc-client-token";
const PIECE_CODES = {
  white: { pawn: "WP", rook: "WR", bishop: "WB", knight: "WN" },
  black: { pawn: "BP", rook: "BR", bishop: "BB", knight: "BN" },
};

const PIECE_SYMBOLS = {
  pawn: "\u265F\uFE0E",
  rook: "\u265C\uFE0E",
  bishop: "\u265D\uFE0E",
  knight: "\u265E\uFE0E",
};

const KINDS = ["pawn", "rook", "bishop", "knight"];

const state = {
  phase: "connecting",
  route: "home",
  token: null,
  lobbyId: null,
  lobbyShareStatus: null,
  roomId: null,
  roomReady: false,
  myColor: null,
  board: null,
  turn: null,
  status: null,
  winner: null,
  selectedPiece: null,
  pawnDirections: null,
  prev: null,
  error: null,
  rematchSent: false,
  opponentWantsRematch: false,
  opponentStatus: null,
  installMessage: null,
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
const joinLobbyBtn = document.getElementById("join-lobby-btn");
const inviteFriendBtn = document.getElementById("invite-friend-btn");
const playBotBtn = document.getElementById("play-bot-btn");
const inviteStatus = document.getElementById("invite-status");
const installAppBtn = document.getElementById("install-app-btn");
const installStatus = document.getElementById("install-status");
const titleLink = document.querySelector(".title-link");
const themeColorMeta = document.querySelector('meta[name="theme-color"]');

const sunSVG =
  '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="5"/><path d="M12 1v3M12 20v3M4.22 4.22l2.12 2.12M17.66 17.66l2.12 2.12M1 12h3M20 12h3M4.22 19.78l2.12-2.12M17.66 6.34l2.12-2.12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>';
const moonSVG =
  '<svg viewBox="0 0 24 24"><path d="M21 12.79A9 9 0 1 1 11.21 3a7 7 0 0 0 9.79 9.79z"/></svg>';

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

  state.token = await ensureClientToken();
  syncRoute();
}

function detectRoute() {
  const roomMatch = location.pathname.match(/^\/room\/([^/]+)$/);
  if (roomMatch) {
    state.route = "room";
    state.roomId = decodeURIComponent(roomMatch[1]);
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

  if (clientId && (await tokenIsValid(clientId))) {
    removeFromHashParams("clientId");
    localStorage.setItem(tokenKey, clientId);
    return clientId;
  }

  const stored = localStorage.getItem(tokenKey);
  if (stored && (await tokenIsValid(stored))) {
    return stored;
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

async function tokenIsValid(token) {
  try {
    const response = await fetch("/api/me", {
      headers: { Authorization: `Bearer ${token}` },
    });
    return response.ok;
  } catch (error) {
    console.warn("token validation failed", error);
    return false;
  }
}

function connectLobby() {
  disconnectSocket();
  resetBoardState();
  state.phase = "connecting";
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
  state.phase = "connecting";
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

    if (!state.roomReady) {
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
      state.selectedPiece = null;
      state.roomReady = true;

      if (state.status === "over") {
        state.phase = "gameOver";
      } else {
        state.phase = "playing";
      }

      render();
      break;
    case "rematchStarted":
      state.phase = "playing";
      state.myColor = data.color;
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

  if (state.route !== "home") {
    exitBtn.classList.add("visible");
  } else {
    exitBtn.classList.remove("visible");
  }
}

function renderRoute() {
  const showHome = state.route === "home";
  homeView.classList.toggle("hidden", !showHome);
  inviteStatus.textContent = showHome ? inviteStatus.textContent : "";
  installStatus.textContent = showHome ? state.installMessage || "" : "";
  installStatus.classList.toggle("hidden", !showHome || !state.installMessage);
  turnIndicator.classList.toggle("hidden", showHome);
  gameArea.classList.toggle("hidden", showHome);
  errorMessage.classList.toggle("hidden", showHome);
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
  if (state.route === "home") {
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
    turnIndicator.textContent = state.lobbyId ? "Private Lobby" : "Matchmaking";
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

    const result = document.createElement("div");
    if (state.winner) {
      result.textContent = state.winner === state.myColor ? "You win!" : "You lose!";
    } else {
      result.textContent = "Draw!";
    }
    turnIndicator.appendChild(result);

    const rematchArea = document.createElement("div");
    rematchArea.className = "rematch-area";

    if (state.rematchSent) {
      rematchArea.textContent = "Waiting for opponent...";
    } else if (state.opponentWantsRematch) {
      rematchArea.textContent = "Opponent wants rematch!";
      rematchArea.appendChild(rematchButton("Accept", sendRematch));
    } else {
      rematchArea.appendChild(rematchButton("Rematch", sendRematch));
    }

    rematchArea.appendChild(rematchButton("Exit to lobby", leaveGameToLobby, "leave-btn"));
    turnIndicator.appendChild(rematchArea);
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
  turnIndicator.textContent = isMyTurn ? "Your turn" : "Opponent's turn";
  turnIndicator.className = isMyTurn ? `piece-${state.myColor}` : "";
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

  if (state.route !== "room" || !state.board) {
    gameArea.innerHTML = "";
    return;
  }

  const flipped = state.myColor === "black";
  const topColor = flipped ? "white" : "black";
  const bottomColor = flipped ? "black" : "white";

  gameArea.innerHTML = "";
  gameArea.appendChild(renderHand(topColor));
  gameArea.appendChild(renderColLabels());
  const boardEl = renderBoard(flipped);
  gameArea.appendChild(boardEl);
  gameArea.appendChild(renderColLabels());
  gameArea.appendChild(renderHand(bottomColor));
  gameArea.appendChild(renderEmojiButton());

  if (state.winner) {
    const winLine = findWinLine(state.board, state.winner);
    if (winLine) {
      requestAnimationFrame(() => drawWinLine(boardEl, winLine, state.winner));
    }
  }
}

function renderInviteLobby() {
  const card = document.createElement("div");
  card.className = "invite-card";

  const title = document.createElement("h2");
  title.className = "invite-card-title";
  title.textContent = state.phase === "connecting" ? "Preparing Invite" : "Invite a Friend";
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
      span.textContent = PIECE_SYMBOLS[kind];
      if (state.prev && state.prev.board && wasPieceOnBoard(state.prev.board, color, kind)) {
        span.classList.add("animate-place");
      }
      if (state.selectedPiece && state.selectedPiece.code === PIECE_CODES[color][kind]) {
        cell.classList.add("selected");
      }
      cell.appendChild(span);

      cell.addEventListener("click", () => {
        if (state.turn !== state.myColor || color !== state.myColor) {
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
        span.textContent = PIECE_SYMBOLS[piece.kind];

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
        if (state.turn !== state.myColor) {
          return;
        }

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

function drawWinLine(boardEl, winLine, color) {
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

function leaveGameToLobby() {
  resetReconnect();
  disconnectSocket();
  navigateToLobby();
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
    const response = await fetch("/api/bot-game", {
      method: "POST",
      headers: { Authorization: `Bearer ${state.token}` },
    });

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
  state.pawnDirections = null;
  state.selectedPiece = null;
  state.rematchSent = false;
  state.opponentWantsRematch = false;
  state.opponentStatus = null;
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
  btn.textContent = "\u{1F4AC}";
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

  const xPct = 10 + Math.random() * 80;
  const wobble = 20 + Math.random() * 30;
  const dir = Math.random() < 0.5 ? 1 : -1;
  const duration = 3.0 + Math.random() * 1.0;

  el.style.left = xPct + "%";
  el.style.setProperty("--wobble", (dir * wobble) + "px");
  el.style.animationDuration = duration + "s";

  document.body.appendChild(el);

  el.addEventListener("animationend", (e) => {
    if (e.animationName === "bubble-rise") el.remove();
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
  themeColorMeta?.setAttribute("content", dark ? "#0a0a12" : "#fdf8ec");
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
  titleLink.addEventListener("click", (event) => {
    event.preventDefault();
    disconnectSocket();
    navigateToHome();
  });
}
