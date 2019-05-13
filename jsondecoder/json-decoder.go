package jsondecoder

import (
	"git.icinga.com/icingadb/icingadb-main/configobject"
	"github.com/json-iterator/go"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type JsonDecodePackage struct {
	// Id of the config object
	Id string
	// Json strings from Redis
	ChecksumsRaw string
	// Json strings from Redis
	ConfigRaw string
	// Unmarshaled config object ready to be used in SQL
	Row configobject.Row
	// Package will be sent back through this channel
	Factory configobject.RowFactory
	// Object type (host, service, endpoint, command...)
	ObjectType string
}

type JsonDecodePackages struct {
	Packages []JsonDecodePackage
	ChBack chan<- []configobject.Row
}

// decodeString unmarshals the string toDecode using the json package. Returns the object as a
// map[string]interface and nil if successful, error if not.
func decodeString(toDecode string, row configobject.Row) error {
	return json.Unmarshal([]byte(toDecode), row)
}

// decodePool takes a channel it receives JsonDecodePackages from and an error channel to forward errors.
// These packages are decoded by a pool of pollSize workers which send their result back through their own channel.
func DecodePool(chInput <-chan *JsonDecodePackages, chError chan error, poolSize int) {
	for i := 0; i < poolSize; i++ {
		go func(in <-chan *JsonDecodePackages, chErrorInternal chan error) {
			chErrorInternal <- decodePackage(in)
		}(chInput, chError)
	}
}

// decodePackage is the worker function for DecodePool. Reads from a channel and sends back decoded
// packages. Returns error if any.
func decodePackage(chInput <-chan *JsonDecodePackages) error {
	var err error

	for pkgs := range chInput{
		var rows []configobject.Row
		for _, pkg := range pkgs.Packages{
			row := pkg.Factory()
			row.SetId(pkg.Id)
			if pkg.ChecksumsRaw != "" {
				if err := decodeString(pkg.ChecksumsRaw, row); err != nil {
					return err
				}
			}
			if pkg.ConfigRaw != ""{
				if err = decodeString(pkg.ConfigRaw, row); err != nil {
					return err
				}
			}

			rows = append(rows, row)
		}

		pkgs.ChBack <- rows
	}

	return nil
}