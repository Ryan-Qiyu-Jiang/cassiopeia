package membership

import (
	"cassiopeia/set"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Heartbeat is a heart-beat object to track peer livelyness
type Heartbeat struct {
	hbCounter int
	timeStamp int
	conn      *net.Conn
}

// GetCounter is getter for hbCounter
func (hb *Heartbeat) GetCounter() int {
	return hb.hbCounter
}

// GetTimeStamp is getter for timeStamp
func (hb *Heartbeat) GetTimeStamp() int {
	return hb.timeStamp
}

// SetConn setter for hearbeat conneciton
func (hb *Heartbeat) SetConn(newConn *net.Conn) {
	hb.conn = newConn
}

// GetConn getter for heartbeat connection
func (hb *Heartbeat) GetConn() *net.Conn {
	return hb.conn
}

// NewHeartbeat hb singleton factory
func NewHeartbeat() *Heartbeat {
	return &Heartbeat{hbCounter: 0, timeStamp: int(time.Now().UnixNano()), conn: nil}
}

// MList is a membership list object that maps addr of peer to hb
type MList map[string]*Heartbeat

// Membership is a object to represent membership list of peers
type Membership struct {
	list     MList
	addrs    set.OrderedSet
	hostAddr string
	dead     int
	kill     int
	time     int
}

// GetList getter for list
func (m *Membership) GetList() MList {
	return m.list
}

// GetAddrs getter for addrs
func (m *Membership) GetAddrs() *set.OrderedSet {
	return &m.addrs
}

// GetHost getter for hostAddr
func (m *Membership) GetHost() string {
	return m.hostAddr
}

// Merge merges in new membership object
func (m *Membership) Merge(newM Membership) (*[]string, *[]string) {
	m.time = int(time.Now().UnixNano())
	grave := []string{}
	babies := []string{}
	for addr, hb2 := range newM.list {
		hb1 := m.list[addr]

		if hb1 == nil {
			if hb2.timeStamp+newM.dead > newM.time {
				babies = append(babies, addr)
				m.addrs.Add(addr)
				newHb := NewHeartbeat()
				newHb.hbCounter = (*hb2).hbCounter
				newHb.timeStamp = (*hb2).timeStamp
				newHb.conn, _ = newConn(addr)
				m.list[addr] = newHb
			}
		} else {
			c1 := (*hb1).hbCounter
			c2 := (*hb2).hbCounter

			if c2 > c1 {
				(*hb1).hbCounter = (*hb2).hbCounter
				(*hb1).timeStamp = m.time
			} else {
				if hb1.timeStamp+m.dead+m.kill < m.time {
					delete(m.list, addr)
					grave = append(grave, addr)
					m.addrs.Remove(addr)
				}
			}
		}
	}
	return &grave, &babies
}

func newConn(host string) (*net.Conn, bool) {
	var err error
	conn := new(net.Conn)
	*conn, err = net.Dial("tcp", host)
	if err != nil {
		fmt.Println("cannot connect to", host)
		return nil, false
	}
	fmt.Println("connected!")
	return conn, true
}

// UpdateSelf updates self heartbeat in membership list
func (m *Membership) UpdateSelf() {
	hb := m.list[m.hostAddr]
	(*hb).hbCounter++
	(*hb).timeStamp = int(time.Now().UnixNano())
	m.time = int(time.Now().UnixNano())
}

// Print utility func
func (m *Membership) Print() {
	fmt.Println("MSL of", m.hostAddr, "at", m.time)
	for addr, hb := range m.list {
		fmt.Printf(">  addr: %s | hbc: %d, ts: %d\n", addr, (*hb).hbCounter, (*hb).timeStamp)
	}
}

// ToString serilize
func (m *Membership) ToString() string {
	serial := ""
	serial += m.hostAddr + ">" + strconv.Itoa(m.time) + ">"
	for addr, hb := range m.list {
		serial += fmt.Sprintf("%s|%d|%d,", addr, (*hb).hbCounter, (*hb).timeStamp)
	}
	return serial
}

// DeSerial deserializes string to membership object
func (m *Membership) DeSerial(serial string) *Membership {
	a := strings.Split(serial, ">")
	host := a[0]
	time, err := strconv.Atoi(a[1])
	if err != nil {
		panic(err)
	}
	b := a[2]
	list := make(MList)
	hbStrs := strings.Split(b, ",")

	for _, hbStr := range hbStrs {
		if hbStr == "" {
			break
		}
		c := strings.Split(hbStr, "|")
		addr := c[0]
		var hbCounter int
		var timeStamp int
		var err error
		//fmt.Printf(">>> %s %s %s\n", c[0], c[1], c[2])
		hbCounter, err = strconv.Atoi(c[1])
		if err != nil {
			fmt.Println(err)
		}
		timeStamp, err = strconv.Atoi(c[2])
		if err != nil {
			fmt.Println(err)
		}
		newHb := NewHeartbeat()
		newHb.hbCounter = hbCounter
		newHb.timeStamp = timeStamp
		list[addr] = newHb
	}
	newMSL := NewMembership(host, []string{})
	newMSL.list = list
	newMSL.time = time
	return newMSL
}

// AddMember adds a member to the msl with conn
func (m *Membership) AddMember(addr string) {
	newHb := NewHeartbeat()
	newHb.conn, _ = newConn(addr)
	m.list[addr] = newHb
	m.addrs.Add(addr)
	m.time = int(time.Now().UnixNano())
}

// ConnAll refreshes connections to all members
func (m *Membership) ConnAll() {
	wg := sync.WaitGroup{}
	for addr, hb := range m.list {
		wg.Add(1)
		go func() {
			hb.conn, _ = newConn(addr)
			wg.Done()
		}()
		wg.Wait()
	}
}

// Retry tryes to reconnect to peer, returns success fail bool
func (m *Membership) Retry(addr string) bool {
	hb, ok := m.list[addr]
	if !ok {
		fmt.Println("does not exist", addr)
		return false
	}
	conn, connected := newConn(addr)
	hb.conn = conn
	return connected
}

// IsAlone checks if only host is alive
func (m *Membership) IsAlone() bool {
	curTime := int(time.Now().UnixNano())
	m.time = curTime
	for addr, hb := range m.list {
		ts := hb.timeStamp
		if ts+m.dead+10*m.kill > curTime && addr != m.hostAddr {
			return false
		}
	}
	return true
}

// KillAllMyFriends , read the title.
func (m *Membership) KillAllMyFriends() {
	newList := make(MList)
	newAddrs := set.NewSet()
	newAddrs.Add(m.hostAddr)
	newHb := NewHeartbeat()
	newList[m.hostAddr] = newHb
	m.list = newList
	m.addrs = *newAddrs
}

// Remove dels member from msl
func (m *Membership) Remove(addr string) {
	delete(m.list, addr)
	m.addrs.Remove(addr)
}

// NewMembership membership singleton factory
func NewMembership(host string, addrs []string) *Membership {
	list := make(MList)
	ordered := set.NewSet()
	curTime := int(time.Now().UnixNano())
	for _, addr := range addrs {
		newHb := NewHeartbeat()
		newHb.timeStamp = curTime
		list[addr] = newHb
		ordered.Add(addr)
	}
	return &Membership{
		list:     list,
		addrs:    *ordered,
		hostAddr: host,
		dead:     20000000000,
		kill:     20000000000,
		time:     int(time.Now().UnixNano())}
}

func main() {
	addrs := []string{"1", "2", "3"}
	msl1 := NewMembership("1", addrs)
	msl2 := NewMembership("2", addrs)
	msl3 := NewMembership("3", addrs)

	(*msl1).Print()
	(*msl2).Print()
	(*msl3).Print()
	time.Sleep(time.Millisecond * 500)
	(*msl2).UpdateSelf()
	(*msl1).Merge(*msl2)
	fmt.Println("________________-")
	(*msl1).Print()
	(*msl2).Print()
	(*msl3).Print()

	fmt.Println(msl1.ToString())

	msl4 := msl1.DeSerial(msl1.ToString())
	msl4.Print()

	msl5 := NewMembership("5", []string{"5"})
	a, b := msl1.Merge(*msl5)
	fmt.Println(*a, *b)
	msl1.Print()

	fmt.Println()
}
