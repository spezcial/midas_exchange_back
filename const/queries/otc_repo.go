package queries

const (
	OTCOrderCreateQuery = `
		INSERT INTO otc_orders (user_id, from_currency_id, to_currency_id, from_amount, proposed_rate, status, comment)
		VALUES ($1, $2, $3, $4, $5, 'awaiting_review', $6)
		RETURNING id, uid, created_at, updated_at
`

	OTCOrderGetByUIDQuery = `
		SELECT
			o.id, o.uid, o.user_id, o.operator_id,
			o.from_currency_id, o.to_currency_id,
			o.from_amount, o.proposed_rate, o.agreed_rate, o.agreed_from_amount, o.to_amount,
			o.status, o.comment, o.cancel_reason, o.cancelled_by, o.payment_deadline,
			o.created_at, o.updated_at,
			fc.id as "from_currency.id", fc.code as "from_currency.code",
			fc.name as "from_currency.name", fc.symbol as "from_currency.symbol",
			fc.is_active as "from_currency.is_active", fc.is_crypto as "from_currency.is_crypto",
			fc.created_at as "from_currency.created_at", fc.updated_at as "from_currency.updated_at",
			tc.id as "to_currency.id", tc.code as "to_currency.code",
			tc.name as "to_currency.name", tc.symbol as "to_currency.symbol",
			tc.is_active as "to_currency.is_active", tc.is_crypto as "to_currency.is_crypto",
			tc.created_at as "to_currency.created_at", tc.updated_at as "to_currency.updated_at"
		FROM otc_orders o
		JOIN currencies fc ON o.from_currency_id = fc.id
		JOIN currencies tc ON o.to_currency_id = tc.id
		WHERE o.uid = $1
`

	OTCOrderListByUserBaseQuery = `
		SELECT
			o.id, o.uid, o.user_id, o.operator_id,
			o.from_currency_id, o.to_currency_id,
			fc.code AS from_currency_code, tc.code AS to_currency_code,
			o.from_amount, o.proposed_rate, o.agreed_rate, o.agreed_from_amount, o.to_amount,
			o.status, o.comment, o.cancel_reason, o.cancelled_by, o.payment_deadline,
			o.created_at, o.updated_at,
			(SELECT COUNT(*) FROM otc_messages m WHERE m.order_id = o.id AND m.sender_role != 'client' AND m.is_read = false) AS unread_count
		FROM otc_orders o
		JOIN currencies fc ON fc.id = o.from_currency_id
		JOIN currencies tc ON tc.id = o.to_currency_id
		WHERE o.user_id = $1
`

	OTCOrderCountByUserBaseQuery = `SELECT COUNT(*) FROM otc_orders WHERE user_id = $1`

	OTCOrderTakeQuery = `
		UPDATE otc_orders
		SET operator_id = $1, status = 'negotiating', updated_at = NOW()
		WHERE id = $2 AND status = 'awaiting_review'
		RETURNING updated_at
`

	OTCOrderAgreeQuery = `
		UPDATE otc_orders
		SET agreed_rate = $1, agreed_from_amount = $2, to_amount = $3,
		    status = 'awaiting_payment', payment_deadline = $4, updated_at = NOW()
		WHERE id = $5
		RETURNING updated_at
`

	OTCOrderCancelQuery = `
		UPDATE otc_orders
		SET status = 'cancelled', cancel_reason = $1, cancelled_by = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING updated_at
`

	OTCOrderSetPaymentReceivedQuery = `
		UPDATE otc_orders
		SET status = 'payment_received', updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
`

	OTCOrderCompleteQuery = `
		UPDATE otc_orders
		SET status = 'completed', updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
`

	OTCOrderExpireQuery = `
		UPDATE otc_orders
		SET status = 'expired', updated_at = NOW()
		WHERE id = $1 AND status = 'awaiting_payment'
		RETURNING updated_at
`

	OTCMessageCreateQuery = `
		INSERT INTO otc_messages (order_id, sender_id, sender_role, message_type, content, offer_rate, offer_from_amount, offer_to_amount, offer_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
`

	OTCMessageListByOrderQuery = `
		SELECT id, order_id, sender_id, sender_role, message_type, content,
		       offer_rate, offer_from_amount, offer_to_amount, offer_status,
		       is_read, read_at, created_at
		FROM otc_messages
		WHERE order_id = $1
		ORDER BY created_at ASC
`

	OTCMessageGetByIDQuery = `
		SELECT id, order_id, sender_id, sender_role, message_type, content,
		       offer_rate, offer_from_amount, offer_to_amount, offer_status,
		       is_read, read_at, created_at
		FROM otc_messages
		WHERE id = $1
`

	OTCMessageMarkReadQuery = `
		UPDATE otc_messages
		SET is_read = true, read_at = NOW()
		WHERE order_id = $1
		  AND sender_id != $2
		  AND is_read = false
`

	OTCMessageUpdateOfferStatusQuery = `
		UPDATE otc_messages
		SET offer_status = $1
		WHERE id = $2 AND offer_status = 'pending'
`
	OTCAllConfigsWithCurrenciesQuery = `
		SELECT
			c.id, c.from_currency_id, c.to_currency_id,
			c.min_from_amount, c.payment_timeout_min, c.is_active,
			c.created_at, c.updated_at,
			fc.id as "from_currency.id", fc.code as "from_currency.code",
			fc.name as "from_currency.name", fc.symbol as "from_currency.symbol",
			fc.is_active as "from_currency.is_active", fc.is_crypto as "from_currency.is_crypto",
			fc.created_at as "from_currency.created_at", fc.updated_at as "from_currency.updated_at",
			tc.id as "to_currency.id", tc.code as "to_currency.code",
			tc.name as "to_currency.name", tc.symbol as "to_currency.symbol",
			tc.is_active as "to_currency.is_active", tc.is_crypto as "to_currency.is_crypto",
			tc.created_at as "to_currency.created_at", tc.updated_at as "to_currency.updated_at"
		FROM otc_config c
		JOIN currencies fc ON c.from_currency_id = fc.id
		JOIN currencies tc ON c.to_currency_id = tc.id
		ORDER BY fc.code, tc.code
`

	OTCActiveConfigsWithCurrenciesQuery = `
		SELECT
			c.id, c.from_currency_id, c.to_currency_id,
			c.min_from_amount, c.payment_timeout_min, c.is_active,
			c.created_at, c.updated_at,
			fc.id as "from_currency.id", fc.code as "from_currency.code",
			fc.name as "from_currency.name", fc.symbol as "from_currency.symbol",
			fc.is_active as "from_currency.is_active", fc.is_crypto as "from_currency.is_crypto",
			fc.created_at as "from_currency.created_at", fc.updated_at as "from_currency.updated_at",
			tc.id as "to_currency.id", tc.code as "to_currency.code",
			tc.name as "to_currency.name", tc.symbol as "to_currency.symbol",
			tc.is_active as "to_currency.is_active", tc.is_crypto as "to_currency.is_crypto",
			tc.created_at as "to_currency.created_at", tc.updated_at as "to_currency.updated_at"
		FROM otc_config c
		JOIN currencies fc ON c.from_currency_id = fc.id
		JOIN currencies tc ON c.to_currency_id = tc.id
		WHERE c.is_active = true
		ORDER BY fc.code, tc.code
`

	OTCConfigListQuery = `
		SELECT id, from_currency_id, to_currency_id, min_from_amount,
		       payment_timeout_min, is_active, created_at, updated_at
		FROM otc_config
		ORDER BY from_currency_id, to_currency_id
`

	OTCConfigGetByIDQuery = `
		SELECT id, from_currency_id, to_currency_id, min_from_amount,
		       payment_timeout_min, is_active, created_at, updated_at
		FROM otc_config
		WHERE id = $1
`

	OTCConfigGetByPairQuery = `
		SELECT id, from_currency_id, to_currency_id, min_from_amount,
		       payment_timeout_min, is_active, created_at, updated_at
		FROM otc_config
		WHERE from_currency_id = $1 AND to_currency_id = $2
`

	OTCConfigCreateQuery = `
		INSERT INTO otc_config (from_currency_id, to_currency_id, min_from_amount, payment_timeout_min, is_active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
`

	OTCConfigUpdateQuery = `
		UPDATE otc_config
		SET min_from_amount = $1, payment_timeout_min = $2, is_active = $3, updated_at = NOW()
		WHERE id = $4
		RETURNING updated_at
`

	OTCConfigDeleteQuery = `DELETE FROM otc_config WHERE id = $1`

	// --- Audit log ---

	OTCAuditLogCreateQuery = `
		INSERT INTO otc_audit_log (order_id, actor_id, actor_role, action, details)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
`

	OTCAuditLogListByOrderQuery = `
		SELECT id, order_id, actor_id, actor_role, action, details, created_at
		FROM otc_audit_log
		WHERE order_id = $1
		ORDER BY created_at ASC
`

	// --- Analytics ---

	OTCAnalyticsSummaryQuery = `
		SELECT
			COUNT(*)                                                              AS total_orders,
			COUNT(*) FILTER (WHERE status = 'completed')                         AS completed,
			COUNT(*) FILTER (WHERE status = 'cancelled')                         AS cancelled,
			COUNT(*) FILTER (WHERE status = 'expired')                           AS expired,
			COALESCE(SUM(CASE WHEN status = 'completed' THEN agreed_from_amount ELSE 0 END), 0) AS total_volume,
			COALESCE(AVG(CASE WHEN agreed_rate IS NOT NULL AND proposed_rate > 0
			               THEN ABS(agreed_rate - proposed_rate) / proposed_rate * 100.0
			               END), 0.0)                                            AS avg_spread_pct
		FROM otc_orders
		WHERE created_at >= $1 AND created_at < $2
`

	// OTCAnalyticsByPeriodQueryTpl: %s is replaced with the granularity (day/week/month).
	OTCAnalyticsByPeriodQueryTpl = `
		SELECT
			TO_CHAR(DATE_TRUNC('%s', created_at), 'YYYY-MM-DD') AS period,
			COUNT(*)                                              AS total,
			COUNT(*) FILTER (WHERE status = 'completed')         AS completed,
			COUNT(*) FILTER (WHERE status = 'cancelled')         AS cancelled,
			COUNT(*) FILTER (WHERE status = 'expired')           AS expired,
			COALESCE(SUM(CASE WHEN status = 'completed' THEN agreed_from_amount ELSE 0 END), 0) AS volume
		FROM otc_orders
		WHERE created_at >= $1 AND created_at < $2
		GROUP BY DATE_TRUNC('%s', created_at)
		ORDER BY DATE_TRUNC('%s', created_at) ASC
`

	// --- Export ---

	OTCOrderExportBaseQuery = `
		SELECT
			o.uid,
			o.status,
			cu.email                AS client_email,
			ou.email                AS operator_email,
			fc.code                 AS from_currency_code,
			tc.code                 AS to_currency_code,
			o.from_amount,
			o.proposed_rate,
			o.agreed_rate,
			o.agreed_from_amount,
			o.to_amount,
			o.cancel_reason,
			o.created_at,
			o.updated_at
		FROM otc_orders o
		JOIN  users      cu ON o.user_id       = cu.id
		LEFT  JOIN users ou ON o.operator_id   = ou.id
		JOIN  currencies fc ON o.from_currency_id = fc.id
		JOIN  currencies tc ON o.to_currency_id   = tc.id
`
)
