package client

import (
	"fmt"
	"net"
	"time"
)

// client methods do not work concurently
// db responses are also not tagged :p
// also don't use /,; in key vals pls :D

// Client wrapper object
type Client struct {
	host string
	conn *net.Conn
}

// Connect connects to cluster
func (c *Client) Connect() {
	conn := new(net.Conn)
	var err error
	*conn, err = net.Dial("tcp", c.host)
	if err != nil {
		panic(err)
	}
	c.conn = conn
}

// GetHost host getter
func (c *Client) GetHost() string {
	return c.host
}

// Disconnect connects to cluster
func (c *Client) Disconnect() {
	(*c.conn).Close()
}

// Set no response
func (c *Client) Set(key string, val string) bool {
	msg := fmt.Sprintf("set/%s/%s;", key, val)
	if c.conn == nil {
		return false
	}
	(*c.conn).Write([]byte(msg))
	return true
}

// Get blocking
func (c *Client) Get(key string) (string, bool) {
	msg := fmt.Sprintf("get/%s;", key)
	if c.conn == nil {
		return "", false
	}
	(*c.conn).Write([]byte(msg))
	bs := make([]byte, 2048)

	// read with timeout
	n := 0
	err := (*c.conn).SetReadDeadline(time.Now().Add(1 * time.Second))
	if err != nil {
		fmt.Println("SetReadDeadline failed:", err)
		return "", false
	}
	n, err = (*c.conn).Read(bs)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			fmt.Println("Connection timed out")
			return "", false
		} else {
			fmt.Println("Connection failed to respond")
			return "", false
		}
	}

	res := string(bs[:n])
	if res == "$NONE" {
		fmt.Println("Key not found")
		return "", false
	}
	if err != nil {
		fmt.Println(err)
		return "", false
	}
	return string(bs[:n]), true
}

// Del no response
func (c *Client) Del(key string) bool {
	msg := fmt.Sprintf("del/%s;", key)
	if c.conn == nil {
		return false
	}
	(*c.conn).Write([]byte(msg))
	return true
}

// New no response
func (c *Client) New(host string) bool {
	// if bad host fail
	msg := fmt.Sprintf("new/%s;", host)
	if c.conn == nil {
		return false
	}
	(*c.conn).Write([]byte(msg))
	return true
}

// NewClient singleton factory
func NewClient(host string) *Client {
	//if not valid host return nil
	client := &Client{host: host, conn: nil}
	return client
}

func main() {

}
