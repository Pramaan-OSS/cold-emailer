package repository

// warmupStandingRankSQL orders health states worst first when sorted DESC.
func warmupStandingRankSQL(col string) string {
	return `CASE ` + col + `
			WHEN 'blocked' THEN 5
			WHEN 'quarantined' THEN 4
			WHEN 'throttled' THEN 3
			WHEN 'watch' THEN 2
			WHEN 'healthy' THEN 1
			ELSE 0
		END`
}

// warmupStandingSQL is a mailbox's worst warmup standing, at most one row:
// mailbox it warms (which has no local pool row). A standing past its term
// ranks below any live one, so it cannot mask one. Every read that gates or
// shows warmup health goes through it, so a cloud verdict holds here too.
// Columns: health_state, blocked_until, last_health_score,
// last_health_reason, last_health_evaluated_at.
func warmupStandingSQL(accountExpr string) string {
	return `SELECT wpp.health_state::text AS health_state, wpp.blocked_until, wpp.last_health_score, wpp.last_health_reason, wpp.last_health_evaluated_at FROM warmup_pool_participants wpp WHERE wpp.email_account_id = ` + accountExpr + ` LIMIT 1`
}
