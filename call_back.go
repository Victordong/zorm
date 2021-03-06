package zorm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

var DefaultCallback = &Callback{}

type Callback struct {
	Creates    []func(scope *Scope)
	Updates    []func(scope *Scope)
	Deletes    []func(scope *Scope)
	Queries    []func(scope *Scope)
	RowQueries []func(scope *Scope)
	processors []*Callback
}

func init() {
	DefaultCallback.Queries = append(DefaultCallback.Queries, queryCallBack)
	DefaultCallback.Creates = append(DefaultCallback.Creates, insertCallBack)
	DefaultCallback.Updates = append(DefaultCallback.Updates, updateCallBack)
	DefaultCallback.Deletes = append(DefaultCallback.Deletes, deleteCallBack)
}

type CallbackProcesser struct {
	name      string
	before    string
	after     string
	replace   string
	remove    string
	kind      string
	processor *func(scope *Scope)
	parent    *Callback
}

type RowQueryResult struct {
	Row *sql.Row
}

type RowsQueryResult struct {
	Rows  *sql.Rows
	Error error
}

func queryCallBack(scope *Scope) {
	defer scope.trace()
	var (
		isSlice, isPtr bool
		resultType     reflect.Type
		results        = scope.IndirectValue()
	)

	if kind := results.Kind(); kind == reflect.Slice {
		isSlice = true
		resultType = results.Type().Elem()
		results.Set(reflect.MakeSlice(results.Type(), 0, 0))

		if resultType.Kind() == reflect.Ptr {
			isPtr = true
			resultType = resultType.Elem()
		}
	} else if kind != reflect.Struct {
		scope.Err(errors.New("unsupported destination, should be slice or struct"))
		return
	}

	scope.prepareQuerySQL()

	if !scope.HasError() {
		scope.db.RowsAffected = 0
		if rows, err := scope.db.db.Query(scope.SQL, scope.SQLVars...); scope.Err(err) == nil {
			columns, _ := rows.Columns()
			for rows.Next() {
				scope.db.RowsAffected++

				elem := results
				if isSlice {
					elem = reflect.New(resultType).Elem()
				}

				scope.scan(rows, columns, scope.New(elem.Addr().Interface()).Fields())

				if isSlice {
					if isPtr {
						results.Set(reflect.Append(results, elem.Addr()))
					} else {
						results.Set(reflect.Append(results, elem))
					}
				}
			}

			if err := rows.Err(); err != nil {
				scope.Err(err)
			}
		}
	}

}

func insertCallBack(scope *Scope) {
	defer scope.trace()
	if !scope.HasError() {
		var (
			columns      []string
			placeholders []string
		)
		now := time.Now()
		if updatedAtField, hasUpdatedAtField := scope.FieldByName("UpdatedAt"); hasUpdatedAtField {
			updatedAtField.Set(now)
		}
		if insertedAtField, hasInsertedAtField := scope.FieldByName("UpdatedAt"); hasInsertedAtField {
			insertedAtField.Set(now)
		}
		for _, field := range scope.Fields() {
			if !field.IsPrimaryKey {
				columns = append(columns, field.DBName)
				placeholders = append(placeholders, scope.AddToVars(field.Field.Interface()))
			}
		}
		scope.Raw(fmt.Sprintf("INSERT INTO %v (%v) VALUES (%v)",
			scope.QuotedTableName(),
			strings.Join(columns, ","),
			strings.Join(placeholders, ","),
		))
		if result, err := scope.db.db.Exec(scope.SQL, scope.SQLVars...); scope.Err(err) == nil {
			scope.db.RowsAffected, _ = result.RowsAffected()
		}
	}
}

func updateCallBack(scope *Scope) {
	defer scope.trace()
	if !scope.HasError() {
		var sqls []string
		now := time.Now()
		if updatedAtField, hasUpdatedAtField := scope.FieldByName("UpdatedAt"); hasUpdatedAtField {
			updatedAtField.Set(now)
		}
		for _, field := range scope.Fields() {
			if !field.IsPrimaryKey && field.IsNormal {
				sqls = append(sqls, fmt.Sprintf("%v = %v", field.DBName, scope.AddToVars(field.Field.Interface())))
			}
		}
		if len(sqls) > 0 {
			scope.Raw(fmt.Sprintf("UPDATE %v SET %v %v",
				scope.QuotedTableName(),
				strings.Join(sqls, ", "),
				scope.CombinedConditionSql(),
			))
		}
		if result, err := scope.db.db.Exec(scope.SQL, scope.SQLVars...); scope.Err(err) == nil {
			scope.db.RowsAffected, _ = result.RowsAffected()
		}
	}
}

func deleteCallBack(scope *Scope) {
	defer scope.trace()
	if !scope.HasError() {
		deletedAtField, hasDeletedAtField := scope.FieldByName("DeletedAt")
		if scope.Search.Unscoped && hasDeletedAtField {
			scope.Raw(fmt.Sprintf("UPDATE %v SET %v=%v %v",
				scope.QuotedTableName(),
				deletedAtField.DBName,
				time.Now(),
				scope.CombinedConditionSql(),
			))
		} else {
			scope.Raw(fmt.Sprintf("DELETE FROM %v %v",
				scope.QuotedTableName(),
				scope.CombinedConditionSql()),
			)
		}
	}
	if result, err := scope.db.db.Exec(scope.SQL, scope.SQLVars...); scope.Err(err) == nil {
		scope.db.RowsAffected, _ = result.RowsAffected()
	}
}
