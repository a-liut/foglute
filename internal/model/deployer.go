package model

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
	Src       string  `json:"src"`
	Dst       string  `json:"dst"`
	Bandwidth float64 `json:"bandwidth"`
}

type MaxLatencyDescription struct {
	Chain []string `json:"chain"`
	Value int      `json:"value"`
}

type Infrastructure struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
}

type Node struct {
	ID       string        `json:"id"`
	Address  string        `json:"address"`
	Location Location      `json:"location"`
	Profiles []NodeProfile `json:"profiles"`
}

type Location struct {
	Longitude int32 `json:"longitude"`
	Latitude  int32 `json:"latitude"`
}

type NodeProfile struct {
	Probability float64  `json:"probability"`
	HWCaps      int      `json:"hw_caps"`
	IotCaps     []string `json:"iot_caps"`
	SecCaps     []string `json:"sec_caps"`
}

type Link struct {
	Src       string  `json:"src"`
	Dst       string  `json:"dst"`
	Latency   float64 `json:"latency"`
	Bandwidth float64 `json:"bandwidth"`
}

type Deployment struct {
	Service Service
	Node    Node
}
