// --- Constants ---

const PIECE_CODES = {
  white: { pawn: "WP", rook: "WR", bishop: "WB", knight: "WN" },
  black: { pawn: "BP", rook: "BR", bishop: "BB", knight: "BN" },
};

const PIECE_SYMBOLS = {
  pawn: "\u265F",
  rook: "\u265C",
  bishop: "\u265D",
  knight: "\u265E",
};

const KINDS = ["pawn", "rook", "bishop", "knight"];

function cellNotation(row, col) {
  return "abcd"[col] + (4 - row);
}

// --- State ---

const state = {
  phase: "connecting", // connecting | waiting | playing | gameOver
  myColor: null, // "white" | "black"
  board: null, // 4x4 array from server, each cell null or {color, kind}
  turn: null, // "white" | "black"
  status: null, // "started" | "over"
  winner: null, // "white" | "black" | null
  selectedPiece: null, // { code: "WP", kind: "pawn", color: "white", source: "hand" | "board" }
  error: null,
  rematchSent: false,
  opponentWantsRematch: false,
};

let ws = null;

// --- DOM references ---

const gameArea = document.getElementById("game-area");
const turnIndicator = document.getElementById("turn-indicator");
const errorMessage = document.getElementById("error-message");
const overlay = document.getElementById("overlay");

// --- Render ---

function render() {
  renderTurnIndicator();
  renderError();
  renderOverlay();
  renderGameArea();
}

