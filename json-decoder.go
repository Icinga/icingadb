package icingadb_json_decoder

import (
	"github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type Row interface {
	InsertValues() []interface{}
	UpdateValues() []interface{}
	GetId() string
	SetId(id string)
}

type RowFactory func() Row


type JsonDecodePackage struct{
	Id [20]byte
	// Json strings from Redis
	ChecksumsRaw string
	ConfigRaw string
	Row Row
	// Package will be sent back through this channel
	ChBack chan *JsonDecodePackage
	Factory RowFactory
}

// decodeString unmarshals the string toDecode using the json package. Returns the object as a
// map[string]interface and nil if successful, error if not.
func decodeString(toDecode string, row Row) error {
	return json.Unmarshal([]byte(toDecode), row)
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
		row := input.Factory()
		if input.ChecksumsRaw != "" {
			if err := decodeString(input.ChecksumsRaw, row); err != nil {
				return err
			}
		}
		if input.ConfigRaw != ""{
			if err = decodeString(input.ConfigRaw, row); err != nil {
				return err
			}
		}

		input.Row = row
		input.ChBack <- input
	}

	return nil
}