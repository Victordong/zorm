package zorm

import (
	"database/sql"
	"errors"
	"sync"
	"time"
)

type SQLCommon interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Begin() (*sql.Tx, error)
}

type sqlTx interface {
	Commit() error
	Rollback() error
}

type DB struct {
	Value        interface{}
	Error        error
	RowsAffected int64

	db      SQLCommon
	logger  Logger
	logMode int

	search    *search
	values    sync.Map
	Callbacks *Callback
}

const (
	defaultLogMode = 1
	noLogMode      = 2
)

func Open(driver string, args ...interface{}) (*DB, error) {
	if len(args) == 0 {
		err := errors.New("invalid database source")
		return nil, err
	}
	var source string
	var dbSQL SQLCommon
	switch args[0].(type) {
	case string:
		source = args[0].(string)
	default:
		return nil, errors.New("invalid database source")
	}
	dbSQL, err := sql.Open(driver, source)
	if err != nil {
		return nil, err
	}
	db := &DB{
		db:        dbSQL,
		Callbacks: DefaultCallback,
	}
	// Send a ping to make sure the database connection is alive.
	if d, ok := dbSQL.(*sql.DB); ok {
		if err = d.Ping(); err != nil {
			d.Close()
		}
	}
	return db, nil
}

func (s *DB) New() *DB {
	clone := s.clone()
	clone.search = nil
	clone.Value = nil
	return clone
}

func (s *DB) NewScope(value interface{}) *Scope {
	dbClone := s.clone()
	dbClone.Value = value
	return &Scope{db: dbClone, Search: dbClone.search, Value: value}
}

func (s *DB) clone() *DB {
	db := &DB{
		db:        s.db,
		Error:     s.Error,
		logger:    s.logger,
		Value:     s.Value,
		Callbacks: s.Callbacks,
	}

	s.values.Range(func(k, v interface{}) bool {
		db.values.Store(k, v)
		return true
	})

	if s.search == nil {
		db.search = &search{limit: -1, offset: -1}
	} else {
		db.search = s.search.clone()
	}
	db.search.db = db
	return db
}

func (s *DB) SetLoggerMode(loggerMode int) {
	s.logMode = loggerMode
}

func (s *DB) SetLogger(logger Logger) {
	s.logger = logger
}

func (s *DB) Where(query interface{}, args ...interface{}) *DB {
	return s.clone().search.Where(query, args...).db
}

func (s *DB) Or(query interface{}, args ...interface{}) *DB {
	return s.clone().search.Or(query, args...).db
}

func (s *DB) Not(query interface{}, args ...interface{}) *DB {
	return s.clone().search.Not(query, args...).db
}

func (s *DB) Limit(limit interface{}) *DB {
	return s.clone().search.Limit(limit).db
}

func (s *DB) Offset(offset interface{}) *DB {
	return s.clone().search.Offset(offset).db
}

func (s *DB) Order(value interface{}, reorder ...bool) *DB {
	return s.clone().search.Order(value, reorder...).db
}

func (s *DB) Select(query string) *DB {
	return s.clone().search.Select(query).db
}

func (s *DB) Omit(columns ...string) *DB {
	return s.clone().search.Omit(columns...).db
}

func (s *DB) Table(tableName string) *DB {
	return s.clone().search.Table(tableName).db
}
func (s *DB) Group(query string) *DB {
	return s.clone().search.Group(query).db
}

func (s *DB) Having(query interface{}, values ...interface{}) *DB {
	return s.clone().search.Having(query, values).db
}

func (s *DB) Joins(query string, args ...interface{}) *DB {
	return s.clone().search.Joins(query, args).db
}

func (s *DB) Raw(sql string, values ...interface{}) *DB {
	return s.clone().search.Raw(true).Where(sql, values...).db
}

func (s *DB) Begin() *DB {
	c := s.clone()
	tx, err := c.db.Begin()
	c.db = interface{}(tx).(SQLCommon)
	c.AddError(err)
	return c
}

func (s *DB) Commit() *DB {
	var emptySQLTx *sql.Tx
	if db, ok := s.db.(sqlTx); ok && db != nil && db != emptySQLTx {
		s.AddError(db.Commit())
	}
	return s
}

func (s *DB) Rollback() *DB {
	var emptySQLTx *sql.Tx
	if db, ok := s.db.(sqlTx); ok && db != nil && db != emptySQLTx {
		s.AddError(db.Rollback())
	}
	return s
}

func (s *DB) Scopes(funcs ...func(*DB) *DB) *DB {
	for _, f := range funcs {
		s = f(s)
	}
	return s
}

func (s *DB) Unscoped() *DB {
	return s.clone().search.unscoped().db
}

func (s *DB) First(out interface{}, where ...interface{}) *DB {
	newScope := s.NewScope(out)
	newScope.Search.Limit(1)
	return newScope.Set("gorm:order_by_primary_key", "ASC").callCallbacks(s.Callbacks.Queries).db
}

func (s *DB) Last(out interface{}, where ...interface{}) *DB {
	newScope := s.NewScope(out)
	newScope.Search.Limit(1)
	return newScope.Set("gorm:order_by_primary_key", "DESC").db
}

func (s *DB) Find(out interface{}, where ...interface{}) *DB {
	scope := s.NewScope(out)
	return scope.callCallbacks(s.Callbacks.Queries).db
}

func (s *DB) Insert(value interface{}) *DB {
	scope := s.NewScope(value)
	return scope.callCallbacks(s.Callbacks.Creates).db
}

func (s *DB) Update(value interface{}) *DB {
	scope := s.NewScope(value)
	return scope.callCallbacks(s.Callbacks.Updates).db
}
func (s *DB) Scan(dest interface{}) *DB {
	return s.NewScope(s.Value).Set("gorm:query_destination", dest).db
}

func (s *DB) Delete() *DB {
	scope := s.NewScope(nil)
	return scope.callCallbacks(s.Callbacks.Deletes).db
}

func (s *DB) Row() *sql.Row {
	return s.NewScope(s.Value).row()
}

func (s *DB) Rows() (*sql.Rows, error) {
	return s.NewScope(s.Value).rows()
}

func (s *DB) ScanRows(rows *sql.Rows, result interface{}) error {
	var (
		scope  = s.NewScope(result)
		clone  = scope.db
		_, err = rows.Columns()
	)
	if clone.AddError(err) == nil {
		return nil
	}
	return clone.Error
}

func (s *DB) Set(name string, value interface{}) *DB {
	return s.clone().InstantSet(name, value)
}

func (s *DB) InstantSet(name string, value interface{}) *DB {
	s.values.Store(name, value)
	return s
}

func (s *DB) Get(name string) (interface{}, bool) {
	value, ok := s.values.Load(name)
	return value, ok
}

func (s *DB) AddError(err error) error {
	return err
}

func (s *DB) print(values ...interface{}) {
	s.logger.Print(values)
}

func (s *DB) slog(sql string, t time.Time, vars ...interface{}) {
	if s.logMode == defaultLogMode {
		s.print("sql", sql, vars, s.RowsAffected)
	}
}
