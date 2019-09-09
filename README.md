# FogLute

FogLute is a software tool that manges applications over a Kubernetes cluster.
For the application it manager, it decides the best placement possible and deploy them consequently.

# Run

FogLute is developed as a Go module. Run `go build` to produce the executable.

Make sure to have `edgeusher`, `problog` and `kubectl` installed.

# Basics

## How it works

When a new application is provided to FogLute, it gets the available cluster nodes.
Then it performs an analysis of both application and infrastructure to devise the best QoS-aware placement of services.
The analysis is currently performed by EdgeUsher tool (https://github.com/di-unipi-socc/EdgeUsher).
The analysis produces a set of feasible placements for application services. The best placement will be deployed on the cluster
and maintained by FogLute. 

## How to use FogLute

FogLute exposes a REST interface

###

- GET /applications: gets information about all active applications

    Example response:
        
     ```json
      [
          {
              "application": {
                  "id": "gio",
                  "name": "gio",
                  "services": [
                      {
                          "id": "frontend",
                          "t_proc": 2,
                          "hw_reqs": 1,
                          "iot_reqs": [],
                          "sec_reqs": [],
                          "image": {
                              "name": "gio-frontend-ms:latest",
                              "local": true,
                              "env": {
                                  "API_GATEWAY_HOST": "gio-api-gateway-endpoint",
                                  "API_GATEWAY_PORT": "5000"
                              },
                              "ports": [
                                  {
                                      "name": "gio-frontend-endpoint",
                                      "host_port": 5005,
                                      "container_port": 8080,
                                      "expose": 30005
                                  }
                              ],
                              "privileged": false
                          }
                      },
                      {
                          "id": "api-gateway",
                          "t_proc": 2,
                          "hw_reqs": 1,
                          "iot_reqs": [],
                          "sec_reqs": [],
                          "image": {
                              "name": "gio-api-gateway-ms:latest",
                              "local": true,
                              "env": {
                                  "DEVICE_SERVICE_HOST": "gio-device-ms-endpoint",
                                  "DEVICE_SERVICE_PORT": "5001"
                              },
                              "ports": [
                                  {
                                      "name": "gio-api-gateway-endpoint",
                                      "host_port": 5000,
                                      "container_port": 8080,
                                      "expose": 30000
                                  }
                              ],
                              "privileged": false
                          }
                      },
                      {
                          "id": "device-ms",
                          "t_proc": 2,
                          "hw_reqs": 1,
                          "iot_reqs": [],
                          "sec_reqs": [],
                          "image": {
                              "name": "gio-device-ms:latest",
                              "local": true,
                              "env": {
                                  "DEVICE_DRIVER_HOST": "gio-device-driver-endpoint",
                                  "DEVICE_DRIVER_PORT": "5006"
                              },
                              "ports": [
                                  {
                                      "name": "gio-device-ms-endpoint",
                                      "host_port": 5001,
                                      "container_port": 8080,
                                      "expose": 30001
                                  }
                              ],
                              "privileged": false
                          }
                      },
                      {
                          "id": "device-driver",
                          "t_proc": 2,
                          "hw_reqs": 1,
                          "iot_reqs": [
                              "fognode"
                          ],
                          "sec_reqs": [],
                          "image": {
                              "name": "gio-device-driver:latest",
                              "local": true,
                              "env": {
                                  "CALLBACK_HOST": "localhost",
                                  "CALLBACK_PORT": "5006",
                                  "DEVICE_SERVICE_HOST": "gio-device-ms-endpoint",
                                  "DEVICE_SERVICE_PORT": "5001",
                                  "FOG_NODE_PORT": "5003"
                              },
                              "ports": [
                                  {
                                      "name": "gio-device-driver-endpoint",
                                      "host_port": 5006,
                                      "container_port": 8080,
                                      "expose": 30006
                                  }
                              ],
                              "privileged": false
                          }
                      }
                  ],
                  "flows": [
                      {
                          "src": "frontend",
                          "dst": "api-gateway",
                          "bandwidth": 1
                      },
                      {
                          "src": "api-gateway",
                          "dst": "device-ms",
                          "bandwidth": 1
                      },
                      {
                          "src": "device-ms",
                          "dst": "device-driver",
                          "bandwidth": 1
                      }
                  ],
                  "max_latency": [
                      {
                          "chain": [
                              "frontend",
                              "api-gateway",
                              "device-ms",
                              "device-driver"
                          ],
                          "value": 99999
                      }
                  ]
              },
              "placement": {
                  "Probability": 1,
                  "Assignments": [
                      {
                          "ServiceID": "frontend",
                          "NodeID": "60dbb626-2772-46fe-835d-1feecc6550bb"
                      },
                      {
                          "ServiceID": "api-gateway",
                          "NodeID": "60dbb626-2772-46fe-835d-1feecc6550bb"
                      },
                      {
                          "ServiceID": "device-ms",
                          "NodeID": "60dbb626-2772-46fe-835d-1feecc6550bb"
                      },
                      {
                          "ServiceID": "device-driver",
                          "NodeID": "60dbb626-2772-46fe-835d-1feecc6550bb"
                      }
                  ]
              }
          }
      ]
     ```
- POST /applications: requests the deploy of a new application

    Example body:
        
     ```json
      {
        "id": "test-app-1",
        "name": "TestApp1",
        "services": [
          {
            "id": "s1",
            "image": {
              "name": "nginx:1.12"
            },
            "t_proc": 2,
            "hw_reqs": 1,
            "iot_reqs": [],
            "sec_reqs": []
          },
          {
            "id": "s2",
            "image": {
              "name": "nginx:1.12"
            },
            "t_proc": 2,
            "hw_reqs": 1,
            "iot_reqs": [],
            "sec_reqs": []
          }
        ],
        "flows": [
          {
            "src": "s1",
            "dst": "s2",
            "bandwidth": 10
          }
        ],
        "max_latency": [
          {
            "chain": [
              "s1",
              "s2"
            ],
            "value": 150
          }
        ]
      }
     ```
  
    Example response:
    ```json
    {
        "message": "Application deployment request added successfully",
        "error": ""
    }  
    ```
  
- GET /applications/{applicationId}: gets information about application identified by a specific ID

    Example response:
    
    ```json
        {
            "application": {
                "id": "gio",
                "name": "gio",
                "services": [
                    {
                        "id": "frontend",
                        "t_proc": 2,
                        "hw_reqs": 1,
                        "iot_reqs": [],
                        "sec_reqs": [],
                        "image": {
                            "name": "gio-frontend-ms:latest",
                            "local": true,
                            "env": {
                                "API_GATEWAY_HOST": "gio-api-gateway-endpoint",
                                "API_GATEWAY_PORT": "5000"
                            },
                            "ports": [
                                {
                                    "name": "gio-frontend-endpoint",
                                    "host_port": 5005,
                                    "container_port": 8080,
                                    "expose": 30005
                                }
                            ],
                            "privileged": false
                        }
                    },
                    {
                        "id": "api-gateway",
                        "t_proc": 2,
                        "hw_reqs": 1,
                        "iot_reqs": [],
                        "sec_reqs": [],
                        "image": {
                            "name": "gio-api-gateway-ms:latest",
                            "local": true,
                            "env": {
                                "DEVICE_SERVICE_HOST": "gio-device-ms-endpoint",
                                "DEVICE_SERVICE_PORT": "5001"
                            },
                            "ports": [
                                {
                                    "name": "gio-api-gateway-endpoint",
                                    "host_port": 5000,
                                    "container_port": 8080,
                                    "expose": 30000
                                }
                            ],
                            "privileged": false
                        }
                    },
                    {
                        "id": "device-ms",
                        "t_proc": 2,
                        "hw_reqs": 1,
                        "iot_reqs": [],
                        "sec_reqs": [],
                        "image": {
                            "name": "gio-device-ms:latest",
                            "local": true,
                            "env": {
                                "DEVICE_DRIVER_HOST": "gio-device-driver-endpoint",
                                "DEVICE_DRIVER_PORT": "5006"
                            },
                            "ports": [
                                {
                                    "name": "gio-device-ms-endpoint",
                                    "host_port": 5001,
                                    "container_port": 8080,
                                    "expose": 30001
                                }
                            ],
                            "privileged": false
                        }
                    },
                    {
                        "id": "device-driver",
                        "t_proc": 2,
                        "hw_reqs": 1,
                        "iot_reqs": [
                            "fognode"
                        ],
                        "sec_reqs": [],
                        "image": {
                            "name": "gio-device-driver:latest",
                            "local": true,
                            "env": {
                                "CALLBACK_HOST": "localhost",
                                "CALLBACK_PORT": "5006",
                                "DEVICE_SERVICE_HOST": "gio-device-ms-endpoint",
                                "DEVICE_SERVICE_PORT": "5001",
                                "FOG_NODE_PORT": "5003"
                            },
                            "ports": [
                                {
                                    "name": "gio-device-driver-endpoint",
                                    "host_port": 5006,
                                    "container_port": 8080,
                                    "expose": 30006
                                }
                            ],
                            "privileged": false
                        }
                    }
                ],
                "flows": [
                    {
                        "src": "frontend",
                        "dst": "api-gateway",
                        "bandwidth": 1
                    },
                    {
                        "src": "api-gateway",
                        "dst": "device-ms",
                        "bandwidth": 1
                    },
                    {
                        "src": "device-ms",
                        "dst": "device-driver",
                        "bandwidth": 1
                    }
                ],
                "max_latency": [
                    {
                        "chain": [
                            "frontend",
                            "api-gateway",
                            "device-ms",
                            "device-driver"
                        ],
                        "value": 99999
                    }
                ]
            },
            "placement": {
                "Probability": 1,
                "Assignments": [
                    {
                        "ServiceID": "frontend",
                        "NodeID": "60dbb626-2772-46fe-835d-1feecc6550bb"
                    },
                    {
                        "ServiceID": "api-gateway",
                        "NodeID": "60dbb626-2772-46fe-835d-1feecc6550bb"
                    },
                    {
                        "ServiceID": "device-ms",
                        "NodeID": "60dbb626-2772-46fe-835d-1feecc6550bb"
                    },
                    {
                        "ServiceID": "device-driver",
                        "NodeID": "60dbb626-2772-46fe-835d-1feecc6550bb"
                    }
                ]
            }
        }
    ```

- DELETE /applications/{applicationId}: requests the withdraw of the application identified by a specific ID

    Example response:
  ```json
  {
      "message": "Application deletion request added successfully",
      "error": ""
  }
  ```

## TODO

- Watch the infrastructure and redeploy services if needed.