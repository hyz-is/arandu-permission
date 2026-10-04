package unit_test

import (
	"testing"

	"github.com/arandu-io/hesape/database/query"

	permission "github.com/hyz-is/arandu-permission"
)

// A consumer locks a group row before a read-modify-write that spans this
// package and its own tables. The generated query exposes no lock, so the
// statement under it must stay reachable from outside the package.
var _ func(*permission.GroupQuery) *query.Builder = (*permission.GroupQuery).GetQuery

func TestAGroupRowCanBeLockedByAConsumer(t *testing.T) {
	groups := permission.Groups(nil).Where("slug", "=", "owners")
	statement := groups.GetQuery()
	if statement == nil {
		t.Fatal("GetQuery returned no statement")
	}
	statement.LockForUpdate()
}
