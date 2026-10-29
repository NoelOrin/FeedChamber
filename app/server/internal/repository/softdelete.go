package repository

// SQLSoftDeleteCondition returns the WHERE clause for filtering soft-deleted rows.
// If prefix is provided (e.g. "p"), it returns "p.deleted_at IS NULL".
func SQLSoftDeleteCondition(prefix ...string) string {
	col := "deleted_at"
	if len(prefix) > 0 && prefix[0] != "" {
		col = prefix[0] + ".deleted_at"
	}
	return col + " IS NULL"
}
