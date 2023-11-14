# Torch

## Description

**Torch** is the ***Trusted Peers Orchestrator***.

This service was created with the idea to manage [Celestia Nodes](https://github.com/celestiaorg/celestia-node/) automatically.

You can use Torch to manage the nodes connections from a config file and Torch will manage those nodes for you.

Torch uses the Kubernetes API to manage the nodes, it gets their multi addresses information and stores them in a Redis instance, also, it provides some metrics to expose the node's IDs through the `/metrics` endpoint.

Torch automatically detects Load Balancer resources in a Kubernetes cluster and exposes metrics related to these Load Balancers.
The service uses OpenTelemetry to instrument the metrics and Prometheus to expose them.
It uses the Kubernetes API server with a watcher to receive events from it. Then filters the list to include only services of type **LoadBalancer**.
For each LoadBalancer service found, it retrieves the LoadBalancer public IP and name and generates metrics with custom labels. These metrics are then exposed via a Prometheus endpoint, making them available for monitoring and visualization in Grafana or other monitoring tools.

---

## Workflow

![Torch Flow](./docs/assets/torch.png)

When Torch receives a new request to the path `/api/v1/gen` with the node name in the body, it will verify if the node received is in the config file, if so, it will start the process, otherwise, it will reject it.

There are two types of connections:

- Using `ENV Vars`: Torch gets the data from the config file and write the connection to the node, using the `containerSetupName` to access to the node and write to a file.
  - If the value of the key `nodeType` is `da`. Torch will try to generate the node ID once the node it will be ready to accept connections (*`containerName` will be up & running*).
- Connection via `Multi Address`: The user can specify the `connectsTo` list of a node, that means the node will have one or more connections.
  - You can either use the node name like:

  ```yaml
  connectsTo:
    - "da-bridge-1-0"
    - "da-bridge-2-0"
  ```

  - or you can specify the full multi address:

  ```yaml
  connectsTo:
    - "/dns/da-bridge-1/tcp/2121/p2p/12D3KooWNFpkX9fuo3GQ38FaVKdAZcTQsLr1BNE5DTHGjv2fjEHG"
    - "/dns/da-bridge-1/tcp/2121/p2p/12D3KooWL8cqu7dFyodQNLWgJLuCzsQiv617SN9WDVX2GiZnjmeE"
  ```

  - If you want to generate the Multi address, you can either use the DNS or IP, to use dns, you will have to add the key `dnsConnections` and Torch will try to connect to this node, in the other hand, if you want to use IPs, just remove this key.
  - Example:

  ```yaml
  # This will use IP to connect to da-bridge-1-0
    - peers:
      - nodeName: "da-full-1-0"
        nodeType: "da"
        connectsTo:
          - "da-bridge-1-0"
  # This will use DNS to connect to da-bridge-1-0 & da-bridge-2-0 
    - peers:
      - nodeName: "da-full-2-0"
        nodeType: "da"
        dnsConnections:
          - "da-bridge-1"
          - "da-bridge-2"
        connectsTo:
          - "da-bridge-1-0"
          - "da-bridge-2-0"
  ```

---

## API Paths

- `/api/v1/config`
  - **Method**: `GET`
  - **Description**: Returns the config added by the user, can be used to debug
- `/api/v1/list`
  - **Method**: `GET`
  - **Description**: Returns the list of the pods available in it's namespace based on the config file
- `/api/v1/noId/<nodeName>`
  - **Method**: `GET`
  - **Description**: Returns the multi address of the node requested.
- `/api/v1/gen`
  - **Method**: `POST`
  - **Description**: Starts the process to generate the trusted peers on the nodes based on the config
  - **Body Example**:

    ```json
    {
        "podName": "da-bridge-1"
    }
    ```

  - **Response Example**:

    ```json
    {
        "status": 200,
        "body": {
            "da-bridge-0": "/dns/da-bridge-0/tcp/2121/p2p/12D3KooWDMuPiHgnB6xwnpaR4cgyAdbB5aN9zwoZCATgGxnrpk1M"
        }
    }
    ```

- `/metrics`
  - **Method**: `GET`
  - **Description**: Prometheus metrics endpoint.

---

## Config Example

Here is an example of the flow, using the config:

```yaml
---
mutualPeers:
  - consensusNode: "consensus-validator-1"
  - peers:
      - nodeName: "consensus-full-1-0"
        containerName: "consensus" # optional - default: consensus
        containerSetupName: "consensus-setup" # optional - default: consensus-setup
        connectsAsEnvVar: true
        nodeType: "consensus"
        connectsTo:
          - "consensus-validator-1"
  - peers:
      - nodeName: "consensus-full-2-0"
        connectsAsEnvVar: true
        nodeType: "consensus"
        connectsTo:
          - "consensus-validator-1"
  - peers:
      - nodeName: "da-bridge-1-0"
        connectsAsEnvVar: true
        nodeType: "da"
        connectsTo:
          - "consensus-full-1"
  - peers:
      - nodeName: "da-bridge-2-0"
        containerName: "da" # optional - default: da
        containerSetupName: "da-setup" # optional - default: da-setup
        connectsAsEnvVar: true
        nodeType: "da"
        connectsTo:
          - "consensus-full-2"
  - peers:
      - nodeName: "da-bridge-3-0"
        containerName: "da"
        nodeType: "da"
        connectsTo:
          - "da-bridge-1-0"
          - "da-bridge-2-0"
  - peers:
      - nodeName: "da-full-1-0"
        containerName: "da"
        containerSetupName: "da-setup"
        nodeType: "da"
        dnsConnections:
          - "da-bridge-1"
          - "da-bridge-2"
        connectsTo:
          - "da-bridge-1-0"
          - "da-bridge-2-0"
  - peers:
      - nodeName: "da-full-2-0"
        containerName: "da"
        containerSetupName: "da-setup"
        nodeType: "da"
        connectsTo:
          - "da-bridge-1-0"
          - "da-bridge-2-0"
  - peers:
      - nodeName: "da-full-3-0"
        nodeType: "da"
        connectsTo:
          # all the nodes in line using IP
          - "/ip4/100.64.5.103/tcp/2121/p2p/12D3KooWNFpkX9fuo3GQ38FaVKdAZcTQsLr1BNE5DTHGjv2fjEHG,/ip4/100.64.5.15/tcp/2121/p2p/12D3KooWL8cqu7dFyodQNLWgJLuCzsQiv617SN9WDVX2GiZnjmeE"
          # all the nodes in line using DNS
          - "/dns/da-bridge-1/tcp/2121/p2p/12D3KooWKsHCeUVJqJwymyi3bGt1Gwbn5uUUFi2N9WQ7G6rUSXig,/dns/da-bridge-2/tcp/2121/p2p/12D3KooWA26WDUmejZzU6XHc4C7KQNSWaEApe5BEyXFNchAqrxhA"
          # one node per line, either IP or DNS
          - "/dns/da-bridge-1/tcp/2121/p2p/12D3KooWKsHCeUVJqJwymyi3bGt1Gwbn5uUUFi2N9WQ7G6rUSXig"
          - "/dns/da-bridge-2/tcp/2121/p2p/12D3KooWA26WDUmejZzU6XHc4C7KQNSWaEApe5BEyXFNchAqrxhA"
    trustedPeersPath: "/tmp"
```

### Another example

The architecture will contain:

- 1 Consensus - Validator
- 2 Consensus - non-validating mode - connected to the validator
- 1 DA-Bridge-1 - connected to the CONS-NON-VALIDATOR
- 1 DA-Bridge-2 - connected to the CONS-NON-VALIDATOR
- 1 DA-Full-Node-1 - connected to DA-BN-1
- 1 DA-Full-Node-2 - connected to DA-BN-1 & DA-BN-2 using DNS

```yaml
---
mutualPeers:
  - consensusNode: "consensus-validator-1"
  - peers:
      - nodeName: "consensus-full-1-0"
        connectsAsEnvVar: true
        nodeType: "consensus"
        connectsTo:
          - "consensus-validator-1"
  - peers:
      - nodeName: "consensus-full-2-0"
        connectsAsEnvVar: true
        nodeType: "consensus"
        connectsTo:
          - "consensus-validator-1"
  - peers:
      - nodeName: "da-bridge-1-0"
        connectsAsEnvVar: true
        nodeType: "da"
        connectsTo:
          - "consensus-full-1"
  - peers:
      - nodeName: "da-bridge-2-0"
        connectsAsEnvVar: true
        nodeType: "da"
        connectsTo:
          - "consensus-full-2"
  - peers:
      - nodeName: "da-full-1-0"
        nodeType: "da"
        dnsConnections:
          - "da-bridge-1"
        connectsTo:
          - "da-bridge-1-0"
  - peers:
      - nodeName: "da-full-2-0"
        nodeType: "da"
        dnsConnections:
          - "da-bridge-1"
          - "da-bridge-2"
        connectsTo:
          - "da-bridge-1-0"
          - "da-bridge-2-0"
```

## Requirements

### Redis

Torch uses [Redis](https://redis.io/) as a DB, so to use Torch, you need to have a Redis instance available to connect.

We are using Redis in two different ways:
- Store the Nodes IDs and reuse them.
- As a message broker, Torch uses the Producer & Consumer approach to process data async.

---

## Metrics

### Multi Address

Custom metrics to expose the nodes multi-address:

- `multiaddr`: This metric represents the nodes Multi Address:
  - `service_name`: The service name. In this case, it is set to **torch**.
  - `node_name`: The name of the node.
  - `multiaddress`: Node Multi Address.
  - `namespace`: The namespace in which the torch is deployed.
  - `value`: The value of the metric. In this example, it is set to 1.

### BlockHeight

Custom metrics to expose the first block height of the chain:

- `block_height_1`: Name of the metric to represent the first block height of the chain:
  - `service_name`: The service name. In this case, it is set to **torch**.
  - `block_height_1`: First block id generated
  - `earliest_block_time`: Timestamp when the chain was created.
  - `days_running`: Number of days that the chain is running.
  - `namespace`: The namespace in which the torch is deployed.
  - `value`: The value of the metric. In this example, it is set to 1.

### Load Balancer

Custom metrics to expose the LoadBalancer public IPs:

- `load_balancer`: This metric represents the LoadBalancer resource and includes the following labels:
  - `service_name`: The service name. In this case, it is set to **torch**.
  - `load_balancer_name`: The name of the LoadBalancer service.
  - `load_balancer_ip`: The IP address of the LoadBalancer.
  - `namespace`: The namespace in which the LoadBalancer is deployed.
  - `value`: The value of the metric. In this example, it is set to 1, but it can be customized to represent different load balancing states.

  
---

## Monitoring and Visualization

Torch exposes some custom metrics through the Prometheus endpoint.
You can use Grafana to connect to Prometheus and create custom dashboards to visualize these metrics.

To access the Prometheus and Grafana dashboards and view the metrics, follow these steps:

1. Access the Prometheus dashboard:
- Open a web browser and navigate to the Prometheus server's URL (e.g., `http://prometheus-server:9090`).
- In the Prometheus web interface, you can explore and query the metrics collected by the Service Torch.

2. Access the Grafana dashboard:
- Open a web browser and navigate to the Grafana server's URL (e.g., `http://grafana-server:3000`).
- Log in to Grafana using your credentials.
- Create a new dashboard or import an existing one to visualize the LoadBalancer metrics from Prometheus.
- Use the `load_balancer` metric and its labels to filter and display the relevant information.

Customizing dashboards and setting up alerts in Grafana will help you monitor the performance and health of your LoadBalancer resources effectively.

---
