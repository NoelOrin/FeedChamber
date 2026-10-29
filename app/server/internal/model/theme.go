package model

// Theme describes a registered declarative theme.
type Theme struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Dark bool   `json:"dark"`
}

const (
	PrefTheme = "theme"
)
