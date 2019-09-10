/*
 * FogLute
 *
 * A Microservice Fog Orchestration platform.
 *
 * API version: 1.0.0
 * Contact: andrea.liut@gmail.com
 */
package edgeusher

import (
	"fmt"
	"foglute/internal/model"
	"foglute/pkg/deployment"
	"github.com/jinzhu/copier"
	"log"
	"os"
	"os/exec"
	"path"
)

const (
	execName          = "edgeusher.pl"
	heuristicExecName = "hedgeusher.pl"
)

// EdgeUsher is an object that wraps the EdgeUsher software to produce placements of an application over an infrastructure.
type EdgeUsher struct {
	execPath  string
	hExecPath string
}

// A PlacementAnalyzer takes an application and an infrastructure and produce a set of placements for them.
// Each Service of the application is assigned to a specific node of the infrastructure.
func (eu *EdgeUsher) GetPlacements(mode deployment.Mode, application *model.Application, infrastructure *model.Infrastructure) ([]model.Placement, error) {
	var euPath string
	switch mode {
	case deployment.Normal:
		euPath = eu.execPath
	case deployment.Heuristic:
		euPath = eu.hExecPath
	default:
		log.Printf("Analysis mode not recognized: %d. Falling back to Normal analysis.\n", mode)
		euPath = eu.execPath
	}

	table := NewSymbolTable()

	// Apply transformations
	processedApp := processApp(application)

	// Cleanup strings within the objects
	safeApp := cleanApp(processedApp, table)
	safeInfr := cleanInfrastructure(infrastructure, table)

	// Generate Problog code
	appProlog := getPlCodeFromApplication(safeApp)
	infrProlog := getPlCodeFromInfrastructure(safeInfr)

	code := getCode(appProlog, infrProlog, euPath)

	result, err := callProblog(code)
	if err != nil {
		return nil, err
	}

	log.Println("EdgeUsher raw result", result)

	placements, err := parseResult(result)
	if err != nil {
		return nil, err
	}

	if len(placements) == 1 && placements[0].Probability == 0 {
		return nil, fmt.Errorf("no placements available")
	}

	cleanedPlacements := cleanPlacements(placements, table)

	return cleanedPlacements, nil
}

func processApp(application *model.Application, infrastructure *model.Infrastructure, execPath string) (*model.Application, error) {

	var processed *model.Application
	err := copier.Copy(processed, application)
	if err != nil {
		return nil, err
	}

	infrCode := getPlCodeFromInfrastructure(infrastructure)

	for i, service := range processed.Services {
		if service.Replicate {

			log.Printf("Service %s is a replicant.", service.Id)

			app := &model.Application{
				ID:           "temp",
				Name:         "temp",
				Services:     []model.Service{service},
				Flows:        make([]model.Flow, 0),
				MaxLatencies: make([]model.MaxLatencyDescription, 0),
			}

			appCode := getPlCodeFromApplication(app)

			code := getCode(appCode, infrCode, execPath)

			log.Println("Replicant code", code)

			result, err := callProblog(code)
			if err != nil {
				return nil, err
			}

			log.Println("EdgeUsher raw result", result)

			placements, err := parseResult(result)
			if err != nil {
				return nil, err
			}

			if len(placements) == 1 && placements[0].Probability == 0 {
				return nil, fmt.Errorf("no placements available")
			}

			// TODO: add new fake services with names and Env. variables. (maybe we can create variables later in the processing, when building the deployment)
			// TODO: Later I'll know if a service is a replicant, and with the name I build proper env vars. The service must be aware of this!

		}
	}

	return processed, nil
}

func getCode(appCode string, infrCode string, execPath string) string {
	return appCode + "\n" + infrCode + "\n\n:- consult('" + execPath + "').\nquery(placement(Chain, Placement, Routes)).\n"
}

// Converts all strings in placements to get real names for services and nodes using symbol tables.
func cleanPlacements(placements []model.Placement, table *SymbolTable) []model.Placement {
	cleaned := make([]model.Placement, len(placements))
	for i, p := range placements {
		cleaned[i].Probability = p.Probability
		cleaned[i].Assignments = make([]model.Assignment, len(p.Assignments))
		for j, a := range p.Assignments {
			cleaned[i].Assignments[j].ServiceID = table.GetByUID(a.ServiceID)
			cleaned[i].Assignments[j].NodeID = table.GetByUID(a.NodeID)
		}
	}
	return cleaned
}

