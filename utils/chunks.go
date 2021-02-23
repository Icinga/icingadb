// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package utils

import (
	"math"
	"reflect"
)

func ChunkKeys(done <-chan struct{}, keys []string, size int) <-chan []string {
	ch := make(chan []string)

	go func() {
		defer close(ch)
		for i := 0; i < len(keys); i += size {
			end := i + size
			if end > len(keys) {
				end = len(keys)
			}
			select {
			case ch <- keys[i:end]:
			case <-done:
				return
			}
		}
	}()

	return ch
}

func ChunkInterfaces(interfaces []interface{}, size int) [][]interface{} {
	chunksLen := int(math.Ceil(float64(len(interfaces)) / float64(size)))
	chunks := make([][]interface{}, chunksLen)

	for i := 0; i < chunksLen; i++ {
		start := i * size
		end := start + size
		if end > len(interfaces) {
			end = len(interfaces)
		}

		chunks[i] = interfaces[start:end]
	}

	return chunks
}

type Chunk struct {
	Begin, End int
}

func ChunkIndices(total, chunk int) []Chunk {
	n := total / chunk
	if total%chunk > 0 {
		n++
	}

	chunks := make([]Chunk, n)
	for i := range chunks {
		chunks[i].Begin = i * chunk
		chunks[i].End = i*chunk + chunk
		if chunks[i].End > total {
			chunks[i].End = total
		}
	}

	return chunks
}

func SafeSlice(slicePtr interface{}, start, stop int) {
	vSlicePtr := reflect.ValueOf(slicePtr)
	if vSlicePtr.Kind() != reflect.Ptr {
		panic("slicePtr must be a pointer")
	}

	vSlice := vSlicePtr.Elem()
	if vSlice.Kind() != reflect.Slice {
		panic("slicePtr must point to a slice")
	}

	l := vSlice.Len()
	if start > l {
		start = l
	}

	if stop > l {
		stop = l
	}

	vSlice.Set(vSlice.Slice(start, stop))
}
