# Torch

## Description

Torch is the **Trusted Peers Orchestrator**.

This service was created with the idea to manage [Celestia Nodes](https://github.com/celestiaorg/celestia-node/) automatically.

You can use Torch to manage the nodes connections from a config file and Torch will manage those nodes for you.

Torch uses the Kubernetes API to manage the nodes, it gets their multi addresses information and stores them in a Redis instance.

---

## Workflow

Nodes side:
- Nodes check their `ENV` var during the start-up process
- If they don't have the value yet, they ask to Torch for it.
  - They send a request to the service asking for the value -> phase-2
  - If the service already has the addresses, return them, otherwise, check the nodes.
- We store the value in the config PVC in a file, to keep it there even if we restart the pod or update it, and we 
will source the value with the `start.sh`

1) Torch checks the peers based on the config file, the scope is in its namespace.
  - How does it work?
    - Torch receives a request with the nodeName in the body, then, checks the config (to validate it) and
      opens a connection to them.
    - checks the multiaddr, and stores it in memory
    - once it has the addresses, it creates a file in the config PVC with the TRUSTED_PEERS value (the path can be defined in the config)
2) Then, it restarts the nodes until all of the peers have the env var available.

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
- `/api/v1/genAll`
  - **Method**: `POST`
  - **Description**: Generate the config for all the peers in the config file
  - **Body Example**:
    ```json
    {
        "podName": 
        [
            "da-bridge-1",
            "da-full-1"
        ]
    }
    ```
  - **Response Example**:
    ```json
    {
        "status": 200,
        "body": {
            "da-bridge-0": "/dns/da-bridge-0/tcp/2121/p2p/12D3KooWDMuPiHgnB6xwnpaR4cgyAdbB5aN9zwoZCATgGxnrpk1M",
            "da-full-0": "/dns/da-full-0/tcp/2121/p2p/12D3KooWDCUaPA5ZQveFfsuAHHBNiAhEERo5J1YfbqwSZKtn9RrD"
        }
    }
    ```
- `/api/v1/metrics`
  - **Method**: `GET`
  - **Description**: Prometheus metrics endpoint.
---

## How does it work?

Here is an example of the flow, using the config:

```yaml
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

![Torch Flow](./docs/assets/torch.png)

---
