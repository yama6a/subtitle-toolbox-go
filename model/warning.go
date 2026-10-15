package model

// ParseWarning is one problem that a parser skipped or repaired.
type ParseWarning struct{}

// WarningAction says what a parser did with a broken block.
type WarningAction string

// The warning actions.
const (
	Skipped  WarningAction = "skipped"
	Repaired WarningAction = "repaired"
)
