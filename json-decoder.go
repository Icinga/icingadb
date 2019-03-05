package icingadb_json_decoder

import (
	"github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type JsonDecodePackage struct{
	// Json strings from Redis
	ChecksumsRaw string
	ConfigRaw string
	// When unmarshaled, results will be written here
	ChecksumsProcessed map[string]interface{}
	ConfigProcessed map[string]interface{}
	// Package will be sent back through this channel
	ChBack *chan *JsonDecodePackage
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

// decodePool takes a channel it receives JsonDecodePackages from and an error channel to forward errors.
// These packages are decoded by a pool of pollSize workers which send their result back through their own channel.
func DecodePool(chInput <-chan *JsonDecodePackage, chError chan error, poolSize int) {
	for i := 0; i < poolSize; i++ {
		go func(in <-chan *JsonDecodePackage, chErrorInternal chan error) {
			chErrorInternal <- decodePackage(in)
		}(chInput, chError)
	}
}

// decodePackage is the worker function for DecodePool. Reads from a channel and sends back decoded
// packages. Returns error if any.
func decodePackage(chInput <-chan *JsonDecodePackage) error {
	var err error

	for input := range chInput{
		if input.ChecksumsProcessed == nil && input.ChecksumsRaw != "" {
			if input.ChecksumsProcessed, err = decodeString(input.ChecksumsRaw); err != nil {
				return err
			}
		}
		if input.ConfigProcessed == nil && input.ConfigRaw != ""{
			if input.ConfigProcessed, err = decodeString(input.ConfigRaw); err != nil {
				return err
			}
		}

		*input.ChBack <- input

	}

	return nil
}