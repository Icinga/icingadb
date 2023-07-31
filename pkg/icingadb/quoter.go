package icingadb

import (
	"fmt"
	"github.com/icinga/icingadb/pkg/driver"
	"github.com/jmoiron/sqlx"
	"strings"
)

// Quoter provides utility functions for quoting table names and columns,
// where the quote character depends on the database driver used.
type Quoter struct {
	quoteCharacter string
}

// NewQuoter creates and returns a new Quoter
// carrying the quote character appropriate for the given database connection.
func NewQuoter(db *sqlx.DB) *Quoter {
	var qc string

	switch db.DriverName() {
	case driver.MySQL:
		qc = "`"
	case driver.PostgreSQL:
		qc = `"`
	default:
		panic("unknown driver " + db.DriverName())
	}

	return &Quoter{quoteCharacter: qc}
}

// BuildAssignmentList quotes the specified columns into `column = :column` pairs for safe use in named query parts,
// i.e. `UPDATE ... SET assignment_list` and `SELECT ... WHERE where_condition`.
func (q *Quoter) BuildAssignmentList(columns []string) []string {
	assign := make([]string, 0, len(columns))
	for _, col := range columns {
		assign = append(assign, fmt.Sprintf("%s = :%s", q.QuoteIdentifier(col), col))
	}

	return assign
}

// QuoteColumnList quotes the given columns into a single comma concatenated string
// so that they can be safely used as a column list for SELECT and INSERT statements.
func (q *Quoter) QuoteColumnList(columns []string) string {
	return fmt.Sprintf("%[1]s%s%[1]s", q.quoteCharacter, strings.Join(columns, q.quoteCharacter+", "+q.quoteCharacter))
}

// QuoteIdentifier quotes the given identifier so that it can be safely used as table or column name,
// even if it is a reserved name where the quote character depends on the database driver used.
func (q *Quoter) QuoteIdentifier(identifier string) string {
	return q.quoteCharacter + identifier + q.quoteCharacter
}
