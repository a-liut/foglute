/*
FogLute
Microservice Fog Orchestration platform.

*/
package deployment

import "foglute/internal/model"

type Mode int

const (
	Normal Mode = iota
	Heuristic
)

// A DeployerAnalyzer takes an application and an infrastructure and produce a set of placements for them.
// Each Service of the application is assigned to a specific node of the infrastructure.
type DeployAnalyzer interface {
	GetDeployment(mode Mode, application *model.Application, infrastructure *model.Infrastructure) ([]model.Placement, error)
}