function renderOverlay() {
  switch (state.phase) {
    case "connecting":
      overlay.textContent = "Connecting...";
      overlay.classList.remove("hidden");
      break;
    case "waiting":
      overlay.textContent = "Waiting for opponent...";
      overlay.classList.remove("hidden");
      break;
    case "disconnected":
      overlay.textContent = "Opponent disconnected";
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
  if (state.status === "over") {
    turnIndicator.innerHTML = "";
    turnIndicator.className = "game-result";

    const result = document.createElement("div");
    if (state.winner) {
      result.textContent =
        state.winner === state.myColor ? "You win!" : "You lose!";
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
      const btn = document.createElement("button");
      btn.className = "rematch-btn";
      btn.textContent = "Accept";
      btn.addEventListener("click", sendRematch);
      rematchArea.appendChild(btn);
    } else {
      const btn = document.createElement("button");
      btn.className = "rematch-btn";
      btn.textContent = "Rematch";
      btn.addEventListener("click", sendRematch);
      rematchArea.appendChild(btn);
    }
    turnIndicator.appendChild(rematchArea);
    return;
  }
  if (!state.turn) {
    turnIndicator.textContent = "";
    return;
  }
  const isMyTurn = state.turn === state.myColor;
  turnIndicator.textContent = isMyTurn ? "Your turn" : "Opponent's turn";
  turnIndicator.className = isMyTurn ? "piece-" + state.myColor : "";
}

function renderError() {
  errorMessage.textContent = state.error || "";
}

function renderGameArea() {
  if (!state.board) {
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

  panel.appendChild(document.createElement("span")); // left spacer

  for (const kind of KINDS) {
    const cell = document.createElement("div");
    cell.className = "hand-cell";

    const inHand = !isPieceOnBoard(color, kind);

    if (inHand) {
      const span = document.createElement("span");
      span.className = "piece-" + color;
      span.textContent = PIECE_SYMBOLS[kind];
      if (
        state.selectedPiece &&
        state.selectedPiece.code === PIECE_CODES[color][kind]
      ) {
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

  panel.appendChild(document.createElement("span")); // right spacer

  return panel;
}

function renderBoard(flipped) {
  const board = document.createElement("div");
  board.className = "board";

  for (let i = 0; i < 4; i++) {
    const engineRow = flipped ? 3 - i : i;
    const rankNum = 4 - engineRow;

    const leftLabel = document.createElement("span");
    leftLabel.className = "row-label";
    leftLabel.textContent = rankNum;
    board.appendChild(leftLabel);

    for (let col = 0; col < 4; col++) {
      const cell = document.createElement("div");
      cell.className = "board-cell";
      cell.dataset.row = engineRow;
      cell.dataset.col = col;

      const piece = state.board[engineRow][col];
      if (piece) {
        const span = document.createElement("span");
        span.className = "piece-" + piece.color;
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
            // reselect new piece
            state.selectedPiece = {
              code: PIECE_CODES[piece.color][piece.kind],
              kind: piece.kind,
              color: piece.color,
              source: "board",
            };
            render();
            return;
          }

          ws.send(
            JSON.stringify({
              type: "move",
              piece: state.selectedPiece.code,
              cell: cellNotation(engineRow, col),
            }),
          );
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

// --- Helpers ---

function drawWinLine(boardEl, winLine, color) {
  const first = winLine[0];
  const last = winLine[winLine.length - 1];
  const firstCell = boardEl.querySelector(
    `[data-row="${first.row}"][data-col="${first.col}"]`,
  );
  const lastCell = boardEl.querySelector(
    `[data-row="${last.row}"][data-col="${last.col}"]`,
  );
  if (!firstCell || !lastCell) return;

  const boardRect = boardEl.getBoundingClientRect();
  const r1 = firstCell.getBoundingClientRect();
  const r2 = lastCell.getBoundingClientRect();

  let x1 = r1.left + r1.width / 2 - boardRect.left;
  let y1 = r1.top + r1.height / 2 - boardRect.top;
  let x2 = r2.left + r2.width / 2 - boardRect.left;
  let y2 = r2.top + r2.height / 2 - boardRect.top;

  // extend line beyond cell centers
  const dx = x2 - x1;
  const dy = y2 - y1;
  const len = Math.sqrt(dx * dx + dy * dy);
  const extend = r1.width * 0.4;
  x1 -= (dx / len) * extend;
  y1 -= (dy / len) * extend;
  x2 += (dx / len) * extend;
  y2 += (dy / len) * extend;

  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("class", "win-line piece-" + color);
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
  let line = [];

  board.forEach((row, rowIndex) => {
    row.forEach((piece, colIndex) => {
      if (piece && piece.color === color) {
        line.push({ row: rowIndex, col: colIndex });
      }
    });
  });

  if (line.length < 4) return null;

  return line;
}

function sendRematch() {
  ws.send(JSON.stringify({ type: "rematch" }));
  state.rematchSent = true;
  render();
}

function isPieceOnBoard(color, kind) {
  if (!state.board) return false;
  for (const row of state.board) {
    for (const cell of row) {
      if (cell && cell.color === color && cell.kind === kind) return true;
    }
  }
  return false;
}

// --- WebSocket ---

function connect() {
  const protocol = location.protocol === "https:" ? "wss://" : "ws://";
  ws = new WebSocket(protocol + location.host + "/ws");

  ws.addEventListener("open", () => {
    state.phase = "waiting";
    render();

    const token = localStorage.getItem("token");
    if (token) {
      ws.send(JSON.stringify({ type: "reconnect", token }));
    } else {
      ws.send(JSON.stringify({ type: "join" }));
    }
  });

  ws.addEventListener("close", () => {
    if (state.phase != "gameOver") {
      state.phase = "connectionLost";
      render();
    }
  });

  ws.addEventListener("message", (event) => {
    const data = JSON.parse(event.data);

    switch (data.type) {
      case "paired":
        state.myColor = data.color;
        state.phase = "playing";
        localStorage.setItem("token", data.token);
        render();
        break;

      case "rematchStarted":
        state.phase = "playing";
        state.myColor = data.color;
        state.board = null;
        state.turn = null;
        state.status = null;
        state.winner = null;
        state.selectedPiece = null;
        state.rematchSent = false;
        state.opponentWantsRematch = false;
        render();
        break;

      case "gameState":
        state.board = data.state.board;
        state.turn = data.state.turn;
        state.status = data.state.status;
        state.winner = data.state.winner;
        state.selectedPiece = null;
        state.rematchSent = false;
        state.opponentWantsRematch = false;
        if (state.status === "over") {
          state.phase = "gameOver";
        } else {
          state.phase = "playing";
        }
        render();
        break;

      case "error":
        state.error = data.error;
        render();
        setTimeout(() => {
          state.error = null;
          render();
        }, 2000);
        break;

      case "opponentDisconnected":
        state.phase = "disconnected";
        render();
        break;

      case "opponentAway":
        state.phase = "waiting";
        render();
        break;

      case "opponentReconnected":
        state.phase = "playing";
        render();
        break;

      case "rematchRequested":
        state.opponentWantsRematch = true;
        render();
        break;

      case "tokenExpired":
        state.phase = "waiting";
        localStorage.removeItem("token");
        ws.close();
        connect();
        break;
    }
  });
}

// --- Theme ---

const themeToggle = document.getElementById("theme-toggle");

function isDark() {
  return document.documentElement.getAttribute("data-theme") === "dark";
}

const sunSVG =
  '<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="5"/><path d="M12 1v3M12 20v3M4.22 4.22l2.12 2.12M17.66 17.66l2.12 2.12M1 12h3M20 12h3M4.22 19.78l2.12-2.12M17.66 6.34l2.12-2.12" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>';
const moonSVG =
  '<svg viewBox="0 0 24 24"><path d="M21 12.79A9 9 0 1 1 11.21 3a7 7 0 0 0 9.79 9.79z"/></svg>';

function applyTheme(dark) {
  if (dark) {
    document.documentElement.setAttribute("data-theme", "dark");
  } else {
    document.documentElement.removeAttribute("data-theme");
  }
  themeToggle.innerHTML = dark ? sunSVG : moonSVG;
}

themeToggle.addEventListener("click", () => {
  const dark = !isDark();
  applyTheme(dark);
  localStorage.setItem("theme", dark ? "dark" : "light");
});

// --- Init ---

document.addEventListener("DOMContentLoaded", () => {
  const saved = localStorage.getItem("theme");
  if (saved) {
    applyTheme(saved === "dark");
  } else {
    applyTheme(window.matchMedia("(prefers-color-scheme: dark)").matches);
  }
  render();
  connect();
});
