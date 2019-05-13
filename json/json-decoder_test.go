package icingadb_json_decoder

import (
	"github.com/stretchr/testify/assert"
	"testing"
)



func Test_decodeString(t *testing.T) {
	var testCorrect = "{\"Integer\": 2.0, \"String\": \"Test One Two Three\"}"
	var testBroken = "{ahahahahaha}"

	ret, err := decodeString(testCorrect)
	assert.NoError(t, err)
	assert.Equal(t, 2.0, ret["Integer"])
	assert.Equal(t, "Test One Two Three", ret["String"])

	_, err = decodeString(testBroken)
	assert.Error(t, err)
}

func Test_DecodePool(t *testing.T) {
	var chInput = make(chan JsonDecodePackage)
	var chOutput = make(chan JsonDecodePackage)
	var chError = make(chan error)

	var TestPackageA = JsonDecodePackage{
		"{\"action_url_id\":\"761ff24e252d57581a7de5d9f417f717fb3c2d7f\",\"checkcommand_id\":\"f5e3b3b22741f40c74326fbcc79d9c331d8fa4ee\",\"customvars_checksum\":\"e9fea9581588f18cfb46969268a94166bd0474ae\",\"environment_id\":\"90a8834de76326869f3e703cd61513081ad73d3c\",\"group_ids\":[\"a63234de9f608c4a4f86053870d79610ec58b258\"],\"groups_checksum\":\"9878a753d010eb1bbde57bb78727a6e6ba26aa51\",\"host_id\":\"330c09556cbb5e01c180343bb669a2d36b48dd2c\",\"name_checksum\":\"9f75a6ea3ea6f1692538c865133a8a08e48f06d5\",\"notes_url_id\":\"31bb5f9a69c659270e2bcd257b77353669c04d1e\",\"properties_checksum\":\"caff92fafc9a17097304f6c2fb9fa029d3ec8aa8\",\"zone_id\":\"407eaa141abcae8ee554e4fe4b9e9b726bac4b77\"}",
		"{\"active_checks_enabled\":false,\"check_interval\":300.0,\"check_retry_interval\":60.0,\"check_timeout\":null,\"checkcommand\":\"dummy\",\"display_name\":\"TestService A - 0.0\",\"event_handler_enabled\":true,\"flapping_enabled\":false,\"flapping_threshold_high\":30.0,\"flapping_threshold_low\":25.0,\"icon_image_alt\":\"\",\"is_volatile\":false,\"max_check_attempts\":3.0,\"name\":\"TestService A - 0.0\",\"notes\":\"\",\"notifications_enabled\":true,\"passive_checks_enabled\":true,\"perfdata_enabled\":true,\"zone\":\"double\"}",
		nil,
		nil,
		&chOutput,
	}

	var TestPackageB = JsonDecodePackage{
		"{\"checkcommand_id\":\"f5e3b3b22741f40c74326fbcc79d9c331d8fa4ee\",\"customvars_checksum\":\"efb9e8a4dff9ee330838909403655ae376251dc9\",\"environment_id\":\"90a8834de76326869f3e703cd61513081ad73d3c\",\"group_ids\":[\"a63234de9f608c4a4f86053870d79610ec58b258\"],\"groups_checksum\":\"9878a753d010eb1bbde57bb78727a6e6ba26aa51\",\"host_id\":\"7bb83f280fee68146e223b51c02c9ac1e5d56305\",\"name_checksum\":\"92420fe84a880f5b7675ba0fb0f4f730f40a144a\",\"properties_checksum\":\"8563b9113161953acabb7bba779cc5706494eb3b\",\"zone_id\":\"407eaa141abcae8ee554e4fe4b9e9b726bac4b77\"}",
		"{\"active_checks_enabled\":false,\"check_interval\":300.0,\"check_retry_interval\":60.0,\"check_timeout\":null,\"checkcommand\":\"dummy\",\"display_name\":\"TestService B - 0.0\",\"event_handler_enabled\":true,\"flapping_enabled\":false,\"flapping_threshold_high\":30.0,\"flapping_threshold_low\":25.0,\"icon_image_alt\":\"\",\"is_volatile\":false,\"max_check_attempts\":3.0,\"name\":\"TestService B - 0.0\",\"notes\":\"\",\"notifications_enabled\":true,\"passive_checks_enabled\":true,\"perfdata_enabled\":true,\"zone\":\"double\"}",
		nil,
		nil,
		&chOutput,
	}

	DecodePool(chInput, chError, 4)


	chInput <- TestPackageA
	chInput <- TestPackageB
	close(chInput)

	resultA := <-chOutput
	resultB := <-chOutput
	close(chOutput)

	assert.NotNil(t, resultA.ConfigProcessed)
	assert.NotNil(t, resultB.ConfigProcessed)
}

