/*
 * FogLute
 *
 * A Microservice Fog Orchestration platform.
 *
 * API version: 1.0.0
 * Contact: andrea.liut@gmail.com
 */
package deployment

import (
	"foglute/internal/model"
	"foglute/pkg/config"
	apiv1 "k8s.io/api/core/v1"
	"strconv"
	"strings"
)

// Converts a list of Kubernetes nodes to a list of Manager nodes
func convertNodes(nodes []apiv1.Node) []model.Node {
	ret := make([]model.Node, len(nodes))
	for i, n := range nodes {
		ret[i] = convertNode(n)
	}

	return ret
}

// Converts a Kubernetes node to a Manager node
func convertNode(node apiv1.Node) model.Node {
	n := model.Node{
		ID:      string(node.GetUID()),
		Name:    node.Name,
		Address: node.Status.Addresses[0].Address,
		Location: model.Location{
			Longitude: model.NodeDefaultLongitude,
			Latitude:  model.NodeDefaultLatitude,
		},
		Profiles: make([]model.NodeProfile, 1),
		Node:     &node,
	}

	if long, err := strconv.ParseInt(node.Labels[config.LongitudeLabel], 10, 32); err == nil {
		n.Location.Longitude = int(long)
	}

	if lat, err := strconv.ParseInt(node.Labels[config.LatitudeLabel], 10, 32); err == nil {
		n.Location.Latitude = int(lat)
	}

	n.Profiles[0].Probability = 1
	if iotCaps, exists := node.Labels[config.IotLabel]; exists {
		n.Profiles[0].IoTCaps = strings.Split(iotCaps, ",")
	} else {
		n.Profiles[0].IoTCaps = make([]string, 0)
	}
	if secCaps, exists := node.Labels[config.SecLabel]; exists {
		n.Profiles[0].SecCaps = strings.Split(secCaps, ",")
	} else {
		n.Profiles[0].SecCaps = make([]string, 0)
	}

	n.Profiles[0].HWCaps = getHwCaps(&node)

	return n
}

// Extracts Hardware capabilities from a node
func getHwCaps(node *apiv1.Node) int64 {
	m := node.Status.Capacity.Memory().Value()
	if m <= 0 {
		return model.NodeDefaultHwCaps
	}

	return m
}
