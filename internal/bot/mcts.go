package bot

import (
	"fmt"
	"math"
	"tic-tac-chec/engine"
)

const defaultCPUCT = 1.4

type node struct {
	game          *engine.Game
	parent        *node
	children      []*node
	action        int     // action index (0-319) that led here from parent
	prior         float32 // policy network prior probability
	visitCount    int
	totalValue    float32 // sum of backpropagated values
	isTerminal    bool
	terminalValue float32 // +1/-1/0 if terminal
}

// ucbScore computes the PUCT score for child selection.
// Q + cPUCT * prior * sqrt(parentVisits) / (1 + childVisits)
func (n *node) ucbScore(cPUCT float32) float32 {
	if n.visitCount == 0 {
		return float32(math.Inf(1))
	}
	q := -n.totalValue / float32(n.visitCount) // negamax: parent wants moves good for itself, not the child's player
	exploration := cPUCT * n.prior * float32(math.Sqrt(float64(n.parent.visitCount))) / float32(1+n.visitCount)
	return q + exploration
}

// mctsSelectAction runs MCTS and returns the best action as (Piece, Cell).
func mctsSelectAction(b *Bot, g *engine.Game, numSimulations int) (engine.Piece, engine.Cell, error) {
	root := &node{game: g.Clone()}

	if root.game.Status == engine.GameOver {
		return engine.Piece{}, engine.Cell{}, fmt.Errorf("bot: game is already over")
	}

	// Expand root node first
	value, err := expand(b, root)
	if err != nil {
		return engine.Piece{}, engine.Cell{}, fmt.Errorf("bot: expand root: %w", err)
	}
	backpropagate(root, value)

	// TODO(human): implement the MCTS simulation loop
	// For each simulation (numSimulations - 1 remaining, since root expansion counts as one):
	//   1. SELECT: walk from root to a leaf using ucbScore to pick children
	//   2. EXPAND: if the leaf is terminal, use terminalValue. Otherwise call expand(b, leaf)
	//   3. BACKPROPAGATE: call backpropagate(leaf, value)
	//
	// After all simulations, pick the root child with the highest visitCount.

	for i := 0; i < numSimulations-1; i++ {
		leaf := selectLeaf(root)

		if leaf.isTerminal {
			// terminalValue=1.0 means parent's player won (the one who moved into this state).
			// From the leaf's perspective, that's a loss (-1.0).
			backpropagate(leaf, -leaf.terminalValue)
		} else {
			value, err := expand(b, leaf)
			if err != nil {
				return engine.Piece{}, engine.Cell{}, fmt.Errorf("bot: expand leaf: %w", err)
			}
			backpropagate(leaf, value)
		}
	}

	// Pick best child by visit count
	var bestChild *node
	for _, child := range root.children {
		if bestChild == nil || child.visitCount > bestChild.visitCount {
			bestChild = child
		}
	}

	if bestChild == nil {
		return engine.Piece{}, engine.Cell{}, fmt.Errorf("bot: no children after MCTS")
	}

	return decodeActionToMove(bestChild.action, g)
}

func selectLeaf(n *node) *node {
	if len(n.children) == 0 {
		return n
	}

	highestUcb := float32(-1e9)
	var best *node
	for _, child := range n.children {
		ucb := child.ucbScore(defaultCPUCT)
		if ucb > highestUcb {
			highestUcb = ucb
			best = child
		}
	}

	return selectLeaf(best)
}

// expand creates child nodes for all legal actions from the given node.
// Calls the neural network to get policy priors and value estimate.
// Returns the value estimate (from the node's player-to-move perspective)
// for the caller to backpropagate.
func expand(b *Bot, n *node) (float32, error) {
	state := NewStateEncoder().Encode(n.game)
	logits, value, err := b.InferWithValue(state)
	if err != nil {
		return 0, err
	}

	legal := legalActions(n.game)
	if len(legal) == 0 {
		n.isTerminal = true
		n.terminalValue = 0
		return 0, nil
	}

	priors := maskedSoftmax(logits, legal)

	for i, action := range legal {
		child := &node{
			game:   n.game.Clone(),
			parent: n,
			action: action,
			prior:  priors[i],
		}

		piece, src, dst, isDrop := DecodeAction(action, child.game.Turn)
		var moveErr error
		if isDrop {
			moveErr = child.game.Move(piece, dst)
		} else {
			boardPiece := child.game.Board[src.Row][src.Col]
			if boardPiece != nil {
				moveErr = child.game.Move(*boardPiece, dst)
			}
		}

		if moveErr != nil {
			continue
		}

		if child.game.Status == engine.GameOver {
			child.isTerminal = true
			child.terminalValue = 1.0 // the player who moved just won
		}

		n.children = append(n.children, child)
	}

	return value, nil
}

// backpropagate updates visit counts and values from leaf to root.
// value is from the perspective of the player at the given node.
func backpropagate(n *node, value float32) {
	for current := n; current != nil; current = current.parent {
		current.visitCount++
		current.totalValue += value
		value = -value // flip perspective at each level
	}
}

// maskedSoftmax computes softmax over only the legal action indices.
// Returns probabilities in the same order as the legal slice.
func maskedSoftmax(logits []float32, legal []int) []float32 {
	maxLogit := float32(math.Inf(-1))
	for _, a := range legal {
		if logits[a] > maxLogit {
			maxLogit = logits[a]
		}
	}

	priors := make([]float32, len(legal))
	sum := float32(0)
	for i, a := range legal {
		priors[i] = float32(math.Exp(float64(logits[a] - maxLogit)))
		sum += priors[i]
	}
	for i := range priors {
		priors[i] /= sum
	}
	return priors
}

// decodeActionToMove converts an action index to (Piece, Cell) for the game.
func decodeActionToMove(action int, g *engine.Game) (engine.Piece, engine.Cell, error) {
	piece, src, dst, isDrop := DecodeAction(action, g.Turn)
	if isDrop {
		return piece, dst, nil
	}

	boardPiece := g.Board[src.Row][src.Col]
	if boardPiece == nil {
		return engine.Piece{}, engine.Cell{}, fmt.Errorf("bot: no piece at source %v", src)
	}

	return *boardPiece, dst, nil
}
