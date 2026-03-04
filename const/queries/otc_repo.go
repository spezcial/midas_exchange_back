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
			id, uid, user_id, operator_id,
			from_currency_id, to_currency_id,
			from_amount, proposed_rate, agreed_rate, agreed_from_amount, to_amount,
			status, comment, cancel_reason, cancelled_by, payment_deadline,
			created_at, updated_at
		FROM otc_orders
		WHERE user_id = $1
`

	OTCOrderCountByUserBaseQuery = `SELECT COUNT(*) FROM otc_orders WHERE user_id = $1`

	OTCOrderListAllBaseQuery = `
		SELECT
			id, uid, user_id, operator_id,
			from_currency_id, to_currency_id,
			from_amount, proposed_rate, agreed_rate, agreed_from_amount, to_amount,
			status, comment, cancel_reason, cancelled_by, payment_deadline,
			created_at, updated_at
		FROM otc_orders
`

	OTCOrderCountAllBaseQuery = `SELECT COUNT(*) FROM otc_orders`

	OTCOrderTakeQuery = `
		UPDATE otc_orders
		SET operator_id = $1, status = 'negotiating', updated_at = NOW()
		WHERE id = $2
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
		WHERE id = $1
		RETURNING updated_at
`

	OTCMessageCreateQuery = `
		INSERT INTO otc_messages (order_id, sender_id, sender_role, message_type, content, offer_rate, offer_from_amount, offer_to_amount, offer_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
`

	OTCMessageListByOrderQuery = `
		SELECT id, order_id, sender_id, sender_role, message_type, content,
		       offer_rate, offer_from_amount, offer_to_amount, offer_status, created_at
		FROM otc_messages
		WHERE order_id = $1
		ORDER BY created_at ASC
`

	OTCMessageGetByIDQuery = `
		SELECT id, order_id, sender_id, sender_role, message_type, content,
		       offer_rate, offer_from_amount, offer_to_amount, offer_status, created_at
		FROM otc_messages
		WHERE id = $1
`

	OTCMessageUpdateOfferStatusQuery = `
		UPDATE otc_messages
		SET offer_status = $1
		WHERE id = $2
`
)
