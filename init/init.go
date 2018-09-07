package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

// FindFriends connects peers with addrs in hosts param
func FindFriends(hosts []string) {
	wg := sync.WaitGroup{}
	for _, host := range hosts {
		wg.Add(1)
		go func(host string) {
			conn, err := net.Dial("tcp", host)
			if err != nil {
				panic(err)
			}
			defer conn.Close()
			for _, h := range hosts {
				//if host != h {
				io.WriteString(conn, fmt.Sprintf("new/%s;", h))
				//}
			}
			wg.Done()
		}(host)
	}
	wg.Wait()
	time.Sleep(time.Second * 2)
}

func main() {
	var hosts []string
	if len(os.Args) > 1 {
		for _, host := range os.Args[1:] {
			hosts = append(hosts, host)
		}
	} else {
		hosts = []string{"localhost:9000"}
	}
	wg := sync.WaitGroup{}
	for _, host := range hosts {
		wg.Add(1)
		go func(host string) {
			conn, err := net.Dial("tcp", host)
			if err != nil {
				panic(err)
			}
			defer conn.Close()
			for _, h := range hosts {
				//if host != h {
				io.WriteString(conn, fmt.Sprintf("new/%s;", h))
				//}
			}
			wg.Done()
		}(host)
	}
	wg.Wait()

}
