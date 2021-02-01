package index_encoder

type Interface interface {
	NewEmpty() (ValueInterface, error)
	NewFromData(data []byte) (ValueInterface, error)
}

type ValueInterface interface {
	Add(...string) error
	Serialize() ([]byte, error)
	GetIds() ([]string, error)
}
