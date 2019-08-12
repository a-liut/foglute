package deployment

import "foglute/internal/model"

type DeployAnalyzer interface {
	GetDeployment(application model.Application, infrastructure model.Infrastructure) ([]model.Deployment, error)
}
