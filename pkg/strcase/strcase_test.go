package strcase

import (
	"strings"
	"testing"
)

var tests = [][]string{
	{"", ""},
	{"Test", "test"},
	{"test", "test"},
	{"testCase", "test_case"},
	{"test_case", "test_case"},
	{"TestCase", "test_case"},
	{"Test_Case", "test_case"},
	{"ID", "id"},
	{"userID", "user_id"},
	{"UserID", "user_id"},
	{"ManyManyWords", "many_many_words"},
	{"manyManyWords", "many_many_words"},
	{"icinga2", "icinga2"},
	{"Icinga2Version", "icinga2_version"},
	{"k8sVersion", "k8s_version"},
	{"1234", "1234"},
	{"a1b2c3d4", "a1b2c3d4"},
	{"with1234digits", "with1234digits"},
	{"with1234Digits", "with1234_digits"},
	{"IPv4", "ipv4"},
	{"IPv4Address", "ipv4_address"},
	{"caféCrème", "café_crème"},
	{"0℃", "0℃"},
	{"~0", "~0"},
	{"icinga💯points", "icinga💯points"},
	{"😃🙃😀", "😃🙃😀"},
	{"こんにちは", "こんにちは"},
	{"\xff\xfe\xfd", "���"},
	{"\xff", "�"},
}

func TestSnake(t *testing.T) {
	for _, test := range tests {
		s, expected := test[0], test[1]
		actual := Snake(s)
		if actual != expected {
			t.Errorf("%q: %q != %q", s, actual, expected)
		}
	}
}

func TestScreamingSnake(t *testing.T) {
	for _, test := range tests {
		s, expected := test[0], strings.ToUpper(test[1])
		actual := ScreamingSnake(s)
		if actual != expected {
			t.Errorf("%q: %q != %q", s, actual, expected)
		}
	}
}
