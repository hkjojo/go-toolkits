package sql

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	goqu "github.com/doug-martin/goqu/v9"
	"github.com/doug-martin/goqu/v9/exp"
	"github.com/fatih/structs"

	//	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
)

// QueryOp ..
type QueryOp string

// Op defined
const (
	EQ         QueryOp = "eq"
	NEQ        QueryOp = "neq"
	BETWEEN    QueryOp = "between"
	IN         QueryOp = "in"
	NOTIN      QueryOp = "notIn"
	GT         QueryOp = "gt"
	GTE        QueryOp = "gte"
	LT         QueryOp = "lt"
	LTE        QueryOp = "lte"
	LIKE       QueryOp = "like"
	ILIKE      QueryOp = "iLike"
	NOTLIKE    QueryOp = "notLike"
	IS         QueryOp = "is"
	NOTBETWEEN QueryOp = "notBetween"
)

// ArgsFilter ...
type ArgsFilter struct {
	ex        goqu.Ex
	filterMap Condition
	exOr      goqu.ExOr
}

// QueryFilter ...
func QueryFilter(filterMap Condition) *ArgsFilter {
	return &ArgsFilter{
		ex:        make(goqu.Ex),
		filterMap: filterMap,
		exOr:      make(goqu.ExOr),
	}
}

// Update ...
func (f *ArgsFilter) Update(field string,
	filterField string) *ArgsFilter {
	if f.filterMap == nil {
		return f
	}

	value, ok := f.filterMap[filterField]
	if ok {
		f.ex[field] = value
	}
	return f
}

// Set ...
func (f *ArgsFilter) Set(field string, value interface{}) *ArgsFilter {
	f.ex[field] = value
	return f
}

// Ex ...
func (f *ArgsFilter) Ex() map[string]interface{} {
	return f.ex
}

// Where ...
func (f *ArgsFilter) Where(field string, op QueryOp,
	filterField string) *ArgsFilter {
	if f.filterMap == nil {
		return f
	}

	value, ok := f.filterMap[filterField]
	if ok {
		if opEx := f.execOp(op, value); opEx != nil {
			f.ex[field] = opEx
		}

	}
	return f
}

// Or ...
func (f *ArgsFilter) Or(field string, op QueryOp, filterField string) *ArgsFilter {
	if f.filterMap == nil {
		return f
	}

	value, ok := f.filterMap[filterField]
	if ok {
		if opEx := f.execOp(op, value); opEx != nil {
			f.exOr[field] = opEx
		}
	}
	return f
}

func (f *ArgsFilter) execOp(op QueryOp, value interface{}) goqu.Op {
	var opEx goqu.Op
	typ := reflect.TypeOf(value)
	switch op {
	case EQ, NEQ, GT, LT, LTE, GTE, IS, ILIKE:
		opEx = goqu.Op{string(op): value}
	case BETWEEN, NOTBETWEEN:
		if typ == nil {
			return opEx
		}
		kind := typ.Kind()
		v := reflect.ValueOf(value)
		if kind == reflect.Slice && v.Len() == 2 {
			opEx = goqu.Op{
				string(op): goqu.Range(v.Index(0).Interface(), v.Index(1).Interface()),
			}
		}
	case IN, NOTIN:
		if typ == nil {
			return opEx
		}
		kind := typ.Kind()
		v := reflect.ValueOf(value)
		if kind == reflect.Slice && v.Len() > 0 {
			opEx = goqu.Op{string(op): value}
		}

	case LIKE, NOTLIKE:
		opEx = goqu.Op{string(op): "%" + fmt.Sprintf("%v", value) + "%"}
	default:
	}

	return opEx
}

// End ...
func (f *ArgsFilter) End() []goqu.Expression {
	var ex []goqu.Expression
	if len(f.ex) > 0 {
		ex = append(ex, f.ex)
	}
	if len(f.exOr) > 0 {
		ex = append(ex, f.exOr)
	}
	return ex
}

// PageQuery ...
func (db *DataBase) PageQuery(query *goqu.SelectDataset, scaner *gorm.DB, pageIndex int64,
	pageSize int64, outRows interface{}, selectEx ...interface{}) (int64, error) {
	var selectQuery = query
	if selectEx != nil {
		selectQuery = query.Select(selectEx...)
	}

	count, err := db.QueryCount(query, selectEx...)
	if err != nil {
		return 0, err
	}

	selectQuery = query.
		Offset(uint((pageIndex - 1) * pageSize)).
		Limit(uint(pageSize))

	sql, args, err := selectQuery.Prepared(true).ToSQL()
	if err != nil {
		return 0, err
	}

	// use gorm to scan rows
	result := scaner.Raw(sql, args...).Find(outRows)
	if result.Error != nil {
		return 0, result.Error
	}

	return count, nil
}

