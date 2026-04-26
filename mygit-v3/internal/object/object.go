package object

// Type represents a Git object type.
type Type string

const (
	TypeBlob   Type = "blob"
	TypeTree   Type = "tree"
	TypeCommit Type = "commit"
	TypeTag    Type = "tag"
)

// Object is the common interface for all Git objects.
type Object interface {
	Type() Type
	// Serialize returns the raw content bytes (NOT the header, NOT compressed).
	Serialize() ([]byte, error)
}
