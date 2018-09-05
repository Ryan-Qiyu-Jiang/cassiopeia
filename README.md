# Cassiopeia

*Like Cassandra but worse.*
Cassiopeia is a distributed hash-table based off Cassandra, with pieces of other systems.
Written in go, it also has a **client**, **cli** and importable **package** runnable on single and multiple machines.

Cassio offers **strong consistency** while prioritizing *fast writes*. 
Cassiopeia is **fault-tolerent**, and *immediately* updates revived nodes.
Cassiopeia is **elastic** and easily supports birth and death of peers.

It is architexturally simular to Cassandra, 
*  without SSLtable optimization (yet), 
* with Kademlia inspired peer finding, 
* with new node initiation instead of coordinator hinted hand-off. 

*Also it's writen by one cs student in the weekend between work and school.*


## Composition:

 - Async Event loop for requests
 - Jobs, workers for tunable response-proccessing between peers
 - Custom Gossip-Style Heartbeating for membership lists
 - Bloom filters to optimize reads
 - Memtable cache flushes and squashes to disk
 - Exponential falloff for automatic node recovery
 - NN based consistant-hashing regardless of current peers
 - Tunable consistancy
 - Lots of reader-writer optimizations
 
 And it's still pretty easy to read.


## Using Cassiopeia

Cassiopeia on k8s is in the works, should just be a helm-chart deployment.
Written in go, Cassio can be compiled to an executable binary for any platform, look for the exe branch for compilable version.

*Running Cassio on a single node for development is easy.*

 1. Import pkg from github 
 2. Make a thread for the node 
 3. Instantiate the client 
 4. Have a blast :)

You can also run multiple Cassio peers on a single node with the Cassiopeia pkg.
1. Make a thread for each node
2. Run the init function
3. Instantiate the client 

The more friends the merrier :D especially when you moved  in too early and you're the only one of your friends in the city right now.


### Example: Single Peer
```go
package main

import  (
"fmt"

"github.com/Ryan-Qiyu-Jiang/cassiopeia"
"github.com/Ryan-Qiyu-Jiang/cassiopeia/client"
)

func  main()  {
	go node.NewNode("127.0.0.1",  "9000")
	c  := client.NewClient("127.0.0.1:9000")

	c.Connect()

	c.Set("ryan",  "jiang")
	c.Set("password",  "pass1234")

	ryan,  ok1  := c.Get("ryan")
	password,  ok2  := c.Get("password")
	tomato,  ok3  := c.Get("tomato")

	fmt.Println("\t\t$$$ ryan ->", ryan, ok1)
	fmt.Println("\t\t$$$ password ->", password, ok2)
	fmt.Println("\t\t$$$ not found tomato ->", tomato, ok3)

	c.Del("ryan")
	ryan,  ok1  = c.Get("ryan")

	fmt.Println("\t\t$$$ ryan ->", ryan, ok1)

	c.Disconnect()
}
```

### Example: Multiple Peers
```go
package main

import  (
"fmt"

"github.com/Ryan-Qiyu-Jiang/cassiopeia"
"github.com/Ryan-Qiyu-Jiang/cassiopeia/client"
"github.com/Ryan-Qiyu-Jiang/cassiopeia/init"
)

func  main()  {
	go node.NewNode("127.0.0.1",  "9000")
	go node.NewNode("127.0.0.1",  "9001")
	go node.NewNode("127.0.0.1",  "9002")
	
	start.FindFriends([]string{
	"127.0.0.1:9000",
	"127.0.0.1:9001",
	"127.0.0.1:9002",
	})

	c  := client.NewClient("127.0.0.1:9000")
	c.Connect()

	c.Set("ryan",  "jiang")
	c.Set("password",  "pass1234")
	c.Set("small",  "mouse")
	c.Set("big",  "cat")
	c.Set("chairs",  "wood")
	c.Set("ryan",  "jiang")
	c.Set("password",  "pass1234")
	c.Set("small",  "mouse")
	c.Set("big",  "cat")
	c.Set("chairs",  "wood")
	c.Set("ryan",  "jiang")
	c.Set("password",  "pass1234")
	c.Set("small",  "mouse")
	c.Set("big",  "cat")
	c.Set("chairs",  "a")
	c.Set("big",  "b")
	c.Set("chairs",  "c")
	c.Set("ryan",  "d")
	c.Set("password",  "e")
	c.Set("small",  "f")
	c.Set("big",  "g")
	c.Set("chairs",  "h")
	  
	ryan,  ok1  := c.Get("ryan")
	password,  ok2  := c.Get("password")
	tomato,  ok3  := c.Get("tomato")

	fmt.Println("\t\t$$$ ryan ->", ryan, ok1)
	fmt.Println("\t\t$$$ password ->", password, ok2)
	fmt.Println("\t\t$$$ not found tomato ->", tomato, ok3)

	c.Del("ryan")
	ryan,  ok1  = c.Get("ryan")

	fmt.Println("\t\t$$$ ryan ->", ryan, ok1)

	c.Disconnect()
}
```


## Writes

 1. Client requests write to some arbitrary node C in P2P cluster.   
     > C acts as coordinator node.
 2. C maps key to set of nodes in ring.
	> map := key is fnv style hashed to 10 dim vector, C has membership    
   list of live nodes through gossip, C applys similarity metric between
   key vec and normalized vec of peer ids. P2P cluster has total order, 
   hence can be treated as both a ring to find next x peers, or a tree  
   if we wanted to implement practical all-reduce.
3. C requests write to replica nodes R<sub>i</sub>
4. Every node R<sub>i</sub> writes to mem-table cache and bloom-filter.

**Write is complete!**  


## Read
 1. Client requests read to some arbitrary node C in P2P cluster.   
     > C acts as coordinator node.
 2. C maps key to set of nodes in ring.
	> map := same mapping as with write
3. C requests read to replica nodes R<sub>i</sub>
4. Every node R<sub>i</sub> does:
	+ If memtable has key return val
	+ If bloom filter says key is not a member of the set, return none
	+ (not in mem, bf says might be on disk) check del log, if deleted return none
	+ (might be on disk, but cannot fit in mem) stream through disk, earlist to latest streaming ensures correctness
	+ If not found through streaming return none
5. Every node R<sub>i</sub> responds with their value for key to C

	> Peer msg that require response include a unique id, which is hashed to a channel in a non-mutating array referencing channels. Worker threads randomly resolve requests spawn response-handlers. This avoids reads in the reader-writer problem requiring subsequent reads. This way writes are guarenteed to eventually happen :D
	
7. C collects votes and responses with most popular

**Read is complete!**

