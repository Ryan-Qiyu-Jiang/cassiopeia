package main

import (
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"math/rand"
	"net"
	"node/db"
	ms "node/membership"
	"node/set"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const readMin = 2
const readNum = 2
const writeNum = 2
const chanNum = 5

// Member is ht of peer node tcp connections
type Member map[string]*net.Conn

// Members is object with connection ht and slice of peer addrs
type Members struct {
	conns Member
	addrs set.OrderedSet
}

func newMembers() *Members {
	return &Members{conns: Member{}, addrs: *set.NewSet()}
}

// NewNode starts a replica node proccess
func NewNode(ip string, port string) {
	var mMSL sync.RWMutex
	var mDB sync.RWMutex

	var reqID uint64

	host := ip + ":" + port

	hostInt, err := strconv.Atoi(port)
	db := *(db.NewDB(hostInt))
	db.Load()
	db.Flush()
	// load+Flush in case of previous failure with straggling del_log

	msl := *(ms.NewMembership(host, []string{host}))

	resChans := [](chan string){}

	for i := 0; i < chanNum; i++ {
		c := make(chan string, 1)
		resChans = append(resChans, c)
	}

	var mResChans sync.RWMutex
	go heartBeating(&db, &mDB, &msl, &mMSL, &reqID, &resChans)

	ln, err := net.Listen("tcp", host)
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				panic(err)
			}
			go proccess(&conn, &msl, &mMSL, &db, &mDB, &resChans, &mResChans, &reqID)
		}
	}()

	go func() {
		time.Sleep(time.Millisecond * 120)
		msl.ConnAll()
		proccessRes(&msl, &mMSL, &resChans, &mResChans)
	}()
	select {}
}

func main() {
	var ip string
	var port string
	if len(os.Args) > 1 {
		ip = os.Args[1]
		port = os.Args[2]
	} else {
		ip = "127.0.0.1"
		port = "9000"
	}
	NewNode(ip, port)
}

func getUCID(reqID *uint64) int {
	id := int(atomic.LoadUint64(reqID))
	atomic.AddUint64(reqID, 1)
	return id % chanNum
}

func cleanChan(c chan string) {
	for {
		select {
		case <-c:
		default:
			return
		}
	}
}

func mPrint(m Members) {
	for _, addr := range m.addrs.Arr {
		fmt.Println("    index", addr)
	}
}

func heartBeating(
	db *db.DB,
	mDB *sync.RWMutex,
	msl *ms.Membership,
	mMSL *sync.RWMutex,
	reqID *uint64,
	resChans *[](chan string)) {
	first := true
	for {
		mMSL.Lock()
		msl.UpdateSelf()
		if msl.IsAlone() && len(msl.GetList()) > 1 {
			msl.KillAllMyFriends()
			fmt.Println("probably being ghosted, killing my friends")
		}
		mMSL.Unlock()
		mMSL.RLock()
		serial := msl.ToString()
		counter := len(msl.GetList()) / 2
		mslCopy := msl.GetList()
		mMSL.RUnlock()
		for addr := range mslCopy {
			if addr == msl.GetHost() {
				continue
			}
			if counter == 0 {
				break
			} else {
				counter--
			}
			id := getUCID(reqID)
			cleanChan((*resChans)[id])
			msg := fmt.Sprintf("alive?/%s/%d;", serial, id)

			mMSL.RLock()
			if msl.GetList()[addr] == nil {
				continue
			}
			conn := msl.GetList()[addr].GetConn()
			mMSL.RUnlock()
			if conn == nil {
				continue
			}
			(*conn).Write([]byte(msg))
			res := ""
			select {
			case res = <-(*resChans)[id]:

			case <-time.After(10 * time.Second):
				fmt.Printf("%d to %s hb timed out\n", id, addr)
				continue
			}
			msl2 := msl.DeSerial(res)

			mMSL.Lock()
			msl.Merge(*msl2)
			fmt.Println("heartbeat", addr, msl.GetAddrs().Arr)
			mMSL.Unlock()
			if first {
				first = false
				go grabDB(mDB, db, mMSL, msl, reqID, resChans)
			}
		}
		if len(mslCopy) == 1 {
			continue
		}
		time.Sleep(time.Millisecond * 2000)
	}
}

