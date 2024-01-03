package dbdrivers

type MemoryConfig struct {
	On        bool
	DelKeyAPI struct {
		EndPoint        string
		AuthBearerToken string
	}
}
