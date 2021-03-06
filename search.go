package zorm

type search struct {
	db               *DB
	whereConditions  []map[string]interface{}
	orConditions     []map[string]interface{}
	notConditions    []map[string]interface{}
	havingConditions []map[string]interface{}
	joinConditions   []map[string]interface{}
	//initAttrs        []interface{}
	//assignAttrs      []interface{}
	selects string
	omits   []string
	orders  []interface{}
	//preload          []searchPreload
	offset           interface{}
	limit            interface{}
	group            string
	tableName        string
	raw              bool
	Unscoped         bool
	ignoreOrderQuery bool
}

func (s *search) clone() *search {
	clone := *s
	return &clone
}

func (s *search) Where(query interface{}, values ...interface{}) *search {
	s.whereConditions = append(s.whereConditions, map[string]interface{}{"query": query, "args": values})
	return s
}

func (s *search) Not(query interface{}, values ...interface{}) *search {
	s.notConditions = append(s.notConditions, map[string]interface{}{"query": query, "args": values})
	return s
}

func (s *search) Or(query interface{}, values ...interface{}) *search {
	s.orConditions = append(s.orConditions, map[string]interface{}{"query": query, "args": values})
	return s
}

func (s *search) Order(value interface{}, reorder ...bool) *search {
	if len(reorder) > 0 && reorder[0] {
		s.orders = []interface{}{}
	}
	if value != nil && value != "" {
		s.orders = append(s.orders, value)
	}
	return s
}

func (s *search) Select(query string) *search {
	s.selects = query
	return s
}

func (s *search) Omit(columns ...string) *search {
	s.omits = columns
	return s
}

func (s *search) Limit(limit interface{}) *search {
	s.limit = limit
	return s
}

func (s *search) Offset(offset interface{}) *search {
	s.offset = offset
	return s
}

func (s *search) Group(query string) *search {
	s.group = query
	return s
}

func (s *search) Having(query interface{}, values ...interface{}) *search {
	s.havingConditions = append(s.havingConditions, map[string]interface{}{"query": query, "args": values})
	return s
}

func (s *search) Joins(query string, values ...interface{}) *search {
	s.joinConditions = append(s.joinConditions, map[string]interface{}{"query": query, "args": values})
	return s
}

func (s *search) Table(name string) *search {
	s.tableName = name
	return s
}

func (s *search) unscoped() *search {
	s.Unscoped = true
	return s
}

func (s *search) Raw(b bool) *search {
	s.raw = b
	return s
}
