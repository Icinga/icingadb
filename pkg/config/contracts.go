package config

type Validator interface {
	Validate() error
}