// Removes from an application strings that can make EdgeUsher fail.
// It produces a new application with updated strings and a symbol table that contains all the performed
// mappings of the names.
func cleanApp(application *model.Application, table *SymbolTable) *model.Application {
	cleaned := &model.Application{
		ID:           table.Add(application.ID),
		Name:         table.Add(application.Name),
		Services:     make([]model.Service, len(application.Services)),
		Flows:        make([]model.Flow, len(application.Flows)),
		MaxLatencies: make([]model.MaxLatencyDescription, len(application.MaxLatencies)),
	}

	for is, s := range application.Services {
		c := &cleaned.Services[is]

		c.Id = table.Add(s.Id)
		c.TProc = s.TProc
		c.HWReqs = s.HWReqs
		c.IoTReqs = make([]string, len(s.IoTReqs))
		c.SecReqs = make([]string, len(s.SecReqs))
		c.Image = s.Image

		for ir, r := range s.IoTReqs {
			c.IoTReqs[ir] = table.Add(r)
		}

		for ir, r := range s.SecReqs {
			c.SecReqs[ir] = table.Add(r)
		}
	}

	for idf, f := range application.Flows {
		c := &cleaned.Flows[idf]

		c.Src = table.Add(f.Src)
		c.Dst = table.Add(f.Dst)
		c.Bandwidth = f.Bandwidth
	}

	for il, l := range application.MaxLatencies {
		c := &cleaned.MaxLatencies[il]

		c.Chain = make([]string, len(l.Chain))
		c.Value = l.Value

		for ic, s := range l.Chain {
			c.Chain[ic] = table.Add(s)
		}
	}

	return cleaned
}

// Removes from an infrastructure strings that can make EdgeUsher fail.
// It produces a new infrastructure with updated strings and a symbol table that contains all the performed
// mappings of the names.
func cleanInfrastructure(infrastructure *model.Infrastructure, table *SymbolTable) *model.Infrastructure {
	cleaned := &model.Infrastructure{
		Nodes: make([]model.Node, len(infrastructure.Nodes)),
		Links: make([]model.Link, len(infrastructure.Links)),
	}

	for in, node := range infrastructure.Nodes {
		c := &cleaned.Nodes[in]

		c.ID = table.Add(node.ID)
		c.Name = table.Add(node.Name)
		c.Address = table.Add(node.Address)
		c.Location = node.Location

		c.Profiles = make([]model.NodeProfile, len(node.Profiles))

		for inp, np := range node.Profiles {
			cp := &cleaned.Nodes[in].Profiles[inp]

			cp.Probability = np.Probability
			cp.HWCaps = np.HWCaps

			cp.IoTCaps = make([]string, len(np.IoTCaps))
			cp.SecCaps = make([]string, len(np.SecCaps))

			for idx, o := range np.IoTCaps {
				cp.IoTCaps[idx] = table.Add(o)
			}

			for idx, o := range np.SecCaps {
				cp.SecCaps[idx] = table.Add(o)
			}
		}
	}

	for il, link := range infrastructure.Links {
		cleaned.Links[il].Probability = link.Probability
		cleaned.Links[il].Src = table.Add(link.Src)
		cleaned.Links[il].Dst = table.Add(link.Dst)
		cleaned.Links[il].Latency = link.Latency
		cleaned.Links[il].Bandwidth = link.Bandwidth
	}

	return cleaned
}

// Returns true if Problog is available
func checkProblog() bool {
	_, err := exec.LookPath("problog")
	return err == nil
}

// Returns true if EdgeUsher is available
func checkEdgeUsher(p string) bool {
	_, errEu := os.Stat(path.Join(p, execName))
	_, errHeu := os.Stat(path.Join(p, heuristicExecName))
	return errEu == nil && errHeu == nil
}

// Returns a new instance of EdgeUsher analyzer
func NewEdgeUsher(p string) (*EdgeUsher, error) {
	if !checkProblog() {
		return nil, fmt.Errorf("cannot find problog")
	}

	if !checkEdgeUsher(p) {
		return nil, fmt.Errorf("cannot find EdgeUsher in %s", p)
	}

	log.Println("EdgeUsher ready!")

	return &EdgeUsher{
		execPath:  path.Join(p, execName),
		hExecPath: path.Join(p, heuristicExecName),
	}, nil
}