func grabDB(mDB *sync.RWMutex, db *db.DB, mMSL *sync.RWMutex, msl *ms.Membership, reqID *uint64, resChans *[](chan string)) {
	mMSL.RLock()
	index := msl.GetAddrs().Find(msl.GetHost())
	numPeers := len(msl.GetList())
	mMSL.RUnlock()
	id := getUCID(reqID)
	cleanChan((*resChans)[id])
	msg := fmt.Sprintf("_getDB/%d", id)
	friend := ""
	for {
		index++
		mMSL.RLock()
		friend = msl.GetAddrs().Arr[index%numPeers]
		conn := msl.GetList()[friend].GetConn()
		mMSL.RUnlock()
		if conn == nil {
			continue
		}
		fmt.Println("fetching db from friend")
		(*conn).Write([]byte(msg))
		break
	}

	res := ""
	select {
	case res = <-(*resChans)[id]:

	case <-time.After(10 * time.Second):
		fmt.Printf("%d getdb from %s timed out\n", id, friend)
		return
	}
	mDB.Lock()
	db.Append(string(res))
	mDB.Unlock()
}

func proccessRes(
	msl *ms.Membership,
	mMSL *sync.RWMutex,
	resChans *[](chan string),
	mResChans *sync.RWMutex) {
	time.Sleep(time.Millisecond * 500)
	go resWorker(msl, mMSL, resChans, mResChans)
	go resWorker(msl, mMSL, resChans, mResChans)
	go resWorker(msl, mMSL, resChans, mResChans)
	go resWorker(msl, mMSL, resChans, mResChans)
	go resWorker(msl, mMSL, resChans, mResChans)
	select {}
}

func resWorker(
	msl *ms.Membership,
	mPeers *sync.RWMutex,
	resChans *[](chan string),
	mResChans *sync.RWMutex) {
	seed := rand.NewSource(time.Now().UnixNano())
	rGen := rand.New(seed)
	for {
		mPeers.RLock()
		numPeers := len(msl.GetList())
		index := rGen.Intn(numPeers)

		addr := msl.GetAddrs().Arr[index]
		hb := msl.GetList()[addr]
		conn := hb.GetConn()

		if conn == nil {
			fmt.Println("nope", addr)
			mPeers.RUnlock()
			continue
		}
		// read with timeout
		bs := make([]byte, 1024)
		n := 0
		err := (*conn).SetReadDeadline(time.Now().Add(1 * time.Second))
		if err != nil {
			fmt.Println("SetReadDeadline failed:", err)
			return
		}
		n, err = (*conn).Read(bs)
		mPeers.RUnlock()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				//fmt.Println("nothing to read yet")
				continue
			} else {
				fmt.Println("Connection failed to respond", addr)
				go reconnect(addr, mPeers, msl)
				continue
			}
		}

		res := string(bs[:n])
		msgs := strings.Split(res, ";")
		for _, msg := range msgs {
			if msg == "" {
				continue
			}
			resArr := strings.Split(msg, "/")
			id, err := strconv.Atoi(resArr[0])
			if err != nil {
				fmt.Println("what is this??", err)
				continue
			}
			args := resArr[1:]
			(*resChans)[id] <- strings.Join(args, "/")
		}
	}
}

