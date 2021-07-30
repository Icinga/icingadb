package v1

type Environment struct {
	EntityWithoutChecksum `json:",inline"`
	Name                  string `json:"name"`
}
