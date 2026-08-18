package v1

import (
	"github.com/icinga/icinga-go-library/types"
)

// IcingadbConfig contains configuration in the environment key/value format, currently only used for Notifications.
type IcingadbConfig struct {
	EnvironmentMeta `json:",inline"`
	EndpointId      types.Binary `json:"endpoint_id"`
	EnvKey          string       `json:"env_key"`
	EnvValue        string       `json:"env_value"`
	Locked          types.Bool   `json:"locked"`
}

// UnconfiguredEndpointId returns an IcingadbConfig.EndpointId for endpoint-unspecific settings.
func UnconfiguredEndpointId() types.Binary {
	return make(types.Binary, 20)
}
