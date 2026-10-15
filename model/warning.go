package model

// ParseWarning is one problem that a parser skipped or repaired.
type ParseWarning struct {
	// Message is the full sentence, for example "The time \"00:00:0G,000\" is not valid.".
	Message string
	// Line is the 1-based input line, or NoLine.
	Line int
	// Block is the 0-based block index as in "Block #n" messages, or NoBlock.
	Block int
	// Lines holds the lines of the block, trimmed as the parser read them. It is empty for a warning without a block.
	Lines []string
	// Action says whether the parser dropped or fixed the block.
	Action WarningAction
}

// The values of ParseWarning.Line and ParseWarning.Block for a warning without a line or a block.
// Lines start at 1, so 0 means no line.
const (
	NoLine  = 0
	NoBlock = -1
)

// WarningAction says what a parser did with a broken block.
type WarningAction string

// The warning actions.
const (
	// Skipped means that the parser dropped the block.
	Skipped WarningAction = "skipped"
	// Repaired means that the parser kept the block and fixed it.
	Repaired WarningAction = "repaired"
)

// String returns "skipped" or "repaired".
func (a WarningAction) String() string {
	return string(a)
}
