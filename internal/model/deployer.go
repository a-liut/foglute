/*
FogLute
Microservice Fog Orchestration platform.

*/
package model

import "encoding/json"

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
	Id      string   `json:"id"`
	TProc   int      `json:"t_proc"`
	HWReqs  int      `json:"hw_reqs"`
	IoTReqs []string `json:"iot_reqs"`
	SecReqs []string `json:"sec_reqs"`
	Image   Image    `json:"image"`
}

// An Image is a description of a Docker image to be used by a Service
type Image struct {
	Name       string `json:"name"`
	Local      bool   `json:"local"`
	Ports      []Port `json:"ports"`
	Privileged bool   `json:"privileged"`
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

const (
	NodeDefaultHwCaps    = 9999
	NodeDefaultLongitude = 0
	NodeDefaultLatitude  = 0
)

// A Node represent a device that can run a service.
type Node struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Address  string        `json:"address"`
	Location Location      `json:"location"`
	Profiles []NodeProfile `json:"profiles"`
}

func (n Node) String() string {
	b, _ := json.Marshal(n)
	return string(b)
}

// A Location represent a geolocated place in the world
type Location struct {
	Longitude int `json:"longitude"`
	Latitude  int `json:"latitude"`
}

// A NodeProfile describes the capabilities of a node taking in consideration the probability of that configuration
type NodeProfile struct {
	Probability float64  `json:"probability"`
	HWCaps      int      `json:"hw_caps"`
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

// A Placement is a set of Node-Service assignments produced by a deploy analyzer
type Placement struct {
	Probability float64
	Assignments []Assignment
}

func (p Placement) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

// An Assignment is a pair Node-Service produced by a deploy analyzer
type Assignment struct {
	ServiceID string
	NodeID    string
}
