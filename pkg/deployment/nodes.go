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

	if hwCaps, err := strconv.ParseInt(node.Labels[config.HwCapsLabel], 10, 32); err == nil {
		n.Profiles[0].HWCaps = int(hwCaps)
	} else {
		// Default value
		n.Profiles[0].HWCaps = model.NodeDefaultHwCaps
	}

	return n
}
