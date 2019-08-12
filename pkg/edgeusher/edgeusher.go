package edgeusher

import (
	"fmt"
	"foglute/internal/model"
	"log"
	"os"
	"path"
)

const (
	EdgeUsherExec  = "edgeusher.pl"
	HedgeUsherExec = "hedgeusher.pl"
)

type EdgeUsher struct {
	path string
}

func (eu *EdgeUsher) GetDeployment(application model.Application, infrastructure model.Infrastructure) ([]model.Deployment, error) {
	return nil, nil
}

func checkPath(p string) bool {
	_, errEu := os.Stat(path.Join(p, EdgeUsherExec))
	_, errHeu := os.Stat(path.Join(p, HedgeUsherExec))
	return errEu == nil && errHeu == nil
}

func NewEdgeUsher(path string) (*EdgeUsher, error) {
	if !checkPath(path) {
		return nil, fmt.Errorf("cannot find EdgeUsher in %s", path)
	}

	log.Printf("EdgeUsher ready!")

	return &EdgeUsher{
		path: path,
	}, nil
}
