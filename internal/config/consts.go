package config

type Environment string

const (
	ProdEnvironment  = "prod"
	DevEnvironment   = "dev"
	LocalEnvironment = "local"
)

func (e Environment) IsDev() bool {
	return e == DevEnvironment || e == LocalEnvironment
}
