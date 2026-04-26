package object

// Blob represents a file blob object.
type Blob struct {
	Data []byte
}

func (b *Blob) Type() Type { return TypeBlob }

func (b *Blob) Serialize() ([]byte, error) {
	return b.Data, nil
}

// NewBlob creates a Blob from raw bytes.
func NewBlob(data []byte) *Blob {
	return &Blob{Data: data}
}

// ParseBlob parses raw content bytes into a Blob.
func ParseBlob(content []byte) (*Blob, error) {
	return &Blob{Data: content}, nil
}
