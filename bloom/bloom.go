package bloom

import (
	"fmt"
	"hash"
	"hash/fnv"
	"strconv"
	"strings"

	"github.com/spaolacci/murmur3"
)

// BloomFilter is a struct for a probablitic  answer to "has?"
type BloomFilter struct {
	bitArr  []bool
	num     int
	hashFun []hash.Hash64
}

// NewBloom singleton factor for bf
func NewBloom(s ...int) *BloomFilter {
	size := 1000
	if len(s) > 0 {
		size = s[0]
	}
	bf := &BloomFilter{
		bitArr:  make([]bool, size),
		num:     0,
		hashFun: []hash.Hash64{murmur3.New64(), fnv.New64(), fnv.New64a()},
	}
	return bf
}

// GetNum returns num of elements in bf
func (bf *BloomFilter) GetNum() int {
	return bf.num
}

// GetBits returns bit arr
func (bf *BloomFilter) GetBits() []bool {
	return bf.bitArr
}

// Merge merges new bf to our own
func (bf *BloomFilter) Merge(bf2 *BloomFilter) {
	for i, bit := range bf2.bitArr {
		if bit && !bf.bitArr[i] {
			bf.bitArr[i] = true
		}
	}
}

// Add adds element to bf
func (bf *BloomFilter) Add(e string) {
	hashes := bf.doHash(e)
	size := len(bf.bitArr)
	for _, hash := range hashes {
		index := hash % size
		//fmt.Println("size", size, "index", index, "hash", hash)
		bf.bitArr[index] = true
	}
	bf.num++
}

// HasNot checks if element is not in bf
func (bf *BloomFilter) HasNot(e string) bool {
	hashes := bf.doHash(e)
	size := len(bf.bitArr)
	for _, hash := range hashes {
		index := hash % size
		if !bf.bitArr[index] {
			return true
		}
	}
	return false
}

func (bf *BloomFilter) doHash(e string) []int {
	output := []int{}
	for _, fun := range bf.hashFun {
		fun.Write([]byte(e))
		h := int(fun.Sum64())
		if h < 0 {
			h *= -1
		}
		fun.Reset()
		output = append(output, h)
	}
	return output
}

// ToString returns serial string of bf
func (bf *BloomFilter) ToString() string {
	bitSet := ""
	for _, bit := range bf.bitArr {
		if bit {
			bitSet += "1"
		} else {
			bitSet += "0"
		}
	}
	serial := fmt.Sprintf("%d/%s/%d", len(bf.bitArr), bitSet, bf.num)
	return serial
}

// DeSerial takes string and updates bf
func (bf *BloomFilter) DeSerial(serial string) {
	a := strings.Split(serial, "/")
	arrSize, err0 := strconv.Atoi(a[0])
	bitArr := make([]bool, arrSize)
	for i, bit := range a[1] {
		if bit == '0' {
			bitArr[i] = false
		} else if bit == '1' {
			bitArr[i] = true
		}
	}
	num, err2 := strconv.Atoi(a[2])
	if err0 != nil || err2 != nil {
		fmt.Println("bad conversion", a)
		return
	}
	bf.bitArr = bitArr
	bf.num = num
}

func main() {
	bf := NewBloom(10)
	bf.Add("ryan")
	bf.Add("jiang")
	bf.Add("cool")

	fmt.Println("doesn't have ryan:", bf.HasNot("ryan"))
	fmt.Println("doesn't have jiang:", bf.HasNot("jiang"))
	fmt.Println("doesn't have cool:", bf.HasNot("cool"))
	fmt.Println("doesn't have grapes:", bf.HasNot("grapes"))
	fmt.Println("doesn't have rzan:", bf.HasNot("rzan"))
	fmt.Println("we have", bf.GetNum(), "elements")
	bf2 := NewBloom()
	bf2.DeSerial(bf.ToString())
	fmt.Println(bf.bitArr)
	fmt.Println(bf.ToString())
	fmt.Println(bf2.bitArr)
	fmt.Println("2doesn't have ryan:", bf2.HasNot("ryan"))
	fmt.Println("2doesn't have jiang:", bf2.HasNot("jiang"))
	fmt.Println("2doesn't have cool:", bf2.HasNot("cool"))
	fmt.Println("2doesn't have grapes:", bf2.HasNot("grapes"))
	fmt.Println("2doesn't have rzan:", bf2.HasNot("rzan"))
	fmt.Println("2we have", bf2.GetNum(), "elements")
}