func proccess(
	conn *net.Conn,
	msl *ms.Membership,
	mMSL *sync.RWMutex,
	db *db.DB,
	mDB *sync.RWMutex,
	resChans *[](chan string),
	mResChans *sync.RWMutex,
	reqID *uint64) {
	defer (*conn).Close()
	bs := make([]byte, 1024)
	var n int

	for {
		fmt.Println(">")
		a, b := (*conn).Read(bs)
		if b != nil {
			fmt.Println("Connection closed before reponse", b)
			break
		}
		n = a

		msgs := strings.Split(string(bs[:n]), ";")
		fmt.Println("<", msgs, len(msgs))
		for _, sMsg := range msgs {
			if sMsg == "" {
				continue
			}
			aMsg := strings.Split(sMsg, "/")

			if aMsg[0] == "new" {
				fmt.Println("new:", strings.Join(aMsg[1:], ", "))
				go newHandler(aMsg, mMSL, msl)

			} else if aMsg[0] == "ping" {
				go pingHandler(aMsg, mMSL, msl)

			} else if aMsg[0] == "set" {
				go setHandler(aMsg, mMSL, msl, mDB, db)

			} else if aMsg[0] == "_set" {
				go _setHandler(aMsg, mDB, db)

			} else if aMsg[0] == "del" {
				go delHandler(aMsg, mMSL, msl, mDB, db)

			} else if aMsg[0] == "_del" {
				go _delHandler(aMsg, mDB, db)

			} else if aMsg[0] == "flush" {
				go flushHandler(conn, mDB, db)

			} else if aMsg[0] == "get" {
				go getHandler(aMsg, conn, mMSL, msl, mResChans, resChans, reqID, mDB, db)

			} else if aMsg[0] == "_get" {
				go _getHandler(aMsg, conn, mDB, db)

			} else if aMsg[0] == "_getDB" {
				go _getDBHandler(aMsg, conn, mDB, db)

			} else if aMsg[0] == "ack" {
				go func() {
					(*conn).Write([]byte("ack"))
				}()

			} else if aMsg[0] == "alive?" {
				go aliveHandler(aMsg, conn, mMSL, msl)

			} else {
				go defaultHandler(aMsg, mMSL, msl)
			}
		}
	}
}

// reconnect tries to retry with exponetial backoff, then removes member
func reconnect(addr string, mMSL *sync.RWMutex, msl *ms.Membership) bool {
	mMSL.RLock()
	_, ok := msl.GetList()[addr]
	mMSL.RUnlock()
	if !ok {
		fmt.Println("does not exist", addr)
		return false
	}
	for i := 0; i < 10; i++ {
		mMSL.Lock()
		success := msl.Retry(addr)
		mMSL.Unlock()
		if success {
			return success
		}
		time.Sleep(time.Millisecond * time.Duration(math.Exp(float64(i))))
	}
	mMSL.Lock()
	msl.Remove(addr)
	mMSL.Unlock()
	return false
}

func aliveHandler(
	aMsg []string,
	conn *net.Conn,
	mMSL *sync.RWMutex,
	msList *ms.Membership) {
	if len(aMsg) < 3 {
		fmt.Println("bad cmd", aMsg)
		return
	}
	id := aMsg[2]
	serial1 := aMsg[1]

	mMSL.Lock()
	msList.UpdateSelf()
	serial2 := msList.ToString()
	mMSL.Unlock()

	msg := fmt.Sprintf("%s/%s;", id, serial2)
	(*conn).Write([]byte(msg))

	msl1 := msList.DeSerial(serial1)

	mMSL.Lock()
	msList.Merge(*msl1)
	//fmt.Println("r connections", msList.GetAddrs().Arr)
	mMSL.Unlock()

}

func newHandler(aMsg []string, m *sync.RWMutex, msl *ms.Membership) {
	newHost := aMsg[1]
	m.Lock()
	msl.AddMember(newHost)
	msl.Print()
	m.Unlock()
}

func pingHandler(aMsg []string, m *sync.RWMutex, msl *ms.Membership) {
	msg := aMsg[1]
	m.RLock()
	for _, v := range msl.GetList() {
		if len(aMsg) == 1 {
			panic("u r bad, what msg?")
		}
		go io.WriteString(*v.GetConn(), msg)
	}
	m.RUnlock()
}

func setHandler(aMsg []string, m *sync.RWMutex, msl *ms.Membership, mDB *sync.RWMutex, db *db.DB) {
	if len(aMsg) < 3 {
		fmt.Println("bad cmd", aMsg)
		return
	}
	m.RLock()
	numPeers := len(msl.GetList())
	closestAddr := hash(aMsg[1], msl.GetAddrs().Arr)
	peerIndex := msl.GetAddrs().Find(closestAddr)
	if peerIndex < 0 {
		panic("cannot find peer")
	}
	for i := peerIndex; i < peerIndex+writeNum; i++ {
		index := i % numPeers
		addr := msl.GetAddrs().Arr[index]
		setMsg := fmt.Sprintf("_set/%s/%s;", aMsg[1], aMsg[2])
		if addr == msl.GetHost() {
			_setHandler([]string{"_set", aMsg[1], aMsg[2]}, mDB, db)
			fmt.Println("set_self:", addr, aMsg[1])
			continue
		}
		conn := msl.GetList()[addr].GetConn()

		if conn != nil {
			fmt.Println("set:", addr, aMsg[1])
			go io.WriteString(*conn, setMsg)
		} else {
			fmt.Println("not set(reconnecting):", addr, aMsg[1])
		}

	}
	m.RUnlock()
}

