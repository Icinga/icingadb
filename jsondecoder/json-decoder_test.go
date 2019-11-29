// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package jsondecoder

import (
	"github.com/Icinga/icingadb/configobject/objecttypes/host"
	"github.com/Icinga/icingadb/connection"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_decodeString(t *testing.T) {
	/*var testCorrect = "{\"Integer\": 2.0, \"String\": \"Test One Two Three\"}"
	var testBroken = "{ahahahahaha}"

	err := decodeString(testCorrect)
	assert.NoError(t, err)
	assert.Equal(t, 2.0, ret["Integer"])
	assert.Equal(t, "Test One Two Three", ret["String"])

	_, err = decodeString(testBroken)
	assert.Error(t, err)*/
}

func Test_DecodePool(t *testing.T) {
	var chInput = make(chan *JsonDecodePackages)
	var chOutput = make(chan []connection.Row)
	var chError = make(chan error)

	var TestPackageA = JsonDecodePackage{
		"3a18e07f776af383cd7355b89eefd1fdf0fd47a9",
		"{\"checkcommand_id\":\"0bba6ab6747f1c0de3bf80932d10bc7b603e27fc\",\"environment_id\":\"90a8834de76326869f3e703cd61513081ad73d3c\",\"group_ids\":[],\"name_checksum\":\"21021feda571e19d8afb53fb11ca089db7578cee\",\"properties_checksum\":\"8d515de29444d9df3e374bcc1b890040fcc48be5\"}",
		"{\"active_checks_enabled\":true,\"address\":\"\",\"address6\":\"\",\"check_interval\":60.0,\"check_retry_interval\":60.0,\"check_timeout\":null,\"checkcommand\":\"random\",\"display_name\":\"aa3derphosta469\",\"event_handler_enabled\":true,\"flapping_enabled\":false,\"flapping_threshold_high\":30.0,\"flapping_threshold_low\":25.0,\"icon_image_alt\":\"\",\"is_volatile\":false,\"max_check_attempts\":3.0,\"name\":\"aa3derphosta469\",\"notes\":\"\",\"notifications_enabled\":true,\"passive_checks_enabled\":true,\"perfdata_enabled\":true}",
		nil,
		host.ObjectInformation.Factory,
		"host",
	}

	var TestPackageB = JsonDecodePackage{
		"7dd90e9833243afef861f257c78ddee941edfa2f",
		"{\"checkcommand_id\":\"0bba6ab6747f1c0de3bf80932d10bc7b603e27fc\",\"environment_id\":\"90a8834de76326869f3e703cd61513081ad73d3c\",\"group_ids\":[],\"name_checksum\":\"f14feab0710d05e4ca9ffd712f8c0af5d8f5119a\",\"properties_checksum\":\"bd3e124054427700571dae7552aa90e3fe4f1fde\"}",
		"{\"active_checks_enabled\":true,\"address\":\"\",\"address6\":\"\",\"check_interval\":60.0,\"check_retry_interval\":60.0,\"check_timeout\":null,\"checkcommand\":\"random\",\"display_name\":\"aa3derphosta378\",\"event_handler_enabled\":true,\"flapping_enabled\":false,\"flapping_threshold_high\":30.0,\"flapping_threshold_low\":25.0,\"icon_image_alt\":\"\",\"is_volatile\":false,\"max_check_attempts\":3.0,\"name\":\"aa3derphosta378\",\"notes\":\"\",\"notifications_enabled\":true,\"passive_checks_enabled\":true,\"perfdata_enabled\":true}",
		nil,
		host.ObjectInformation.Factory,
		"host",
	}

	DecodePool(chInput, chError, 2)

	pkgs := JsonDecodePackages{
		ChBack: chOutput,
	}

	pkgs.Packages = append(pkgs.Packages, TestPackageA)
	pkgs.Packages = append(pkgs.Packages, TestPackageB)

	chInput <- &pkgs

	result := <-chOutput

	assert.NotNil(t, result)
	assert.Equal(t, 2, len(result))

	close(chInput)
	close(chOutput)
}