// QueryCount ...
func (db *DataBase) QueryCount(query *goqu.SelectDataset, selectEx ...interface{}) (int64, error) {
	var (
		selectQuery = query
		count       int64
	)

	if selectEx != nil {
		selectQuery = query.Select(selectEx...)
	}

	sql, args, err := db.goqu.From(selectQuery.As("query_count")).Select(goqu.COUNT(goqu.L("*"))).Prepared(true).ToSQL()
	if err != nil {
		return 0, err
	}

	result := db.Raw(sql, args...).Count(&count)
	if result.Error != nil {
		return 0, err
	}
	return count, nil
}

// Query ...
func (db *DataBase) Query(query *goqu.SelectDataset, scaner *gorm.DB,
	outRows interface{}, selectEx ...interface{}) error {

	selectQuery := query
	if selectEx != nil {
		selectQuery = query.Select(selectEx...)
	}

	sql, args, err := selectQuery.Prepared(true).ToSQL()
	if err != nil {
		return err
	}

	// use gorm to scan rows
	err = scaner.Raw(sql, args...).Find(outRows).Error
	if err != nil {
		return err
	}

	return nil
}

// QueryAll ...
func (db *DataBase) QueryAll(query *goqu.SelectDataset,
	outRows interface{}, selectEx ...interface{}) error {
	selectQuery := query
	if selectEx != nil {
		selectQuery = query.Select(selectEx...)
	}

	sql, args, err := selectQuery.Prepared(true).ToSQL()
	if err != nil {
		return err
	}

	// use gorm to scan rows
	err = db.Raw(sql, args...).Scan(outRows).Error
	if err != nil {
		return err
	}

	return nil
}

// QueryPluck ...
func (db *DataBase) QueryPluck(query *goqu.SelectDataset, pluckColumn string, outRows interface{}) error {
	sql, args, err := query.Prepared(true).ToSQL()
	if err != nil {
		return err
	}

	// use gorm to scan rows
	err = db.Raw(sql, args...).Pluck(pluckColumn, outRows).Error
	if err != nil {
		return err
	}

	return nil
}

// QueryFirst ..
func (db *DataBase) QueryFirst(query *goqu.SelectDataset, scaner *gorm.DB,
	outRows interface{}, selectEx ...interface{}) error {

	selectQuery := query

	if selectEx != nil {
		selectQuery = query.Select(selectEx...)
	}

	sql, args, err := selectQuery.Prepared(true).ToSQL()
	if err != nil {
		return err
	}

	// use gorm to scan rows
	err = scaner.Raw(sql, args...).Limit(1).Find(outRows).Error
	if err != nil {
		return err
	}

	return nil
}

// QueryRows ..
func (db *DataBase) QueryRows(sqlBuilder *goqu.SelectDataset, scaner *gorm.DB,
	selectEx ...interface{}) (*sql.Rows, error) {
	selectQuery := sqlBuilder
	if selectEx != nil {
		selectQuery = selectQuery.Select(selectEx...)
	}
	sql, args, err := selectQuery.Prepared(true).ToSQL()
	if err != nil {
		return nil, err
	}
	return scaner.Raw(sql, args...).Rows()
}

// DebugSQL ...
func DebugSQL(sqlBuilder *goqu.SelectDataset) string {
	sql, args, err := sqlBuilder.ToSQL()
	return fmt.Sprint("Sql:", sql, args, err)
}

// StringOutRows ...
func StringOutRows(rows []string, pri ...string) (outs []interface{}) {
	var goquout []exp.AliasedExpression
	for _, row := range rows {
		if len(pri) != 0 {
			goquout = append(goquout, goqu.I(pri[0]+"."+row).As(row))
		} else {
			goquout = append(goquout, goqu.I(row).As(row))
		}
	}

	for _, row := range goquout {
		outs = append(outs, row)
	}

	return
}

// GetRows only support default gorm column name
func GetRows(data interface{}, omit ...string) (outs []string) {
	fields := structs.Fields(data)
	for _, f := range fields {
		// TODO only gorm type
		if f.Tag("gorm") == "-" || f.Kind() == reflect.Slice {
			continue
		}
		gname := gorm.ToColumnName(f.Name())
		var skip bool
		for _, o := range omit {
			if o == gname {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		outs = append(outs, "`"+gname+"`")
	}

	return
}

// PrepareInsertSQL only support default gorm column name
func PrepareInsertSQL(table string, data interface{}, omit ...string) (result string) {
	columns := GetRows(data, omit...)
	var marks []string
	for i := 0; i < len(columns); i++ {
		marks = append(marks, "?")
	}
	result = fmt.Sprintf("INSERT INTO `%s` (%s) VALUES(%s)", table, strings.Join(columns, ","), strings.Join(marks, ","))
	return
}
