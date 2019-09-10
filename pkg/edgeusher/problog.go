package edgeusher

import (
	"fmt"
	"foglute/internal/model"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Returns Problog code from an application
func getPlCodeFromApplication(application *model.Application) string {
	names := make([]string, len(application.Services))
	servicesDescr := make([]string, len(application.Services))
	flowsDescr := make([]string, len(application.Flows))
	maxLatenciesDescr := make([]string, len(application.MaxLatencies))

	for idx, s := range application.Services {
		names[idx] = s.Id
		servicesDescr[idx] = fmt.Sprintf("service(%s, %d, %d, [%s], [%s]).", s.Id, s.TProc, s.HWReqs, strings.Join(s.IoTReqs, ","), strings.Join(s.SecReqs, ","))
	}

	for idx, f := range application.Flows {
		flowsDescr[idx] = fmt.Sprintf("flow(%s, %s, %d).", f.Src, f.Dst, f.Bandwidth)
	}

	for idx, l := range application.MaxLatencies {
		maxLatenciesDescr[idx] = fmt.Sprintf("maxLatency([%s], %d).", strings.Join(l.Chain, ", "), l.Value)
	}

	return fmt.Sprintf("%%%% Application: %s\nchain(%s, [%s]).\n%s\n%s\n%s\n",
		application.Name,
		application.Name,
		strings.Join(names, ", "),
		strings.Join(servicesDescr, "\n"),
		strings.Join(flowsDescr, "\n"),
		strings.Join(maxLatenciesDescr, "\n"),
	)
}

// Returns Problog code from an infrastructure
func getPlCodeFromInfrastructure(infrastructure *model.Infrastructure) string {
	nodesCode := make([]string, 0)
	linksCode := make([]string, len(infrastructure.Links))

	for _, n := range infrastructure.Nodes {
		for _, profile := range n.Profiles {
			nodesCode = append(nodesCode, fmt.Sprintf("%0.2f::node(%s, %d, [%s], [%s]).", profile.Probability, n.Name, profile.HWCaps, strings.Join(profile.IoTCaps, ","), strings.Join(profile.SecCaps, ",")))
		}
	}

	for idx, l := range infrastructure.Links {
		linksCode[idx] = fmt.Sprintf("%0.2f::link(%s, %s, %d, %d).", l.Probability, l.Src, l.Dst, l.Latency, l.Bandwidth)
	}

	return fmt.Sprintf("%%%% Infrastructure: %s\n%s\n%s", "kube_infrastructure", strings.Join(nodesCode, "\n"), strings.Join(linksCode, "\n"))
}

// Calls Problog using the command string passed
// It returns the output of the process
func callProblog(code string) (string, error) {
	cmdString := "echo \"" + code + "\""

	log.Println(cmdString)

	cmd := exec.Command("bash", "-c", cmdString+" | problog")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Println(string(out))
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
			list[i].Assignments[di].NodeName = depl[2]
		}
	}

	return list, nil
}
