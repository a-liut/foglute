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

After proper setup, `fogluted` uses a Unix socket to receive new commands.

Currently, it supports only two commands:

- Add: Add a new application to the manager. This operation triggers the deployment of the application.
- Delete: Removes an application from the manager. This operation triggers the deletion of the application from the kubernetes cluster.

The command is specified as a JSON object and the structure changes depending on the operation. See `/examples` folder for some examples.

## Todo

- Watch the infrastructure and redeploy services if needed.