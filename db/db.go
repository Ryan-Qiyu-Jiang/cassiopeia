package db

import (
	"bufio"
	"cassiopeia/bloom"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"
)

// DB is object representing this shard of db,
// this includes in-memory ht and other db logic
type DB struct {
	ht            map[string]string
	bf            *bloom.BloomFilter
	id            int
	flushCounter  int
	flushDefault  int
	squashCounter int
	squashDefault int
}

func (db *DB) diskStats() []string {
	path := fmt.Sprintf("db/%d", db.id)
	files, err := ioutil.ReadDir(path)
	if err != nil {
		fmt.Println("err", err)
	}
	names := []string{}
	for _, file := range files {
		if file.Name()[:2] == "ht" {
			names = append(names, file.Name())
		}
	}
	sort.Strings(names)
	numFiles := len(names)
	// reverse
	for i := 0; i < numFiles/2; i++ {
		names[i], names[numFiles-i-1] = names[numFiles-i-1], names[i]
	}
	return names
}

// Set adds new elements to ht
func (db *DB) Set(k string, v string) {
	_, ok := db.ht[k]
	if !ok {
		db.addBF(k)
	}
	db.ht[k] = v
	db.rmDelLog(k)

	db.flushCounter--
	if db.flushCounter < 1 {
		db.flushCounter = db.flushDefault
		db.squashCounter--
		if db.squashCounter == 0 {
			db.squashCounter = db.squashDefault
			db.Squash()
		} else {
			db.Flush()
		}
	}
}

// Updates bf file, adds k to bf
func (db *DB) addBF(k string) {
	db.bf.Add(k)
	db.writeFile(db.bf.ToString(), "bloom")
}

func (db *DB) getBFLog() (string, bool) {
	path := fmt.Sprintf("db/%d/bloom.ryan", db.id)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println("can't find bf log", path)
		return "", false
	}
	data, err := ioutil.ReadFile(path)
	check(err)
	return string(data), true
}

//writeFile write file helper
func (db *DB) writeFile(data string, fileName string) {
	mkdir(fmt.Sprintf("db/%d", db.id))
	f, err := os.Create(fmt.Sprintf("db/%d/%s.ryan", db.id, fileName))
	check(err)

	defer f.Close()

	w := bufio.NewWriter(f)
	_, err = w.WriteString(data)
	check(err)
	w.Flush()
}

func (db *DB) rmDelLog(key string) {
	keyArr, ok := db.getDelLog()
	if !ok {
		return
	}
	for i, k := range keyArr {
		if k == key {
			keyArr[i] = ""
		}
	}
	newData := strings.Join(keyArr, "/")
	db.writeDelLog(newData)
}

func (db *DB) simpleSet(k string, v string) {
	db.ht[k] = v
}

// Get returns value of key in ht and t/f has key
func (db *DB) Get(k string) (string, bool) {
	if db.bf.HasNot(k) {
		fmt.Println("bloom says no")
		return "", false
	}
	v, ok := db.ht[k]
	if ok {
		fmt.Println("found in memtable")
		return v, true
	}

	// bf says maybe has, not in mem try on disk
	// first make sure not removed

	// check del_log
	keyArr, existDelLog := db.getDelLog()
	if existDelLog {
		for _, delKey := range keyArr {
			if k == delKey {
				fmt.Println("was removed")
				return "", false
			}
		}
	}
	// not removed, check ht files
	fileNames := db.diskStats()
	tempDB := NewDB(db.id)
	// latest to oldest order maintains correctness assumptions
	for _, name := range fileNames {
		tempDB.loadFile(name)
		v, ok = tempDB.ht[k]
		if ok {
			fmt.Println("found in ht files")
			return v, true
		}
	}

	// don't have key
	return "", false
}

// Remove deletes elem from ht
func (db *DB) Remove(k string) {
	delete(db.ht, k)
	data, _ := db.getDelLog()
	data = append(data, k)
	newLog := strings.Join(data, "/")
	db.writeDelLog(newLog)
	db.flushCounter--
	if db.flushCounter < 1 {
		db.flushCounter = db.flushDefault
		db.Flush()
	}
}

func (db *DB) writeDelLog(data string) {
	db.writeFile(data, "del_log")
}

// GetHT returns copy of ht
func (db *DB) GetHT() map[string]string {
	return db.ht
}

// ToString returns serialized ht string
func (db *DB) ToString() string {
	serial := ""
	for k, v := range db.ht {
		serial += fmt.Sprintf("%s/%s,", k, v)
	}
	return serial
}

// DeSerial takes serial string and updates ht with it's values
func (db *DB) DeSerial(serial string) {
	ht := make(map[string]string)
	a := strings.Split(serial, ",")
	for _, v := range a {
		if v == "" {
			continue
		}
		kv := strings.Split(v, "/")
		key := kv[0]
		val := kv[1]
		ht[key] = val
	}
	db.ht = ht
	// initialize bf
}

