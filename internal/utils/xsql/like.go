// Package xsql provides helpers for building SQL fragments out of values that
// come from outside the system.
package xsql

import "strings"

// likeEscaper neutralizes the LIKE metacharacters in a user-supplied query so
// that "%" and "_" match themselves. Postgres treats a backslash as the default
// escape character, so no ESCAPE clause is needed.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// EscapeLike makes a search string match literally inside a LIKE pattern.
// Without it "%" is a wildcard: searching for it alone returns every row, which
// reads as a filter that quietly stopped filtering.
//
// The caller still adds its own surrounding wildcards, e.g.
// postgres.String("%" + EscapeLike(query) + "%").
func EscapeLike(s string) string {
	return likeEscaper.Replace(s)
}
