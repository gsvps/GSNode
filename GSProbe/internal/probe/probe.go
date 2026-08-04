package probe

import (
	"context"
	"time"

	"gsprobe/internal/model"
)

type Logger func(string)

type Probe interface {
	ID() string
	Name() string
	Run(context.Context, model.Options, Logger) model.Section
}

func Base(p Probe) model.Section {
	return model.Section{ID: p.ID(), Name: p.Name(), Status: model.StatusRunning, Metrics: []model.Metric{}, Details: map[string]any{}}
}

func Finish(section model.Section, started time.Time) model.Section {
	section.Duration = time.Since(started).Milliseconds()
	if section.Stars == 0 && section.Score > 0 {
		section.Stars = max(1, min(5, (section.Score+1999)/2000))
	}
	if section.Status == model.StatusRunning {
		section.Status = model.StatusPassed
	}
	return section
}
