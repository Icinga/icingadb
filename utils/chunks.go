// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package utils

import (
	"math"
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
		start := i * size;
		end := start + size
		if end > len(interfaces) {
			end = len(interfaces)
		}

		chunks[i] = interfaces[start:end]
	}

	return chunks
}
