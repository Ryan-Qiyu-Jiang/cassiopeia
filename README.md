 # Cassiopeia

*Like Cassandra but worse.*

Cassiopeia is a distributed hash-table based off Cassandra, with pieces from Kademlia and other systems.
Written in go, it also has a **client**, **cli** and importable **package** runnable on single and multiple machines.

Cassio offers tune-able **strong consistency** while prioritizing *fast writes*. <br/>
Cassio is **fault-tolerent**, and *immediately* updates revived nodes.<br/>
Cassio is **elastic** and easily supports birth and death of peers.

It is architecturally similar to Cassandra,
* with Kademlia inspired peer finding,
* without SSL table optimization (yet), 
* with new node initiation instead of coordinator hinted hand-off. 

*Also it's written by one cs student in the weekend between work and school.*


## Composition:

 - Async Event loop for requests
 - Jobs, workers for tunable response-processing between peers
 - Gossip-Style Heartbeating for membership lists
 - Bloom filters to optimize reads
 - Memtable cache flushes and squashes to disk
 - Exponential falloff for automatic node recovery
 - NN based consistent-hashing regardless of current peers
 - Tunable consistency
 - Lots of reader-writer optimizations
 
And it's pretty easy to read the code.


## Using Cassiopeia

Cassiopeia on k8s is in the works, it should just be a helm-chart deployment. <br>
Written in go, Cassio can be compiled to an executable binary for any platform, look for the exe branch for compilable version. <br>
Added the ubuntu_init script I made to config Cassio on ubuntu!

*Running Cassio on multiple machines is easy.*

 1. Get access to some machines with ubuntu, tcp, and ports 9000+
 2. Apt-get update, install git
 3. git clone https://github.com/Ryan-Qiyu-Jiang/cassiopeia.git
 4. mv ./cassiopeia/ubuntu_init.sh $HOME
 5. chmod +x $HOME/ubuntu_init.sh
 6. $HOME/ubuntu_init.sh
 7. $GOPATH/bin/cassiopeia <machine_ip> <machine_port> 

Other platforms work as similarly.

*Running Cassio on a single node for development is also easy.*

 1. Import pkg from GitHub 
 2. Make a thread for the node 
 3. Instantiate the client 
 4. Have a blast :)

*Running multiple Cassio peers on a single node with the Cassiopeia pkg.*

1. Make a thread for each node
2. Run the init function
3. Instantiate the client 

The more friends the merrier :D especially when you moved in too early and you're the only one of your friends in the city right now.


### Example: Single Peer
```go
package main

import (
    cassio "cassiopeia"
    "fmt"

    "github.com/Ryan-Qiyu-Jiang/cassiopeia/client"
)

func main() {
    go cassio.NewNode("127.0.0.1", "9000")

    c := client.NewClient("127.0.0.1:9000")
    c.Connect()

    c.Set("ryan", "jiang")
    c.Set("password", "pass1234")

    ryan, ok1 := c.Get("ryan")
    password, ok2 := c.Get("password")
    tomato, ok3 := c.Get("tomato")

    fmt.Println("\t\t$$$ ryan ->", ryan, ok1)
    fmt.Println("\t\t$$$ password ->", password, ok2)
    fmt.Println("\t\t$$$ not found tomato ->", tomato, ok3)

    c.Del("ryan")
    ryan, ok1 = c.Get("ryan")

    fmt.Println("\t\t$$$ ryan ->", ryan, ok1)

    c.Disconnect()
}

```

