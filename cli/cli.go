package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
)

func main() {
	var ip string
	var port string
	if len(os.Args) > 1 {
		ip = os.Args[1]
		port = os.Args[2]
	} else {
		ip = "localhost"
		port = "9000"
	}

	conn, err := net.Dial("tcp", ip+":"+port)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			text, _ := reader.ReadString('\n')
			io.WriteString(conn, text[0:len(text)-2]+";")
		}
	}()

	go func() {
		bs := make([]byte, 1024)
		for {
			n, err := conn.Read(bs)
			if err != nil {
				panic(err)
			}
			fmt.Println("> " + string(bs[:n]))
		}
	}()

	select {}

}