func _setHandler(aMsg []string, mDB *sync.RWMutex, db *db.DB) {
	k := aMsg[1]
	v := aMsg[2]
	mDB.Lock()
	db.Set(k, v)
	mDB.Unlock()
}

func delHandler(aMsg []string, m *sync.RWMutex, msl *ms.Membership, mDB *sync.RWMutex, db *db.DB) {
	m.RLock()
	numPeers := len(msl.GetList())
	closestAddr := hash(aMsg[1], msl.GetAddrs().Arr)
	peerIndex := msl.GetAddrs().Find(closestAddr)
	if peerIndex < 0 {
		panic("cannot find peer")
	}
	for i := peerIndex; i < peerIndex+writeNum; i++ {
		index := i % numPeers
		addr := msl.GetAddrs().Arr[index]
		delMsg := fmt.Sprintf("_del/%s;", aMsg[1])
		if addr == msl.GetHost() {
			_delHandler([]string{"_del", aMsg[1]}, mDB, db)
			fmt.Println("del_self:", addr, aMsg[1])
			continue
		}
		conn := msl.GetList()[addr].GetConn()

		if conn != nil {
			fmt.Println("del:", addr, aMsg[1])
			go io.WriteString(*conn, delMsg)
		} else {
			fmt.Println("not del(reconnecting):", addr, aMsg[1])
		}
	}
	m.RUnlock()
}

func _delHandler(aMsg []string, mDB *sync.RWMutex, db *db.DB) {
	k := aMsg[1]
	mDB.Lock()
	db.Remove(k)
	mDB.Unlock()
}

func flushHandler(conn *net.Conn, mDB *sync.RWMutex, db *db.DB) {
	mDB.RLock()
	for k, v := range (*db).GetHT() {
		fmt.Printf("    %s:%s\n", k, v)
		msg := fmt.Sprintf("    %s:%s", k, v)
		(*conn).Write([]byte(msg))
	}
	mDB.RUnlock()
}

func getHandler(
	aMsg []string,
	conn *net.Conn,
	mMSL *sync.RWMutex,
	msl *ms.Membership,
	mResChans *sync.RWMutex,
	resChans *[](chan string),
	reqID *uint64,
	mDB *sync.RWMutex,
	db *db.DB) {

	var wg sync.WaitGroup

	mMSL.RLock()
	numPeers := len(msl.GetList())
	closestAddr := hash(aMsg[1], msl.GetAddrs().Arr)
	peerIndex := msl.GetAddrs().Find(closestAddr)
	result := make(chan []byte, readNum)

	for i := peerIndex; i < peerIndex+readNum; i++ {
		index := i % numPeers
		addr := msl.GetAddrs().Arr[index]
		pConn := msl.GetList()[addr].GetConn()

		if addr == msl.GetHost() {
			k := aMsg[1]
			fmt.Println("get_self:", addr, aMsg[1])
			v := ""
			ok := false
			mDB.RLock()
			v, ok = (*db).Get(k)
			mDB.RUnlock()
			if !ok {
				fmt.Println("not found,", k)
				continue
			}
			result <- []byte(v)
			continue
		}

		fmt.Println("get:", addr, aMsg[1])
		wg.Add(1)

		go func(mResChans *sync.RWMutex, wg *sync.WaitGroup) {
			id := getUCID(reqID)
			cleanChan((*resChans)[id])
			getMsg := fmt.Sprintf("_get/%s/%d;", aMsg[1], id)

			if pConn != nil {
				io.WriteString(*pConn, getMsg)
			}

			res := ""
			select {
			case res = <-(*resChans)[id]:

			case <-time.After(3 * time.Second):
				fmt.Printf("%d timed out.\n", id)
				wg.Done()
				return
			}

			result <- []byte(res)

			wg.Done()
		}(mResChans, &wg)
	}

	mMSL.RUnlock()
	wg.Add(1)
	go func() {
		response := []byte{}
		votes := make(map[string]int)
		for f := 0; f < readMin; f++ {
			select {
			case temp := <-result:
				res := string(temp)
				if res == "" {
					continue
				}
				c, ok := votes[res]
				if ok {
					votes[res] = c + 1
				} else {
					votes[res] = 1
				}

			case <-time.After(2 * time.Second):
				fmt.Printf("timed out.\n")
			}
		}
		max := ""
		if len(votes) == 0 {
			fmt.Printf("no answers")
		} else {
			for max = range votes {
			}
			for res, count := range votes {
				if count > votes[max] {
					max = res
				}
			}
			response = []byte(max)
		}
		fmt.Println("votes:", votes, string(response))
		(*conn).Write(response)
		wg.Done()
	}()
	wg.Wait()
}