### Example: Multiple Peers
```go
package main

import (
    cassio "cassiopeia"
    "fmt"

    "github.com/Ryan-Qiyu-Jiang/cassiopeia/client"
    "github.com/Ryan-Qiyu-Jiang/cassiopeia/init"
)

func main() {
    // configure new node replica threads
    // db files will be generated in db subdir under seperate port subdir
    go cassio.NewNode("127.0.0.1", "9000", 3, 3, 3)
    go cassio.NewNode("127.0.0.1", "9001", 3, 3, 3)
    go cassio.NewNode("127.0.0.1", "9002", 3, 3, 3)
    go cassio.NewNode("127.0.0.1", "9003", 3, 3, 3)
    go cassio.NewNode("127.0.0.1", "9004", 3, 3, 3)

    // connect peers in network
    start.FindFriends([]string{
        "127.0.0.1:9000",
        "127.0.0.1:9001",
        "127.0.0.1:9002",
        "127.0.0.1:9003",
        "127.0.0.1:9004",
    })

    // Connect client to cassiopeia network
    c := client.NewClient("127.0.0.1:9000")
    c.Connect()

    // Writes are very fast
    c.Set("ryan", "jiang")
    c.Set("password", "pass1234")
    c.Set("small", "mouse")
    c.Set("big", "cat")
    c.Set("chairs", "wood")
    c.Set("ryan", "jiang")
    c.Set("password", "pass1234")
    c.Set("small", "mouse")
    c.Set("big", "cat")
    c.Set("chairs", "wood")
    c.Set("ryan", "jiang")
    c.Set("password", "pass1234")
    c.Set("small", "mouse")
    c.Set("big", "cat")
    c.Set("chairs", "a")
    c.Set("big", "b")
    c.Set("chairs", "c")
    c.Set("ryan", "d")
    c.Set("password", "e")
    c.Set("small", "f")
    c.Set("big", "g")
    c.Set("chairs", "h")

    ryan, ok1 := c.Get("ryan")
    password, ok2 := c.Get("password")
    tomato, ok3 := c.Get("tomato")

    fmt.Println("\t\t$$$ ryan ->", ryan, ok1)
    fmt.Println("\t\t$$$ password ->", password, ok2)
    fmt.Println("\t\t$$$ not found tomato ->", tomato, ok3)

    c.Del("ryan")
    ryan, ok1 = c.Get("ryan")

    fmt.Println("\t\t$$$ ryan ->", ryan, ok1)

    // Close connection, flush whatever is still buffered
    c.Disconnect()
}
```

## Internals Overview

Cassiopeia is a peer-to-peer distributed hash table with tunable consistency and performance under node failure. As with Cassandra, each peer maintains a membership list of all other nodes. Cassio uses gossip style heartbeatings, asynchronous queries to avoid timeout deplays from dead friends. Cassiopeia's base approach is simular to other DHTs. Keys and node IDs are both in the same bit key space, though I used 640-bit instead of 160-bits. Key, value pairs are stored in the friends with the largest simularity. Cassio converts keys and IDs to normalized vectors and uses inner product as the simularity metric. The most "simular" node id is mapped to a peer in a "ring" of peers, simular to the ring in Cassandra or Cord. This ring is sorted with similar node IDs together, the next *X* friends are used to store the <k, v>. Where *X* can be tuned for appropriate consistency vs avalibility trade-offs.

## Writes

 1. Client requests write to some arbitrary node C in P2P cluster. 
  > C acts as the coordinator node.
 2. C maps key to set of nodes in ring.
    > map := key is fnv style hashed to 10 dim vector, C has a membership 
list of live nodes through gossip, C applys similarity metric between
key vec and normalized vec of peer ids. P2P cluster has total order,
hence can be treated as both a ring to find next x peers, or a tree
if we wanted to implement practical all-reduce.
3. C requests write to replica nodes R<sub>i</sub>
4. Every node R<sub>i</sub> writes to mem-table cache and bloom-filter.

**Write is complete!**  


## Read
 1. Client requests read to some arbitrary node C in P2P cluster. 
  > C acts as the coordinator node.
 2. C maps key to set of nodes in the ring.
    > map := same mapping as with write
3. C requests read to replica nodes R<sub>i</sub>
4. Every node R<sub>i</sub> does:
    + If memtable has key return val
    + If bloom filter says key is not a member of the set, return none
    + (not in mem, bf says might be on disk) check del log, if deleted return none
    + (might be on disk, but cannot fit in mem) stream through disk, earliest to latest streaming ensures correctness
    + If not found through streaming return none
5. Every node R<sub>i</sub> responds with their value for key to C

    > Peer msg that require response include a unique id, which is hashed to a channel in a non-mutating array referencing channels. Worker threads randomly resolve requests spawn response-handlers. This avoids reads in the reader-writer problem requiring subsequent reads. This way writes are guarenteed to eventually happen :D
    
7. C collects votes and responses with the most popular

**Read is complete!**

