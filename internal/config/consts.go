package config

type Environment string

const (
	ProdEnvironment            = "prod"
	DevEnvironment             = "dev"
	LocalEnvironment           = "local"
	PerformanceTestEnvironment = "performance_test"
)

func (e Environment) IsDev() bool {
	return e.isDev() || e.IsPerformanceTest()
}

func (e Environment) isDev() bool {
	return e == DevEnvironment || e == LocalEnvironment
}

func (e Environment) IsPerformanceTest() bool {
	return e == PerformanceTestEnvironment
}
