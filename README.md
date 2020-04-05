BottleNet - Find Network Bottlenecks
----------------
BottleNet supports two types of networking topologies.
- Mesh Network (default) - Find the slowest nodes in a flat network.
- Client-Server Network - Find the slowest nodes in a client-server network. 

#### Mesh Network
Start the bottlenet cli on one of the nodes. The first node where you start the bottlenet cli will produce a single consolidated report on the console and also act as a coordinator for the rest of the nodes.
##### Example
```
$ bottlenet 
Run the following command on each of the other nodes.
  $ bottlenet THIS-SERVER-IP:7007

Total Nodes      :  16
Total Throughput : 12.10 GB/s (max), 7.25 GB/s (avg)

Slowest nodes in your network:
1. NODE13  3.00 GB/s out of 12 GB/s
2. NODE5   7.00 GB/s out of 12 GB/s
```
#### Client-Server Mode
Start the bottlenet cli on one of the server nodes (use -s) or the client nodes (use -c). The first node where you start the bottlenet cli will produce a single consolidated report on the console and also act as a coordinaor for the rest of the server and client nodes.

##### Example
```
$> bottlenet -s
Run the following command on each of the server nodes
  $ bottlenet THIS-SERVER-IP:7007 -s
and client nodes.
  $ bottlenet THIS-SERVER-IP:7007 -c

Total Nodes      :  8 Servers, 8 Clients
Total Throughput : 5.40 GB/s (max), 6.10 GB/s (avg)

Slowest servers in your network:
1. SERVER-3  0.50 GB/s out of 2.10 GB/s
2. SERVER-8  1.11 GB/s out of 2.10 GB/s

Slowest clients in your network:
1. CLIENT-2  201.00 MB/s out of 1.01 GB/s
2. CLIENT-7  500.00 MB/s out of 1.01 GB/s
```

### Help

```
$> bottlenet --help
bottlenet [IP] [-c|-s]

--client-network, -c      bottlenet on the client node
--server-network, -s      bottlenet on the server node
```
