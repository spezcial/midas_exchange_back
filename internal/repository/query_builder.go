package repository

// QueryBuilder helps build dynamic SQL queries with WHERE clauses
type QueryBuilder struct {
	baseQuery    string
	whereClauses []string
	args         []interface{}
	paramCounter int
}

func newQueryBuilder(baseQuery string) *QueryBuilder {
	return &QueryBuilder{
		baseQuery:    baseQuery,
		whereClauses: []string{},
		args:         []interface{}{},
		paramCounter: 1,
	}
}

func (qb *QueryBuilder) AddWhere(condition string, arg interface{}) *QueryBuilder {
	qb.whereClauses = append(qb.whereClauses, condition)
	qb.args = append(qb.args, arg)
	qb.paramCounter++
	return qb
}

func (qb *QueryBuilder) Build(orderBy, limitOffset string) (string, []interface{}) {
	query := qb.baseQuery

	if len(qb.whereClauses) > 0 {
		query += "\nWHERE " + qb.whereClauses[0]
		for i := 1; i < len(qb.whereClauses); i++ {
			query += " AND " + qb.whereClauses[i]
		}
	}

	if orderBy != "" {
		query += "\n" + orderBy
	}

	if limitOffset != "" {
		query += "\n" + limitOffset
	}

	return query, qb.args
}
