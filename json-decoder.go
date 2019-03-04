package icingadb_json_decoder

import (
	"encoding/json"
)

var poolSize = 16

type JsonDecodePackage struct{
	ChecksumsRaw string
	ConfigRaw string
	ChecksumsProcessed map[string]interface{}
	ConfigProcessed map[string]interface{}
	ChBack *chan JsonDecodePackage
}

func decodeString(toDecode string) (map[string]interface{}, error) {
	var unJson interface{} = nil
	if err := json.Unmarshal([]byte(toDecode), &unJson); err != nil {
		return nil, err
	}

	return unJson.(map[string]interface{}), nil
}

func decodePool(chInput <-chan JsonDecodePackage) error {
	consumers := make([]chan JsonDecodePackage, poolSize)
	for i := range consumers {
		consumers[i] = make(chan JsonDecodePackage)
		go DecodePackage(consumers[i])
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

func DecodePackage(chInput <-chan JsonDecodePackage) error {
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