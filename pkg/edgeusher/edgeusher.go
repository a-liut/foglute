/*
Fogluted
Microservice Fog Orchestration platform.

*/
package edgeusher

import (
	"fmt"
	"foglute/internal/model"
	"foglute/pkg/deployment"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
)

const (
	EdgeUsherExec  = "edgeusher.pl"
	HedgeUsherExec = "hedgeusher.pl"
)

// EdgeUsher is an object that wraps the EdgeUsher software to produce placements of an application over an infrastructure.
type EdgeUsher struct {
	execPath  string
	hExecPath string
}

// A DeployerAnalyzer takes an application and an infrastructure and produce a set of placements for them.
// Each Service of the application is assigned to a specific node of the infrastructure.
func (eu *EdgeUsher) GetDeployment(mode deployment.Mode, application *model.Application, infrastructure *model.Infrastructure) ([]model.Placement, error) {
	var euPath string
	switch mode {
	case deployment.Normal:
		euPath = eu.execPath
	case deployment.Heuristic:
		euPath = eu.hExecPath
	default:
		log.Printf("Analysis mode not recognized: %d. Falling back to Normal analysis.", mode)
		euPath = eu.execPath
	}

	// Cleanup strings within the objects
	safeApp, appSymbolTable := cleanApp(application)
	safeInfr, infrSymbolTable := cleanInfr(infrastructure)

	// Get Problog code
	appProlog := getPlCodeFromApplication(safeApp)
	infrProlog := getPlCodeFromInfrastructure(safeInfr)

	cmdString := "echo \"" + appProlog + "\n" + infrProlog + "\n\n:- consult('" + euPath + "').\nquery(placement(Chain, Placement, Routes)).\n" + "\""

	result, err := callProblog(cmdString)
	if err != nil {
		return nil, err
	}

	log.Printf("EdgeUsher raw result: %s\n", result)

	placements, err := parseResult(result)
	if err != nil {
		return nil, err
	}

	cleanedPlacements := cleanPlacements(placements, appSymbolTable, infrSymbolTable)

	return cleanedPlacements, nil
}

// Converts all strings in placements to get real names for services and nodes using symbol tables.
func cleanPlacements(placements []model.Placement, appSymbolTable *SymbolTable, infrSymbolTable *SymbolTable) []model.Placement {
	cleaned := make([]model.Placement, len(placements))
	for i, p := range placements {
		cleaned[i].Probability = p.Probability
		cleaned[i].Assignments = make([]model.Assignment, len(p.Assignments))
		for j, a := range p.Assignments {
			cleaned[i].Assignments[j].ServiceID = appSymbolTable.GetByUID(a.ServiceID)
			cleaned[i].Assignments[j].NodeID = infrSymbolTable.GetByUID(a.NodeID)
		}
	}
	return cleaned
}

// Removes from an application strings that can make EdgeUsher fail.
// It produces a new application with updated strings and a symbol table that contains all the performed
// mappings of the names.
func cleanApp(application *model.Application) (*model.Application, *SymbolTable) {
	table := NewSymbolTable()

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

	return cleaned, table
}

// Removes from an infrastructure strings that can make EdgeUsher fail.
// It produces a new infrastructure with updated strings and a symbol table that contains all the performed
// mappings of the names.
func cleanInfr(infrastructure *model.Infrastructure) (*model.Infrastructure, *SymbolTable) {
	table := NewSymbolTable()

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
		cleaned.Links[il].Src = table.Add(link.Src)
		cleaned.Links[il].Dst = table.Add(link.Dst)
		cleaned.Links[il].Latency = link.Latency
		cleaned.Links[il].Bandwidth = link.Bandwidth
	}

	return cleaned, table
}

