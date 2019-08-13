package model

import "encoding/json"

type Application struct {
	ID           string                  `json:"id,omitempty"`
	Name         string                  `json:"name"`
	Services     []Service               `json:"services"`
	Flows        []Flow                  `json:"flows"`
	MaxLatencies []MaxLatencyDescription `json:"max_latency"`
}

type Service struct {
	Id      string   `json:"id"`
	Name    string   `json:"name"`
	TProc   int      `json:"t_proc"`
	HWReqs  int      `json:"hw_reqs"`
	IoTReqs []string `json:"iot_reqs"`
	SecReqs []string `json:"sec_reqs"`
	Image   string   `json:"image"`
}

type Flow struct {
	Src       string `json:"src"`
	Dst       string `json:"dst"`
	Bandwidth int    `json:"bandwidth"`
}

type MaxLatencyDescription struct {
	Chain []string `json:"chain"`
	Value int      `json:"value"`
}

type Infrastructure struct {
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
}

func (i Infrastructure) String() string {
	b, _ := json.Marshal(i)
	return string(b)
}

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

type Location struct {
	Longitude int32 `json:"longitude"`
	Latitude  int32 `json:"latitude"`
}

type NodeProfile struct {
	Probability float64  `json:"probability"`
	HWCaps      int      `json:"hw_caps"`
	IoTCaps     []string `json:"iot_caps"`
	SecCaps     []string `json:"sec_caps"`
}

type Link struct {
	Src       string `json:"src"`
	Dst       string `json:"dst"`
	Latency   int    `json:"latency"`
	Bandwidth int    `json:"bandwidth"`
}

type Placement struct {
	Probability float64
	Assignments []Assignment
}

func (p Placement) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

type Assignment struct {
	ServiceID string
	NodeID    string
}
