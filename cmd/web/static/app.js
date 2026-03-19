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
    if (state.winner) {
      turnIndicator.textContent =
        state.winner === state.myColor ? "You win!" : "You lose!";
    } else {
      turnIndicator.textContent = "Draw!";
    }
    turnIndicator.className = "game-result";
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
  gameArea.appendChild(renderBoard(flipped));
  gameArea.appendChild(renderColLabels());
  gameArea.appendChild(renderHand(bottomColor));
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

      case "gameState":
        state.board = data.state.board;
        state.turn = data.state.turn;
        state.status = data.state.status;
        state.winner = data.state.winner;
        state.selectedPiece = null;
        if (state.status === "over") state.phase = "gameOver";
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

function applyTheme(dark) {
  if (dark) {
    document.documentElement.setAttribute("data-theme", "dark");
  } else {
    document.documentElement.removeAttribute("data-theme");
  }
  themeToggle.textContent = dark ? "\u2600\uFE0F" : "\uD83C\uDF19";
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
