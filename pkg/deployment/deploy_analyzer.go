/*
 * FogLute
 *
 * A Microservice Fog Orchestration platform.
 *
 * API version: 1.0.0
 * Contact: andrea.liut@gmail.com
 */
package deployment

import "foglute/internal/model"

type Mode int

const (
	Normal Mode = iota
	Heuristic
)

// A PlacementAnalyzer takes an application and an infrastructure and produce a set of placements for them.
// Each Service of the application is assigned to a specific node of the infrastructure.
type PlacementAnalyzer interface {
	GetPlacements(mode Mode, application *model.Application, infrastructure *model.Infrastructure) ([]model.Placement, error)
}