// Append takes serial db and appends values to one's own
func (db *DB) Append(serial string) {
	a := strings.Split(serial, ",")
	for _, v := range a {
		if v == "" {
			continue
		}
		kv := strings.Split(v, "/")
		key := kv[0]
		val := kv[1]
		db.ht[key] = val
	}
}

// AppendBF takes serial bf and appends values to one's own bf
func (db *DB) AppendBF(serial string) {
	newBF := bloom.NewBloom()
	newBF.DeSerial(serial)
	db.bf.Merge(newBF)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func mkdir(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			panic(err)
		}
	}
}

// hardFlush takes ht and writes serial to disk in ht.ryan file
func (db *DB) hardFlush(fileName string) {
	serial := db.ToString()
	db.writeHT(serial, fileName)
}

func (db *DB) writeHT(serial string, fileName string) {
	db.writeFile(serial, fileName)
}

// Flush takes ht and writes serial to disk in ht.ryan file
func (db *DB) Flush() {
	now := int(time.Now().UnixNano())
	name := fmt.Sprintf("ht_%d", now)
	db.hardFlush(name)
	db.ht = make(map[string]string)

	// db1 := NewDB(db.id)
	// db1.Load()
	// for k, v := range db.ht {
	// 	db1.simpleSet(k, v)
	// }
	// db1.applyDel()
	// db1.hardFlush("ht")
	// db.writeDelLog("")
}

// Squash merges all the string table files
func (db *DB) Squash() {
	db.Flush()

	now := int(time.Now().UnixNano())
	name := fmt.Sprintf("ht_%d", now)

	db1 := NewDB(db.id)
	db1.loadFull()
	fmt.Println("squash")
	db1.Print()
	db1.applyDel()
	db1.Print()
	db1.bf = bloom.NewBloom()
	// delete bloom.ryan, ht_.ryan, del_log.ryan
	files := db.diskStats()
	fmt.Println(files[0], now)
	db1.hardFlush(name)
	for _, fileName := range files {
		path := fmt.Sprintf("db/%d/%s", db.id, fileName)
		err := os.Remove(path)
		if err != nil {
			fmt.Println(err)
			continue
		}
	}
	db1.writeDelLog("")
	// fresh bloom filter
	for key := range db1.GetHT() {
		db1.bf.Add(key)
	}
	db1.writeFile(db1.bf.ToString(), "bloom")
}

// Load sets ht to data stored in ht.ryan
func (db *DB) Load() {
	fileNames := db.diskStats()
	index := len(fileNames) - 1
	if index < 0 {
		// no files yet
		return
	}
	latest := fileNames[index]
	db.loadFile(latest)
	bfSerial, ok := db.getBFLog()
	if ok {
		db.bf.DeSerial(bfSerial)
	}
}

func (db *DB) loadFile(fileName string) {
	path := fmt.Sprintf("db/%d/%s", db.id, fileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println("cannot find", path)
		return
	}
	data, err := ioutil.ReadFile(path)
	check(err)
	db.Append(string(data))
	fmt.Println("loading", string(data))
}

// loadFull tried to load all disk to mem
func (db *DB) loadFull() {
	files := db.diskStats()
	numFiles := len(files)
	// reverse
	for i := 0; i < numFiles/2; i++ {
		files[i], files[numFiles-i-1] = files[numFiles-i-1], files[i]
	}
	fmt.Println("files", files)
	for _, name := range files {
		db.loadFile(name)
	}
	fmt.Println("ht", db.ht)
}

func (db *DB) applyDel() {
	data, ok := db.getDelLog()
	if !ok {
		return
	}
	for _, key := range data {
		delete(db.ht, key)
	}
}

func (db *DB) getDelLog() ([]string, bool) {
	path := fmt.Sprintf("db/%d/del_log.ryan", db.id)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println("can't find del log", path)
		return []string{}, false
	}
	data, err := ioutil.ReadFile(path)
	check(err)
	a := string(data)
	b := strings.Split(a, "/")
	return b, true
}

// Print outputs ht
func (db *DB) Print() {
	fmt.Println(db.ht)
}

// NewDB singleton factory for db object
func NewDB(ids ...int) *DB {
	id := int(time.Now().UnixNano())
	if len(ids) > 0 {
		id = ids[0]
	}
	db := &DB{
		ht:            map[string]string{},
		id:            id,
		flushCounter:  100,
		flushDefault:  100,
		squashCounter: 20,
		squashDefault: 20,
		bf:            bloom.NewBloom(),
	}
	return db
}

func main() {
	db := NewDB(1)
	db.Load()
	db.Flush()
	db.Set("ryan", "jiang")
	db.Set("favourite", "coffee")
	db.Set("big", "cat")
	db.Flush()
	db.Print()
	fmt.Println(db.ToString())
	db2 := NewDB(2)
	db2.Load()
	db2.Flush()
	db2.DeSerial(db.ToString())
	db2.Remove("ryan")
	db2.Remove("ryan")
	db2.Set("ryan", "qiyu")
	db2.Print()
	db2.Flush()
	db2.Set("classic", "pizza")
	db2.Flush()
}
