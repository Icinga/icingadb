package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// limit < len(first_performance_data) test
func TestTruncateInvalidLimitPerfData(t *testing.T) {
	str, truncated := TruncatePerfData("a=1.00 c=10% d=1.0 e=15KB f=2 z=189s", 2)
	assert.Equal(t, "", str)
	assert.Equal(t, true, truncated)
}

// limit = len(first_performance_data) test
func TestTruncateSinglePerfDataLimit(t *testing.T) {
	str, truncated := TruncatePerfData("a=1.00 c=10% d=1.0 e=15KB f=2 z=189s", 6)
	assert.Equal(t, "a=1.00", str)
	assert.Equal(t, true, truncated)
}

// simple performance data test
func TestTruncateSimplePerfData(t *testing.T) {
	str, truncated := TruncatePerfData("a=1.00 c=10% d=1.0 e=15KB f=2 z=189s", 30)
	assert.Equal(t, "a=1.00 c=10% d=1.0 e=15KB f=2", str)
	assert.Equal(t, true, truncated)
}

// quoted performance data test
func TestTruncateQuotedPerfData(t *testing.T) {
	str, truncated := TruncatePerfData("a=1.00 c=10% d=1.0 e=15KB f=2 z=189s 'x y z'=123s 'md n o'=1.0 e=15KB f=2 z=189s", 58)
	assert.Equal(t, "a=1.00 c=10% d=1.0 e=15KB f=2 z=189s 'x y z'=123s", str)
	assert.Equal(t, true, truncated)
}

// quoted and with utf-8 characters performance data test
func TestTruncateUtf8PerfData(t *testing.T) {
	str, truncated := TruncatePerfData("世界=15 汉字=1.00s '汉字 漢 字'=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s '世 界 世'=123s", 92)
	assert.Equal(t, "世界=15 汉字=1.00s '汉字 漢 字'=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s", str)
	assert.Equal(t, true, truncated)
}

// complex, quoted and with utf-8 characters performance data test
func TestTruncateComplexPerfData(t *testing.T) {
	str, truncated := TruncatePerfData("世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s '= 汉字 漢 字 x y z kl $! ='=123s", 130)
	assert.Equal(t, "世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s", str)
	assert.Equal(t, true, truncated)
}

// limit > len(performance_data) test
func TestTruncatePerfDataBigLimit(t *testing.T) {
	str, truncated := TruncatePerfData("世界=15 汉字=1.00s '汉字 漢 字'=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s '世 界 世'=123s", 200)
	assert.Equal(t, "世界=15 汉字=1.00s '汉字 漢 字'=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s '世 界 世'=123s", str)
	assert.Equal(t, false, truncated)
}

// complex, quoted and with utf-8 characters performance data test
func TestTruncateComplexQuotedPerfData(t *testing.T) {
	str, truncated := TruncatePerfData("世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢=10 字=15 '=2% z=189s '=9 汉字 漢=b 字=10 x=11 y=12 z=13 kl $! ='=123s", 142)
	assert.Equal(t, "世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢=10 字=15 '=2% z=189s", str)
	assert.Equal(t, true, truncated)
}

// Worst Case scenario: The last performance data in the truncated string is invalid is incorrect
// Here the performance data is like "'=10 c'=15 '=10 c'=15"
func TestTruncatePerfDataWorstCase(t *testing.T) {
	str, truncated := TruncatePerfData("世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢=10 字=15 '=2% z=189s '=9 汉字 漢=b 字=10 x=11 y=12 z=13 kl $! '=123s '=9 汉字 漢=b 字=10 x=11 y=12 z=13 kl $! '=123s '汉 漢=10 字=15 '=2% '=汉 漢=10 字=15 '=2% '=汉 漢=10 字=15 '=2%", 281)
	assert.Equal(t, "世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢=10 字=15 '=2% z=189s '=9 汉字 漢=b 字=10 x=11 y=12 z=13 kl $! '=123s '=9 汉字 漢=b 字=10 x=11 y=12 z=13 kl $! '=123s '汉 漢=10 字=15 '=2% '=汉 漢=10 字=15 '=2% '=汉 漢=10 字=15", str)
	assert.Equal(t, true, truncated)
}

// simple Truncateation text data test
func TestTruncateSimpleText(t *testing.T) {
	str, truncated := TruncateText("da39a3ee5e6b4b0d3255bfef95601890afd80709", 30)
	assert.Equal(t, "da39a3ee5e6b4b0d3255bfef956018", str)
	assert.Equal(t, true, truncated)
}

