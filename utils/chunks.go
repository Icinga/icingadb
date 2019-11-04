// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package utils

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
