/*
 * FogLute
 *
 * A Microservice Fog Orchestration platform.
 *
 * API version: 1.0.0
 * Contact: andrea.liut@gmail.com
 */
package model

import (
	"encoding/json"
	v1 "k8s.io/api/core/v1"
)

const (
	NodeDefaultHwCaps    = 9999
	NodeDefaultLongitude = 0
	NodeDefaultLatitude  = 0
)

// An Application is a set of services and relations between them.
type Application struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Services     []Service               `json:"services"`
	Flows        []Flow                  `json:"flows"`
	MaxLatencies []MaxLatencyDescription `json:"max_latency"`
}

// A Service is a part of an application that can be executed.
type Service struct {
	Id       string   `json:"id"`
	TProc    int      `json:"t_proc"`
	HWReqs   int      `json:"hw_reqs"`
	IoTReqs  []string `json:"iot_reqs"`
	SecReqs  []string `json:"sec_reqs"`
	Images   []Image  `json:"images"`
	NodeName string   `json:"node_name"`
}

// An Image is a description of a Docker image to be used by a Service
type Image struct {
	Name       string            `json:"name"`
	Local      bool              `json:"local"`
	Env        map[string]string `json:"env"`
	Ports      []Port            `json:"ports"`
	Privileged bool              `json:"privileged"`
}

type Port struct {
	Name          string `json:"name"`
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Expose        int    `json:"expose"`
}

// A Flow is a requirement that a connection between two services must satisfy
type Flow struct {
	Src       string `json:"src"`
	Dst       string `json:"dst"`
	Bandwidth int    `json:"bandwidth"`
}

// A MaxLatencyDescription describe the maximum latency that a chain of services should have
type MaxLatencyDescription struct {
	Chain []string `json:"chain"`
	Value int      `json:"value"`
}

// An Infrastructure is a collection of nodes and links between them.
type Infrastructure struct {
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
}

func (i Infrastructure) String() string {
	b, _ := json.Marshal(i)
	return string(b)
}

// A Node represent a device that can run a service.
type Node struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Address  string        `json:"address"`
	Location Location      `json:"location"`
	Profiles []NodeProfile `json:"profiles"`

	Node *v1.Node `json:"-"`
}

func (n Node) String() string {
	b, _ := json.Marshal(n)
	return string(b)
}

// A Location represent a geo-located place in the world
type Location struct {
	Longitude int `json:"longitude"`
	Latitude  int `json:"latitude"`
}

// A NodeProfile describes the capabilities of a node taking in consideration the probability of that configuration
type NodeProfile struct {
	Probability float64  `json:"probability"`
	HWCaps      int64    `json:"hw_caps"`
	IoTCaps     []string `json:"iot_caps"`
	SecCaps     []string `json:"sec_caps"`
}

// A Link is a connection between two nodes
type Link struct {
	Probability float64 `json:"probability"`
	Src         string  `json:"src"`
	Dst         string  `json:"dst"`
	Latency     int     `json:"latency"`
	Bandwidth   int     `json:"bandwidth"`
}

// A Placement is a set of Node-Service assignments produced by a Placement Analyzer
type Placement struct {
	Probability float64
	Assignments []Assignment
}

func (p Placement) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

// An Assignment is a pair Node-Service produced by a Placement Analyzer
type Assignment struct {
	ServiceID string `json:"service_id"`
	NodeID    string `json:"node_id"`
	NodeName  string `json:"node_name"`
}
