package deployment

import "foglute/internal/model"

type Mode int

const (
	Normal Mode = iota
	Heuristic
)

type DeployAnalyzer interface {
	GetDeployment(mode Mode, application *model.Application, infrastructure *model.Infrastructure) ([]model.Placement, error)
}
