BottleNet - Find Network Bottlenecks
----------------
Mesh Network (default) - Find the slowest nodes in a flat server network.

#### Mesh Network
Start the bottlenet cli on one of the nodes. The first node where you start the bottlenet cli will produce a single consolidated report in a json file and also act as a coordinator for the rest of the nodes.

##### Example
Run one instance of bottlenet on control node, where output will be collected:
```
$ bottlenet
```


Run the following command on each of the other peer nodes.

```
$ bottlenet THIS-SERVER-IP:7007
```

Once all the peer nodes have been added, press 'y' on the prompt (on control node) to start the tests. The output is written to `bottlenet_20060102150405.json`.

### Help

```
~ ./bottlenet --help

Bottlenet finds bottlenecks in your cluster network

1. Run one instance of bottlenet on control node, where output will be collected:

  $>_ bottlenet

2. Run one instance of bottlenet on each of the peer nodes:

  $>_ bottlenet CONTROL-SERVER-IP:PORT

Once all the peer nodes have been added, press 'y' on the prompt (on control node) to start the tests.

In order to bind bottlenet to specific interface and port

  $>_ bottlenet --adddress IP:PORT

Note: --address should be applied to both control and peer nodes

  $>_ bottlenet --address IP:PORT CONTROL-SERVER-IP:PORT

Usage:
  ./bottlenet [IP...] [-a]

Flags:
  -a, --address string   listen address (default ":7007")
  -h, --help             help for ./bottlenet
```
