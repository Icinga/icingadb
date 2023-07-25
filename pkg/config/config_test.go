package config

import (
	"github.com/creasty/defaults"
	"github.com/icinga/icingadb/pkg/logging"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestFromYAMLFile(t *testing.T) {
	const miniConf = `
database:
  host: 192.0.2.1
  database: icingadb
  user: icingadb
  password: icingadb

redis:
  host: 2001:db8::1
`

	subtests := []struct {
		name   string
		input  string
		output *Config
	}{
		{
			name:  "mini",
			input: miniConf,
			output: func() *Config {
				c := &Config{}
				_ = defaults.Set(c)

				c.Database.Host = "192.0.2.1"
				c.Database.Database = "icingadb"
				c.Database.User = "icingadb"
				c.Database.Password = "icingadb"

				c.Redis.Host = "2001:db8::1"
				c.Logging.Output = logging.CONSOLE

				return c
			}(),
		},
		{
			name:   "mini-with-unknown",
			input:  miniConf + "\nunknown: 42",
			output: nil,
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			tempFile, err := os.CreateTemp("", "")
			require.NoError(t, err)
			defer func() { _ = os.Remove(tempFile.Name()) }()

			require.NoError(t, os.WriteFile(tempFile.Name(), []byte(st.input), 0o600))

			if actual, err := FromYAMLFile(tempFile.Name()); st.output == nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, st.output, actual)
			}
		})
	}
}
