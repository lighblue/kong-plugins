package interfaces

type Cache interface {
	Get(key string) (interface{}, error)
	Set(key string, value interface{}) error
}
