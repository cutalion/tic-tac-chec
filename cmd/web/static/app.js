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
  roomId: null,
  roomReady: false,
  myColor: null,
  board: null,
  turn: null,
  status: null,
  winner: null,
  selectedPiece: null,
  error: null,
  rematchSent: false,
  opponentWantsRematch: false,
  opponentStatus: null,
};

let ws = null;

const gameArea = document.getElementById("game-area");
const turnIndicator = document.getElementById("turn-indicator");
const errorMessage = document.getElementById("error-message");
const overlay = document.getElementById("overlay");
const exitBtn = document.getElementById("exit-btn");
const themeToggle = document.getElementById("theme-toggle");
const homeView = document.getElementById("home-view");
const joinLobbyBtn = document.getElementById("join-lobby-btn");

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
  detectRoute();
  state.token = await ensureClientToken();
  render();

  if (state.route === "room") {
    connectRoom();
  } else if (state.route === "lobby") {
    connectLobby();
  } else {
    state.phase = "idle";
    render();
  }
}

function detectRoute() {
  const roomMatch = location.pathname.match(/^\/room\/([^/]+)$/);
  if (roomMatch) {
    state.route = "room";
    state.roomId = decodeURIComponent(roomMatch[1]);
    return;
  }

  if (location.pathname === "/lobby") {
    state.route = "lobby";
    state.roomId = null;
    return;
  }

  state.route = "home";
  state.roomId = null;
}

async function ensureClientToken() {
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

  ws = new WebSocket(wsURL(`/ws/lobby?token=${encodeURIComponent(state.token)}`));

  ws.addEventListener("open", () => {
    console.debug("lobby ws open");
    state.phase = "waiting";
    render();
  });

  ws.addEventListener("message", (event) => {
    const data = JSON.parse(event.data);
    console.debug("lobby message", data);

    switch (data.type) {
      case "waiting":
        state.phase = "waiting";
        render();
        break;
      case "matched":
        if (data.roomId) {
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

  ws.addEventListener("close", (event) => {
    console.debug("lobby ws close", event.code, event.reason);
    if (state.route === "lobby") {
      state.phase = "connectionLost";
      render();
    }
  });

  ws.addEventListener("error", (event) => {
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

  ws = new WebSocket(
    wsURL(`/ws/room/${encodeURIComponent(state.roomId)}?token=${encodeURIComponent(state.token)}`),
  );

  ws.addEventListener("open", () => {
    console.debug("room ws open", state.roomId);
  });

  ws.addEventListener("message", (event) => {
    const data = JSON.parse(event.data);
    console.debug("room message", data);
    handleRoomMessage(data);
  });

  ws.addEventListener("close", (event) => {
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
    }
  });

  ws.addEventListener("error", (event) => {
    console.error("room ws error", event);
  });
}

function handleRoomMessage(data) {
  switch (data.type) {
    case "roomJoined":
      state.myColor = data.color;
      state.roomReady = true;
      state.phase = "playing";
      render();
      break;
    case "gameState":
      state.board = data.state.board;
      state.turn = data.state.turn;
      state.status = data.state.status;
      state.winner = data.state.winner;
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

  if (state.route === "room" && (state.phase === "playing" || state.phase === "gameOver")) {
    exitBtn.classList.add("visible");
  } else {
    exitBtn.classList.remove("visible");
  }
}

function renderRoute() {
  const showHome = state.route === "home";
  homeView.classList.toggle("hidden", !showHome);
  turnIndicator.classList.toggle("hidden", showHome);
  gameArea.classList.toggle("hidden", showHome);
  errorMessage.classList.toggle("hidden", showHome);
}

function renderOverlay() {
  if (state.route === "home") {
    overlay.classList.add("hidden");
    return;
  }

  switch (state.phase) {
    case "connecting":
      overlay.textContent = state.route === "lobby" ? "Connecting..." : "Joining room...";
      overlay.classList.remove("hidden");
      break;
    case "waiting":
      overlay.textContent = "Waiting for opponent...";
      overlay.classList.remove("hidden");
      break;
    case "connectionLost":
      overlay.textContent = "Connection lost";
      overlay.classList.remove("hidden");
      break;
    default:
      overlay.classList.add("hidden");
  }
}

function renderTurnIndicator() {
  if (state.route === "home") {
    turnIndicator.textContent = "";
    return;
  }

  if (state.route === "lobby") {
    turnIndicator.textContent = "Matchmaking";
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

    rematchArea.appendChild(rematchButton("Exit to lobby", leaveRoom, "leave-btn"));
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

  if (state.winner) {
    const winLine = findWinLine(state.board, state.winner);
    if (winLine) {
      requestAnimationFrame(() => drawWinLine(boardEl, winLine, state.winner));
    }
  }
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
      span.className = `piece-${color}`;
      span.textContent = PIECE_SYMBOLS[kind];
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

      const piece = state.board[engineRow][col];
      if (piece) {
        const span = document.createElement("span");
        span.className = `piece-${piece.color}`;
        span.textContent = PIECE_SYMBOLS[piece.kind];

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
          render();
          return;
        }

        state.selectedPiece = null;
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

function leaveRoom() {
  disconnectSocket();
  navigateToLobby();
}

function disconnectSocket() {
  if (ws) {
    ws.close();
    ws = null;
  }
}

function navigateToLobby() {
  window.location.assign("/lobby");
}

function navigateToRoom(roomId) {
  window.location.assign(`/room/${encodeURIComponent(roomId)}`);
}

function resetBoardState() {
  state.board = null;
  state.turn = null;
  state.status = null;
  state.winner = null;
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

function send(payload) {
  if (!ws || ws.readyState !== WebSocket.OPEN) {
    console.warn("socket is not open", payload);
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

function bindExit() {
  exitBtn.addEventListener("click", leaveRoom);
  joinLobbyBtn.addEventListener("click", navigateToLobby);
}
