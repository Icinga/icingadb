// IcingaDB | (c) 2019 Icinga GmbH | GPLv2+

package connection

import (
	"errors"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"testing"
)

type TestRow struct {
	Name string
}

func (*TestRow) InsertValues() []interface{} {
	return nil
}

func (*TestRow) UpdateValues() []interface{} {
	return nil
}

func (*TestRow) GetId() string {
	return ""
}

func (*TestRow) SetId(id string) {
}

func (*TestRow) GetFinalRows() ([]Row, error) {
	return nil, nil
}

func TestMakePlaceholderList(t *testing.T) {
	assert.Equal(t, "(?)", MakePlaceholderList(1))
	assert.Equal(t, "(?,?,?,?,?)", MakePlaceholderList(5))
	assert.Equal(t, "(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", MakePlaceholderList(20))
}

func TestConvertValueForDb(t *testing.T) {
	var v interface{}

	v = ConvertValueForDb(nil)
	assert.IsType(t, nil, v)

	v = ConvertValueForDb([]byte{100})
	assert.IsType(t, []byte{100}, v)

	v = ConvertValueForDb("this-is-a-string")
	assert.IsType(t, "this-is-a-string", v)

	v = ConvertValueForDb(float32(123.456))
	assert.IsType(t, float64(123.456), v)

	v = ConvertValueForDb(float64(123.456))
	assert.IsType(t, float64(123.456), v)

	v = ConvertValueForDb(uint(20))
	assert.IsType(t, int64(10), v)

	v = ConvertValueForDb(uint8(30))
	assert.IsType(t, int64(10), v)

	v = ConvertValueForDb(uint16(40))
	assert.IsType(t, int64(10), v)

	v = ConvertValueForDb(uint32(50))
	assert.IsType(t, int64(10), v)

	v = ConvertValueForDb(uint64(60))
	assert.IsType(t, int64(10), v)

	v = ConvertValueForDb(int(70))
	assert.IsType(t, int64(10), v)

	v = ConvertValueForDb(int8(80))
	assert.IsType(t, int64(10), v)

	v = ConvertValueForDb(int16(90))
	assert.IsType(t, int64(10), v)

	v = ConvertValueForDb(int32(100))
	assert.IsType(t, int64(10), v)

	v = ConvertValueForDb([][]string{{"123123"}, {"aosndkajsd"}, {"97z1h28idnaks", "asdnjaksdj", "h9iu2rh39nu2"}})
	assert.Equal(t, "[[123123] [aosndkajsd] [97z1h28idnaks asdnjaksdj h9iu2rh39nu2]]", v)

	v = ConvertValueForDb(true)
	assert.Equal(t, "y", v)
}

func TestIsSerializationFailure(t *testing.T) {
	assert.True(t, isSerializationFailure(&mysql.MySQLError{Number: 1205}))
	assert.True(t, isSerializationFailure(&mysql.MySQLError{Number: 1213}))

	assert.False(t, isSerializationFailure(&mysql.MySQLError{Number: 6342}))
	assert.False(t, isSerializationFailure(errors.New("random error")))
}

func TestMysqlConnectionError_Error(t *testing.T) {
	err := MysqlConnectionError{"The chicken has left the database!"}
	assert.Equal(t, "The chicken has left the database!", err.Error())
}

func TestFormatLogQuery(t *testing.T) {
	assert.Equal(t, "This is my string", formatLogQuery("\tThis is\nmy string\n"))
}

func TestChunkRows(t *testing.T) {
	rows := []Row{
		&TestRow{"herp"},
		&TestRow{"derp"},
		&TestRow{"merp"},
		&TestRow{"lerp"},
		&TestRow{"perp"},
	}

	want := [][]Row{
		{
			rows[0],
			rows[1],
			rows[2],
			rows[3],
			rows[4],
		},
	}
	chunks := ChunkRows(rows, 5)
	assert.Equal(t, want, chunks)

	want = [][]Row{
		{
			rows[0],
			rows[1],
			rows[2],
			rows[3],
			rows[4],
		},
	}
	chunks = ChunkRows(rows, 10)
	assert.Equal(t, want, chunks)

	want = [][]Row{
		{
			rows[0],
		},
		{
			rows[1],
		},
		{
			rows[2],
		},
		{
			rows[3],
		},
		{
			rows[4],
		},
	}
	chunks = ChunkRows(rows, 1)
	assert.Equal(t, want, chunks)

	want = [][]Row{
		{
			rows[0],
			rows[1],
		},
		{
			rows[2],
			rows[3],
		},
		{
			rows[4],
		},
	}
	chunks = ChunkRows(rows, 2)
	assert.Equal(t, want, chunks)

	{
		rows := make([]Row, 501)
		for i := 0; i <= 501; i++ {
			for j := 1; j <= 501; j++ {
				for _, chunk := range ChunkRows(rows[:i], j) {
					assert.NotEqual(t, 0, len(chunk))
				}
			}
		}
	}
}