// complex Truncateation text data test
func TestTruncateComplexText(t *testing.T) {
	str, truncated := TruncateText("世界=15 世Hola/~~%&/()汉字 漢字)!%&(12342542452世界", 37)
	assert.Equal(t, "世界=15 世Hola/~~%&/()汉字 漢", str)
	assert.Equal(t, true, truncated)
}

// limit > len(text) test
func TestTruncateTextBigLimit(t *testing.T) {
	str, truncated := TruncateText("世界=15 世Hola/~~%&/()汉字 漢字)!%&(12342542452世界", 100)
	assert.Equal(t, "世界=15 世Hola/~~%&/()汉字 漢字)!%&(12342542452世界", str)
	assert.Equal(t, false, truncated)
}

func benchmarkTruncatePerfData(str string, limit uint, b *testing.B) {
	for i := 0; i < b.N; i++ {
		TruncatePerfData(str, limit)
	}
}

func benchmarkTruncateText(str string, limit uint, b *testing.B) {
	for i := 0; i < b.N; i++ {
		TruncateText(str, limit)
	}
}

func BenchmarkTruncateInvalidLimitPerfData(b *testing.B) {
	benchmarkTruncatePerfData("a=1.00 c=10% d=1.0 e=15KB f=2 z=189s", 2, b)
}

func BenchmarkTruncateSinglePerfDataLimit(b *testing.B) {
	benchmarkTruncatePerfData("a=1.00 c=10% d=1.0 e=15KB f=2 z=189s", 6, b)
}

func BenchmarkTruncateSimplePerfData(b *testing.B) {
	benchmarkTruncatePerfData("a=1.00 c=10% d=1.0 e=15KB f=2 z=189s", 65536, b)
}

func BenchmarkTruncateQuotedPerfData(b *testing.B) {
	benchmarkTruncatePerfData("a=1.00 c=10% d=1.0 e=15KB f=2 z=189s 'x y z'=123s 'md n o'=1.0 e=15KB f=2 z=189s", 58, b)
}

func BenchmarkTruncateUtf8PerfData(b *testing.B) {
	benchmarkTruncatePerfData("世界=15 汉字=1.00s '汉字 漢 字'=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s '世 界 世'=123s", 80, b)
}

func BenchmarkTruncateComplexPerfData(b *testing.B) {
	benchmarkTruncatePerfData("世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s '= 汉字 漢 字 x y z kl $! ='=123s", 130, b)
}

func BenchmarkTruncateComplexQuotedPerfData(b *testing.B) {
	benchmarkTruncatePerfData("世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢=10 字=15'=2% z=189s '= 汉字 漢=b 字=10 x=11 y=12 z=13 kl $! ='=123s", 142, b)
}

func BenchmarkTruncatePerfDataBigLimit(b *testing.B) {
	benchmarkTruncatePerfData("世界=15 汉字=1.00s '汉字 漢 字'=10% 字=1.00 世=15KB '汉 漢 字'=2% z=189s '世 界 世'=123s", 200, b)
}

func BenchmarkTruncatePerfDataWorstCase(b *testing.B) {
	benchmarkTruncatePerfData("世界=15 汉字=1.00s '= 汉字 漢 字 x y z $!='=10% 字=1.00 世=15KB '汉 漢=10 字=15 '=2% z=189s '=9 汉字 漢=b 字=10 x=11 y=12 z=13 kl $! '=123s '=9 汉字 漢=b 字=10 x=11 y=12 z=13 kl $! '=123s '汉 漢=10 字=15 '=2% '=汉 漢=10 字=15 '=2% '=汉 漢=10 字=15 '=2%", 281, b)
}

func BenchmarkTruncateSimpleText(b *testing.B) {
	benchmarkTruncateText("da39a3ee5e6b4b0d3255bfef95601890afd80709", 30, b)
}

func BenchmarkTruncateComplexText(b *testing.B) {
	benchmarkTruncateText("世界=15 世Hola/~~%&/()汉字 漢字)!%&(12342542452世界", 37, b)
}

func BenchmarkTruncateTextBigLimit(b *testing.B) {
	benchmarkTruncateText("世界=15 世Hola/~~%&/()汉字 漢字)!%&(12342542452世界", 100, b)
}