func _getHandler(aMsg []string, conn *net.Conn, mDB *sync.RWMutex, db *db.DB) {
	k := aMsg[1]
	v := ""
	ok := false
	mDB.RLock()
	v, ok = (*db).Get(k)
	mDB.RUnlock()
	if !ok {
		fmt.Println("not found,", k)
		v = ""
	}
	msg := fmt.Sprintf("%s/%s;", aMsg[2], v)
	fmt.Println("responding:", msg)
	(*conn).Write([]byte(msg))
}

func _getDBHandler(aMsg []string, conn *net.Conn, mDB *sync.RWMutex, db *db.DB) {
	mDB.RLock()
	serial := db.ToString()
	mDB.RUnlock()

	msg := fmt.Sprintf("%s/%s;", aMsg[1], serial)
	fmt.Println("sending db to friend")
	(*conn).Write([]byte(msg))
}

func defaultHandler(aMsg []string, m *sync.RWMutex, msl *ms.Membership) {
	m.RLock()
	fmt.Println("other:", aMsg)
	msl.Print()
	m.RUnlock()
}

func hash(s string, addrs []string) string {
	h := fnv.New32a()
	h.Write([]byte(s))
	a := int(h.Sum32())
	wordVec := intToVec(a)
	m := addrs[0]
	mVal := 0.0
	for _, addr := range addrs {
		nodeVec := addrToVec(addr)
		normVec := norm(nodeVec)
		match := dot(wordVec, normVec)
		if match > mVal {
			mVal = match
			m = addr
		}
	}
	return m
}

func norm(i []int) []float64 {
	vec := []float64{}
	sum := 0
	for _, v := range i {
		sum += v
	}
	for f := 0; f < len(i); f++ {
		vec = append(vec, float64(i[f])/float64(sum))
	}
	return vec
}

func dot(a []int, b []float64) float64 {
	if len(a) != len(b) {
		panic(fmt.Sprintf("length mismatch %d %d", len(a), len(b)))
	}
	sum := 0.0
	for i := 0; i < len(a); i++ {
		sum += float64(a[i]) * b[i]
	}
	return sum
}

func addrToVec(addr string) []int {
	vec := []int{}
	for _, c := range addr {
		if c == ':' || c == '.' {
			continue
		}
		v := int(c - '0')
		vec = append(vec, v)
	}
	num := len(vec)
	for i := num; i < 10; i++ {
		vec = append(vec, 0)
	}
	if num > 10 {
		vec = vec[num-10:]
	}
	return vec
}

func intToVec(n int) []int {
	vec := []int{}
	for n > 0 {
		vec = append(vec, n%10)
		n /= 10
	}
	num := len(vec)
	for i := 0; i < num/2; i++ {
		vec[num-i-1], vec[i] = vec[i], vec[num-i-1]
	}
	for i := num; i < 10; i++ {
		vec = append(vec, 0)
	}
	if num > 10 {
		vec = vec[:10]
	}
	return vec
}

func newConn(host string) *net.Conn {
	var err error
	conn := new(net.Conn)
	*conn, err = net.Dial("tcp", host)
	if err != nil {
		panic(err)
	}
	fmt.Println("connected!")
	return conn
}
