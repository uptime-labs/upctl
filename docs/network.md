## Network (Kind) - Setting up Load Balancers

#### Setup address pool used by LoadBalancers
*Skip if you are using Docker Desktop.*

To complete layer2 configuration, we need to provide MetalLB a range of IP addresses it controls. We want this range to be on the docker kind network.

```bash
$ docker network inspect -f '{{.IPAM.Config}}' kind
```

The output will contain a cidr such as 172.18.0.0/16. We want our loadbalancer IP range to come from this subclass. We can configure MetalLB, for instance, to use 172.18.255.200 to 172.19.255.250 by creating the IPAddressPool and the related L2Advertisement.

Update the configuration file `metallb-config.yaml` located in config folder.

```yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: example
  namespace: metallb-system
spec:
  addresses:
  - 172.18.255.200-172.18.255.250
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: empty
  namespace: metallb-system
```

## Using custom networks - Experimental

You can configure docker to use the IP pool of your local LAN network and then configure kind to use a customized bridge network.

1. Install dhcpd and start the service - Linux only

    ```bash
    $ sudo apt install dhcpd5
    $ sudo systemctl start dhcp5
    ```

2. In order to create a Docker network using net-dhcp, you'll need a pre-configured bridge interface on the host. How you set this up will depend on your system, but the following (manual) instructions should work on most Linux distros:

    ```bash
    # Create the bridge
    $ sudo ip link add kind-bridge type bridge
    $ sudo ip link set kind-bridge up
    
    # Assuming 'eth0' is connected to your LAN (where the DHCP server is)
    $ sudo ip link set enp88s0 up
    # Attach your network card to the bridge
    $ sudo ip link set enp88s0 master kind-bridge
    
    # If your firewall's policy for forwarding is to drop packets, you'll need to add an ACCEPT rule
    $ sudo iptables -A FORWARD -i kind-bridge -j ACCEPT
    
    # Get an IP for the host (will go out to the DHCP server since eth0 is attached to the bridge)
    # Replace this step with whatever network configuration you were using for eth0
    $ sudo dhcpcd kind-bridge
    ```

3. Install DHCP plugin

    ```bash
    $ docker plugin install ghcr.io/devplayer0/docker-net-dhcp:release-linux-amd64
    ```

4. Create a new bridge network with dhcp plugin

    ```bash
    $  docker network create -d ghcr.io/devplayer0/docker-net-dhcp:release-linux-amd64 --ipam-driver null -o bridge=kind-bridge bm-kind
    ```

    ```bash
    $ docker network ls
    
    NETWORK ID     NAME      DRIVER                                                   SCOPE
    cc3001afa2c6   bm-kind   ghcr.io/devplayer0/docker-net-dhcp:release-linux-amd64   local
    1609134ded29   bridge    bridge                                                   local
    5efbb04a6f40   host      host                                                     local
    a0b984a6d0db   none      null                                                     local
    ```

5. Create a new cluster and you should see WARNINGS similar to this:

    ```log
    Creating cluster "riddler" ...
    WARNING: Overriding docker network due to KIND_EXPERIMENTAL_DOCKER_NETWORK
    WARNING: Here be dragons! This is not supported currently.
    ```

6. Attach networks to the nodes

    ```bash
    LOOPBACK_PREFIX="1.1.1."
    KIND_BRIDGE="kind-bridge"
    MY_ROUTE=192.168.0.0/24
    MY_GW=192.168.1.1
    # Configure nodes to use the second network
    for n in $(kind get nodes); do
      # Connect the node to the second network
      docker network connect ${KIND_BRIDGE} ${n}
      # Configure a loopback address
      docker exec ${n} ip addr add ${LOOPBACK_PREFIX}${i}/32 dev lo
      # Add static routes
      docker exec ${n} ip route add ${MY_ROUTE} via {$MY_GW}
    done
```

