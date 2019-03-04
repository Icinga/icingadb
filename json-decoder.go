package icingadb_json_decoder

import (
	"encoding/json"
)

// Number of workers DecodePool uses
var poolSize = 16

type JsonDecodePackage struct{
	// Json strings from Redis
	ChecksumsRaw string
	ConfigRaw string
	// When unmarshaled, results will be written here
	ChecksumsProcessed map[string]interface{}
	ConfigProcessed map[string]interface{}
	// Package will be sent back through this channel
	ChBack *chan JsonDecodePackage
}

// decodeString unmarshals the string toDecode using the json package. Returns the object as a
// map[string]interface and nil if successful, error if not.
func decodeString(toDecode string) (map[string]interface{}, error) {
	var unJson interface{} = nil
	if err := json.Unmarshal([]byte(toDecode), &unJson); err != nil {
		return nil, err
	}

	return unJson.(map[string]interface{}), nil
}

// decodePool takes a channel it receives JsonDecodePackages from. These packages are decoded
// by a pool of workers which send their result back through their own channel. Returns error
// if any.
func DecodePool(chInput <-chan JsonDecodePackage) error {
	consumers := make([]chan JsonDecodePackage, poolSize)
	for i := range consumers {
		consumers[i] = make(chan JsonDecodePackage)
		go decodePackage(consumers[i])
	}

	distribution := func(ch <-chan JsonDecodePackage, consumers []chan JsonDecodePackage) {
		defer func(providers []chan JsonDecodePackage) {
			for _, channel := range providers {
				close(channel)
			}
		}(consumers)

		for {
			for _, consumer := range consumers {
				select {
				case packag := <- ch:
					consumer <- packag
				}
			}
		}
	}

	go distribution(chInput, consumers)

	return nil
}

// decodePackage is the worker function for DecodePool. Reads from a channel and sends back decoded
// packages. Returns error if any.
func decodePackage(chInput <-chan JsonDecodePackage) error {
	var err error


	for input := range chInput{
		if input.ChecksumsProcessed == nil {
			if input.ChecksumsProcessed, err = decodeString(input.ChecksumsRaw); err != nil {
				return err
			}
		}
		if input.ConfigProcessed == nil {
			if input.ConfigProcessed, err = decodeString(input.ConfigRaw); err != nil {
				return err
			}
		}

		*input.ChBack <- JsonDecodePackage(input)

	}

	return nil
}