package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// limit < len(first_performance_data) test
func TestTruncInvalidLimitPerfData(t *testing.T) {
	str, boolValue := TruncPerfData("a=1.00 c=10% d=1.0 e=15KB f=2 z=189s", 2)
	assert.Equal(t, "", str)
	assert.Equal(t, true, boolValue)
}

// limit = len(first_performance_data) test
func TestTruncSinglePerfDataLimit(t *testing.T) {
	str, boolValue := TruncPerfData("a=1.00 c=10% d=1.0 e=15KB f=2 z=189s", 6)
	assert.Equal(t, "a=1.00", str)
	assert.Equal(t, true, boolValue)
}

// simple performance data test
func TestTruncSimplePerfData(t *testing.T) {
	str, boolValue := TruncPerfData("a=1.00 c=10% d=1.0 e=15KB f=2 z=189s", 30)
	assert.Equal(t, "a=1.00 c=10% d=1.0 e=15KB f=2", str)
	assert.Equal(t, true, boolValue)
}

// quoted performance data test
func TestTruncQuotedPerfData(t *testing.T) {
	str, boolValue := TruncPerfData("a=1.00 c=10% d=1.0 e=15KB f=2 z=189s 'x y z'=123s 'md n o'=1.0 e=15KB f=2 z=189s", 58)
	assert.Equal(t, "a=1.00 c=10% d=1.0 e=15KB f=2 z=189s 'x y z'=123s", str)
	assert.Equal(t, true, boolValue)
}

// quoted and with utf-8 characters performance data test
func TestTruncUtf8PerfData(t *testing.T) {
	str, boolValue := TruncPerfData("世界=15 汉字=1.00s '汉字 漢 字'=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s '世 界 世'=123s", 92)
	assert.Equal(t, "世界=15 汉字=1.00s '汉字 漢 字'=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s", str)
	assert.Equal(t, true, boolValue)
}

// complex, quoted and with utf-8 characters performance data test
func TestTruncComplexPerfData(t *testing.T) {
	str, boolValue := TruncPerfData("世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s '= 汉字 漢 字 x y z kl $! ='=123s", 130)
	assert.Equal(t, "世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s", str)
	assert.Equal(t, true, boolValue)
}

// limit > len(performance_data) test
func TestTruncPerfDataBigLimit(t *testing.T) {
	str, boolValue := TruncPerfData("世界=15 汉字=1.00s '汉字 漢 字'=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s '世 界 世'=123s", 200)
	assert.Equal(t, "世界=15 汉字=1.00s '汉字 漢 字'=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s '世 界 世'=123s", str)
	assert.Equal(t, false, boolValue)
}

// complex, quoted and with utf-8 characters performance data test
func TestTruncComplexQuotedPerfData(t *testing.T) {
	str, boolValue := TruncPerfData("世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢=10 字=15 '=2% z=189s '=9 汉字 漢=b 字=10 x=11 y=12 z=13 kl $! ='=123s", 142)
	assert.Equal(t, "世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢=10 字=15 '=2% z=189s", str)
	assert.Equal(t, true, boolValue)
}

// Worst Case scenario: The last performance data in the truncated string is invalid is incorrect
// Here the performance data is like "'=10 c'=15 '=10 c'=15"
func TestTruncPerfDataWorstCase(t *testing.T) {
	str, boolValue := TruncPerfData("世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢=10 字=15 '=2% z=189s '=9 汉字 漢=b 字=10 x=11 y=12 z=13 kl $! '=123s '=9 汉字 漢=b 字=10 x=11 y=12 z=13 kl $! '=123s '汉 漢=10 字=15 '=2% '=汉 漢=10 字=15 '=2% '=汉 漢=10 字=15 '=2%", 281)
	assert.Equal(t, "世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢=10 字=15 '=2% z=189s '=9 汉字 漢=b 字=10 x=11 y=12 z=13 kl $! '=123s '=9 汉字 漢=b 字=10 x=11 y=12 z=13 kl $! '=123s '汉 漢=10 字=15 '=2% '=汉 漢=10 字=15 '=2% '=汉 漢=10 字=15", str)
	assert.Equal(t, true, boolValue)
}

