package kb

// Store defines the interface for the space knowledge base.
type Store interface {
	Get(namespace, key string) ([]byte, error)
	Set(namespace, key string, value []byte) error
	Delete(namespace, key string) error
	List(namespace string) ([]string, error)
	Close() error
}