// Returns Problog code from an application
func getPlCodeFromApplication(application *model.Application) string {
	names := make([]string, len(application.Services))
	servicesDescr := make([]string, len(application.Services))
	flowsDescr := make([]string, len(application.Flows))
	maxLatenciesDescr := make([]string, len(application.MaxLatencies))

	for idx, s := range application.Services {
		names[idx] = s.Id
		servicesDescr[idx] = fmt.Sprintf("service(%s, %d, %d, [%s], [%s]).", s.Id, s.TProc, s.HWReqs, strings.Join(s.IoTReqs[:], ","), strings.Join(s.SecReqs[:], ","))
	}

	for idx, f := range application.Flows {
		flowsDescr[idx] = fmt.Sprintf("flow(%s, %s, %d).", f.Src, f.Dst, f.Bandwidth)
	}

	for idx, l := range application.MaxLatencies {
		maxLatenciesDescr[idx] = fmt.Sprintf("maxLatency([%s], %d).", strings.Join(l.Chain[:], ","), l.Value)
	}

	return fmt.Sprintf("%%%% Application: %s\nchain(%s, [%s]).\n%s\n%s\n%s\n",
		application.Name,
		application.Name,
		strings.Join(names[:], ","),
		strings.Join(servicesDescr[:], "\n"),
		strings.Join(flowsDescr[:], "\n"),
		strings.Join(maxLatenciesDescr[:], "\n"),
	)
}

// Returns Problog code from an infrastructure
func getPlCodeFromInfrastructure(infrastructure *model.Infrastructure) string {
	nodesCode := make([]string, 0)
	linksCode := make([]string, len(infrastructure.Links))

	for _, n := range infrastructure.Nodes {
		for _, profile := range n.Profiles {
			nodesCode = append(nodesCode, fmt.Sprintf("%0.2f::node(%s, %d, [%s], [%s]).", profile.Probability, n.Name, profile.HWCaps, strings.Join(profile.IoTCaps[:], ","), strings.Join(profile.SecCaps[:], ",")))
		}
	}

	for idx, l := range infrastructure.Links {
		linksCode[idx] = fmt.Sprintf("link(%s, %s, %d, %d).", l.Src, l.Dst, l.Latency, l.Bandwidth)
	}

	return fmt.Sprintf("%%%% Infrastructure: %s\n%s\n%s", "kube_infrastructure", strings.Join(nodesCode[:], "\n"), strings.Join(linksCode[:], "\n"))
}

// Calls Problog using the command string passed
// It returns the output of the process
func callProblog(cmdString string) (string, error) {
	cmd := exec.Command("bash", "-c", cmdString+" | problog")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return string(out), nil
}

// Parse Problog result
func parseResult(result string) ([]model.Placement, error) {
	placementRe, _ := regexp.Compile(`placement\((?P<deployments>.*)\):\s*(?P<probability>[-+]?[0-9]*\.?[0-9]+)`)
	deploymentRe, _ := regexp.Compile(`on\((?P<service>\w*),(?P<node>\w*)\)`)

	// Get first all placements
	placementsMatch := placementRe.FindAllStringSubmatch(result, -1)
	list := make([]model.Placement, len(placementsMatch))

	for i, placement := range placementsMatch {
		probability, err := strconv.ParseFloat(placement[2], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid probability: %s", placementsMatch[2])
		}

		// Get all service-node mappings
		deploymentsMatch := deploymentRe.FindAllStringSubmatch(placement[1], -1)
		list[i].Probability = probability
		list[i].Assignments = make([]model.Assignment, len(deploymentsMatch))

		for di, depl := range deploymentsMatch {
			list[i].Assignments[di].ServiceID = depl[1]
			list[i].Assignments[di].NodeID = depl[2]
		}
	}

	return list, nil
}

// Returns true if Problog is available
func checkProblog() bool {
	_, err := exec.LookPath("problog")
	return err == nil
}

// Returns true if EdgeUsher is available
func checkEdgeUsher(p string) bool {
	_, errEu := os.Stat(path.Join(p, EdgeUsherExec))
	_, errHeu := os.Stat(path.Join(p, HedgeUsherExec))
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

	log.Printf("EdgeUsher ready!")

	return &EdgeUsher{
		execPath:  path.Join(p, EdgeUsherExec),
		hExecPath: path.Join(p, HedgeUsherExec),
	}, nil
}