// simple truncation text data test
func TestTruncSimpleText(t *testing.T) {
	str, boolValue := TruncText("da39a3ee5e6b4b0d3255bfef95601890afd80709", 30)
	assert.Equal(t, "da39a3ee5e6b4b0d3255bfef956018", str)
	assert.Equal(t, true, boolValue)
}

// complex truncation text data test
func TestTruncComplexText(t *testing.T) {
	str, boolValue := TruncText("世界=15 世Hola/~~%&/()汉字 漢字)!%&(12342542452世界", 37)
	assert.Equal(t, "世界=15 世Hola/~~%&/()汉字 漢", str)
	assert.Equal(t, true, boolValue)
}

// limit > len(text) test
func TestTruncTextBigLimit(t *testing.T) {
	str, boolValue := TruncText("世界=15 世Hola/~~%&/()汉字 漢字)!%&(12342542452世界", 100)
	assert.Equal(t, "世界=15 世Hola/~~%&/()汉字 漢字)!%&(12342542452世界", str)
	assert.Equal(t, false, boolValue)
}

func benchmarkTruncPerfData(str string, limit int, b *testing.B) {
	for i := 0; i < b.N; i++ {
		TruncPerfData(str, limit)
	}
}

func benchmarkTruncText(str string, limit int, b *testing.B) {
	for i := 0; i < b.N; i++ {
		TruncText(str, limit)
	}
}

func BenchmarkTruncInvalidLimitPerfData(b *testing.B) {
	benchmarkTruncPerfData("a=1.00 c=10% d=1.0 e=15KB f=2 z=189s", 2, b)
}

func BenchmarkTruncSinglePerfDataLimit(b *testing.B) {
	benchmarkTruncPerfData("a=1.00 c=10% d=1.0 e=15KB f=2 z=189s", 6, b)
}

func BenchmarkTruncSimplePerfData(b *testing.B) {
	benchmarkTruncPerfData("a=1.00 c=10% d=1.0 e=15KB f=2 z=189s", 65536, b)
}

func BenchmarkTruncQuotedPerfData(b *testing.B) {
	benchmarkTruncPerfData("a=1.00 c=10% d=1.0 e=15KB f=2 z=189s 'x y z'=123s 'md n o'=1.0 e=15KB f=2 z=189s", 58, b)
}

func BenchmarkTruncUtf8PerfData(b *testing.B) {
	benchmarkTruncPerfData("世界=15 汉字=1.00s '汉字 漢 字'=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s '世 界 世'=123s", 80, b)
}

func BenchmarkTruncComplexPerfData(b *testing.B) {
	benchmarkTruncPerfData("世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s '= 汉字 漢 字 x y z kl $! ='=123s", 130, b)
}

func BenchmarkTruncComplexQuotedPerfData(b *testing.B) {
	benchmarkTruncPerfData("世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢=10 字=15'=2% z=189s '= 汉字 漢=b 字=10 x=11 y=12 z=13 kl $! ='=123s", 142, b)
}

func BenchmarkTruncPerfDataBigLimit(b *testing.B) {
	benchmarkTruncPerfData("世界=15 汉字=1.00s '汉字 漢 字'=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s '世 界 世'=123s", 200, b)
}

func BenchmarkTruncPerfDataWorstCase(b *testing.B) {
	benchmarkTruncPerfData("世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢=10 字=15 '=2% z=189s '=9 汉字 漢=b 字=10 x=11 y=12 z=13 kl $! '=123s '=9 汉字 漢=b 字=10 x=11 y=12 z=13 kl $! '=123s '汉 漢=10 字=15 '=2% '=汉 漢=10 字=15 '=2% '=汉 漢=10 字=15 '=2%", 281, b)
}

func BenchmarkTruncSimpleText(b *testing.B) {
	benchmarkTruncText("da39a3ee5e6b4b0d3255bfef95601890afd80709", 30, b)
}

func BenchmarkTruncComplexText(b *testing.B) {
	benchmarkTruncText("世界=15 世Hola/~~%&/()汉字 漢字)!%&(12342542452世界", 37, b)
}

func BenchmarkTruncTextBigLimit(b *testing.B) {
	benchmarkTruncText("世界=15 世Hola/~~%&/()汉字 漢字)!%&(12342542452世界", 100, b)
}