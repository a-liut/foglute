# fogluted

`fogluted` is a daemon tool that manges applications over a Kubernetes cluster.
For the application it manager, it decides the best placement possible and deploy them consequently.

# Run

`fogluted` is developed as a Go module. Run `go build` to produce the executable.

Make sure to have `edgeusher`, `problog` and `kubectl` installed

# Basics

## How it works

When a new application is provided to `fogluted`, it gets the available cluster nodes.
Then it performs an analysis of both application and infrastructure to devise the best QoS-aware placement of services.
The analysis is currently performed by EdgeUsher tool (https://github.com/di-unipi-socc/EdgeUsher).
The analysis produces a set of feasible placements for application services. The best placement will be deployed on the cluster
and maintained by `fogluted`. 

## How to use fogluted

`fogluted` exposes a REST interface:

| Path               | Method | Description                                        |
|--------------------|--------|----------------------------------------------------|
| /applications      | GET    | Get information about all active applications      |
|                    | POST   | Add a new application                              |
| /applications/{id} | GET    | Get information about application with specific ID |
| /applications/{id} | DELETE | Remove the application with a specific ID          |

## Todo

- Watch the infrastructure and redeploy services if needed.
- Implement proper messages for the REST interface