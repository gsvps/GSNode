package model

import "time"

type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusPassed  Status = "passed"
	StatusWarning Status = "warning"
	StatusSkipped Status = "skipped"
	StatusFailed  Status = "failed"
)

type Metric struct {
	Name   string  `json:"name"`
	Value  float64 `json:"value,omitempty"`
	Unit   string  `json:"unit,omitempty"`
	Text   string  `json:"text,omitempty"`
	Higher bool    `json:"higherIsBetter,omitempty"`
}

type Section struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Status   Status         `json:"status"`
	Score    int            `json:"score"`
	Stars    int            `json:"stars"`
	Summary  string         `json:"summary"`
	Metrics  []Metric       `json:"metrics"`
	Details  map[string]any `json:"details,omitempty"`
	Duration int64          `json:"durationMs"`
	Error    string         `json:"error,omitempty"`
}

type Report struct {
	ID        string    `json:"id"`
	Version   string    `json:"version"`
	Mode      string    `json:"mode"`
	Hostname  string    `json:"hostname"`
	Platform  string    `json:"platform"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt"`
	Score     int       `json:"score"`
	Stars     int       `json:"stars"`
	HostProvider string    `json:"hostProvider,omitempty"`
	Sections  []Section `json:"sections"`
}

type Event struct {
	Type      string    `json:"type"`
	Message   string    `json:"message,omitempty"`
	ReportID  string    `json:"reportId,omitempty"`
	Section   *Section  `json:"section,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type Options struct {
	DataDir      string
	HostProvider string
}
