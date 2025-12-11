package annotae

import "gorm.io/gen"

// Querier is the interface for querying the database.
type Querier interface {
	// SELECT * FROM @@table WHERE id=@id
	GetByID(id int) (gen.T, error) // returns struct and error
}
