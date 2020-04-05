Bottlenet - Find network bottlenecks 
----------------

Bottlenet has two modes
- mesh
- client/server

### Mesh Network

```sh
# start the mesh console on node [1] with addr server-ip:8098
$> bottlenet 
started the network bottleneck test server. waiting for clients to connect.

execute the following command on each of the other nodes (in case of this example, nodes: [2], [3], [4])
  
  bottlenet server-ip:8098

total nodes       :  4
total throughput  :  max: 12 GB/s          avg: 7.25 GB/s
bottleneck        :  node1

node1   3.00  GB/s  out of  12 GB/s
node2   9.00  GB/s  out of  12 GB/s
node3   6.00  GB/s  out of  12 GB/s
node4   12.00 GB/s  out of  12 GB/s

[topology] // representation for this example. This is not a part of the output 

                        [1]*------*2
			 | \____/ |
			 | /    \ |
			 3*-------*4

[1] - origin node
```

### Client/Server Mode

```sh
# start the server+console on node bottlenet:8098
$> bottlenet -s
started the network bottleneck test server. waiting for clients to connect.

execute the following command on each of the server nodes

  bottlenet server-ip:8098 -s
  
execute the following command on each of the client nodes
  
  bottlenet server-ip:8098 -c

[servers]
node1   3.00  GB/s  out of  12 GB/s
node2   9.00  GB/s  out of  12 GB/s

[clients]
node3   6.00  GB/s  out of  12 GB/s
node4   12.00 GB/s  out of  12 GB/s

[topology] // representation for this example. This is not a part of the output 

                        [1]*------*2
			 | \____/ |
			 | /    \ |
			 3*-------*4

[1] - origin node
```

### Help

```sh
$> bottlenet --help

bottlenet [IP] [-c | -s]

--client-network, -c      bottlenet running on the client network
--server-network, -s      bottlenet running on the server network
```
