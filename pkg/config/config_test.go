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

	miniOutput := &Config{}
	_ = defaults.Set(miniOutput)

	miniOutput.Database.Host = "192.0.2.1"
	miniOutput.Database.Database = "icingadb"
	miniOutput.Database.User = "icingadb"
	miniOutput.Database.Password = "icingadb"

	miniOutput.Redis.Host = "2001:db8::1"
	miniOutput.Logging.Output = logging.CONSOLE

	subtests := []struct {
		name   string
		input  string
		output *Config
		warn   bool
	}{
		{
			name:   "mini",
			input:  miniConf,
			output: miniOutput,
			warn:   false,
		},
		{
			name:   "mini-with-unknown",
			input:  miniConf + "\nunknown: 42",
			output: miniOutput,
			warn:   true,
		},
	}

	for _, st := range subtests {
		t.Run(st.name, func(t *testing.T) {
			tempFile, err := os.CreateTemp("", "")
			require.NoError(t, err)
			defer func() { _ = os.Remove(tempFile.Name()) }()

			require.NoError(t, os.WriteFile(tempFile.Name(), []byte(st.input), 0o600))

			actual, err := FromYAMLFile(tempFile.Name())
			require.NoError(t, err)

			if st.warn {
				require.Error(t, actual.DecodeWarning, "reading config should produce a warning")

				// Reset the warning so that the following require.Equal() doesn't try to compare it.
				actual.DecodeWarning = nil
			} else {
				require.NoError(t, actual.DecodeWarning, "reading config should not produce a warning")
			}

			require.Equal(t, st.output, actual)
		})
	}
}