func Test_decodePackage(t *testing.T) {
	var chInput = make(chan JsonDecodePackage)
	var chOutput = make(chan JsonDecodePackage)

	var TestPackageA = JsonDecodePackage{
		"{\"action_url_id\":\"761ff24e252d57581a7de5d9f417f717fb3c2d7f\",\"checkcommand_id\":\"f5e3b3b22741f40c74326fbcc79d9c331d8fa4ee\",\"customvars_checksum\":\"e9fea9581588f18cfb46969268a94166bd0474ae\",\"environment_id\":\"90a8834de76326869f3e703cd61513081ad73d3c\",\"group_ids\":[\"a63234de9f608c4a4f86053870d79610ec58b258\"],\"groups_checksum\":\"9878a753d010eb1bbde57bb78727a6e6ba26aa51\",\"host_id\":\"330c09556cbb5e01c180343bb669a2d36b48dd2c\",\"name_checksum\":\"9f75a6ea3ea6f1692538c865133a8a08e48f06d5\",\"notes_url_id\":\"31bb5f9a69c659270e2bcd257b77353669c04d1e\",\"properties_checksum\":\"caff92fafc9a17097304f6c2fb9fa029d3ec8aa8\",\"zone_id\":\"407eaa141abcae8ee554e4fe4b9e9b726bac4b77\"}",
		"{\"active_checks_enabled\":false,\"check_interval\":300.0,\"check_retry_interval\":60.0,\"check_timeout\":null,\"checkcommand\":\"dummy\",\"display_name\":\"TestService A - 0.0\",\"event_handler_enabled\":true,\"flapping_enabled\":false,\"flapping_threshold_high\":30.0,\"flapping_threshold_low\":25.0,\"icon_image_alt\":\"\",\"is_volatile\":false,\"max_check_attempts\":3.0,\"name\":\"TestService A - 0.0\",\"notes\":\"\",\"notifications_enabled\":true,\"passive_checks_enabled\":true,\"perfdata_enabled\":true,\"zone\":\"double\"}",
		nil,
		nil,
		&chOutput,
	}

	var TestPackageB = JsonDecodePackage{
		"{\"checkcommand_id\":\"f5e3b3b22741f40c74326fbcc79d9c331d8fa4ee\",\"customvars_checksum\":\"efb9e8a4dff9ee330838909403655ae376251dc9\",\"environment_id\":\"90a8834de76326869f3e703cd61513081ad73d3c\",\"group_ids\":[\"a63234de9f608c4a4f86053870d79610ec58b258\"],\"groups_checksum\":\"9878a753d010eb1bbde57bb78727a6e6ba26aa51\",\"host_id\":\"7bb83f280fee68146e223b51c02c9ac1e5d56305\",\"name_checksum\":\"92420fe84a880f5b7675ba0fb0f4f730f40a144a\",\"properties_checksum\":\"8563b9113161953acabb7bba779cc5706494eb3b\",\"zone_id\":\"407eaa141abcae8ee554e4fe4b9e9b726bac4b77\"}",
		"{\"active_checks_enabled\":false,\"check_interval\":300.0,\"check_retry_interval\":60.0,\"check_timeout\":null,\"checkcommand\":\"dummy\",\"display_name\":\"TestService B - 0.0\",\"event_handler_enabled\":true,\"flapping_enabled\":false,\"flapping_threshold_high\":30.0,\"flapping_threshold_low\":25.0,\"icon_image_alt\":\"\",\"is_volatile\":false,\"max_check_attempts\":3.0,\"name\":\"TestService B - 0.0\",\"notes\":\"\",\"notifications_enabled\":true,\"passive_checks_enabled\":true,\"perfdata_enabled\":true,\"zone\":\"double\"}",
		nil,
		nil,
		&chOutput,
	}

	go func() {
		err := decodePackage(chInput)
		assert.NoError(t, err)
	}()

	go func() {
		resultA := <-chOutput
		resultB := <-chOutput

		assert.NotNil(t, resultA.ConfigProcessed)
		assert.NotNil(t, resultB.ConfigProcessed)
	}()

	chInput <- TestPackageA
	chInput <- TestPackageB
	close(chInput)
	close(chOutput)
}