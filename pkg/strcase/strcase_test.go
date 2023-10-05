package strcase

import "testing"

func TestSnake(t *testing.T) { snake(t) }

func BenchmarkSnake(b *testing.B) {
	for n := 0; n < b.N; n++ {
		snake(b)
	}
}

func TestScreamingSnake(t *testing.T) { screamingSnake(t) }

func BenchmarkScreamingSnake(b *testing.B) {
	for n := 0; n < b.N; n++ {
		screamingSnake(b)
	}
}

func snake(tb testing.TB) {
	cases := [][]string{
		{"", ""},
		{"AnyKind of_string", "any_kind_of_string"},
		{" Test Case ", "test_case"},
		{" Test Case", "test_case"},
		{"testCase", "test_case"},
		{"test_case", "test_case"},
		{"Test Case ", "test_case"},
		{"Test Case", "test_case"},
		{"TestCase", "test_case"},
		{"Test", "test"},
		{"test", "test"},
		{"ID", "id"},
		{"ManyManyWords", "many_many_words"},
		{"manyManyWords", "many_many_words"},
		{" some string", "some_string"},
		{"some string", "some_string"},
		{"userID", "user_id"},
		{"icinga2", "icinga_2"},
	}
	for _, c := range cases {
		s, expected := c[0], c[1]
		actual := Snake(s)
		if actual != expected {
			tb.Errorf("%q: %q != %q", s, actual, expected)
		}
	}
}

func screamingSnake(tb testing.TB) {
	cases := [][]string{
		{"", ""},
		{"AnyKind of_string", "ANY_KIND_OF_STRING"},
		{" Test Case ", "TEST_CASE"},
		{" Test Case", "TEST_CASE"},
		{"testCase", "TEST_CASE"},
		{"test_case", "TEST_CASE"},
		{"Test Case ", "TEST_CASE"},
		{"Test Case", "TEST_CASE"},
		{"TestCase", "TEST_CASE"},
		{"Test", "TEST"},
		{"test", "TEST"},
		{"ID", "ID"},
		{"ManyManyWords", "MANY_MANY_WORDS"},
		{"manyManyWords", "MANY_MANY_WORDS"},
		{" some string", "SOME_STRING"},
		{"some string", "SOME_STRING"},
		{"userID", "USER_ID"},
		{"icinga2", "ICINGA_2"},
	}
	for _, c := range cases {
		s, expected := c[0], c[1]
		actual := ScreamingSnake(s)
		if actual != expected {
			tb.Errorf("%q: %q != %q", s, actual, expected)
		}
	}
}
