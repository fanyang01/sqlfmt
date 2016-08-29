package sqlfmt

import (
	"bytes"
	"fmt"
	"log"
	"regexp"
	"strings"
	"text/template"

	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/mysql"
	"github.com/pingcap/tidb/parser"
)

var (
	reList = regexp.MustCompile(`\(((\d+(, ?\d+)*)|('[^']*'(, ?'[^']*')*))\)`)
)

func fmtType(s string) string {
	list := reList.FindString(s)
	if list == "" {
		return s
	}
	list = strings.Trim(list, "()")
	nums := strings.Split(list, ",")
	for i := 0; i < len(nums); i++ {
		nums[i] = strings.TrimSpace(nums[i])
	}
	list = "(" + strings.Join(nums, ", ") + ")"
	return reList.ReplaceAllString(s, list)
}

func Parse(sql string) *Table {
	parser := parser.New()
	node, err := parser.ParseOneStmt(sql, "", "")
	if err != nil {
		log.Fatal(err)
	}
	switch stmt := node.(type) {
	case *ast.CreateTableStmt:
		table := conv(stmt)
		table.SQL = sql
		return table
	// case *ast.AlterTableType:
	default:
		log.Fatal("unsupported SQL statement")
	}
	return nil
}

func Format(sql string) string {
	table := Parse(sql)

	fmap := template.FuncMap{}
	fmap["add"] = func(x, y int) int { return x + y }
	fmap["upper"] = strings.ToUpper
	fmap["fmtType"] = fmtType
	fmap["nullable"] = func(notNull *bool) string {
		if notNull == nil {
			return ""
		} else if *notNull {
			return "NOT NULL"
		}
		return "NULL"
	}

	tmpl := template.New("root").Funcs(fmap)
	template.Must(tmpl.New("begin").Parse(beginTmpl))
	template.Must(tmpl.New("col-def").Parse(colDefTmpl))
	template.Must(tmpl.New("idx-def").Parse(idxDefTmpl))
	template.Must(tmpl.New("ref-def").Parse(refDefTmpl))
	template.Must(tmpl.New("end").Parse(endTmpl))

	var (
		buf    = new(bytes.Buffer)
		result = make(map[string]string, 5)
	)
	for _, name := range []string{
		"begin", "col-def", "idx-def", "ref-def", "end",
	} {
		buf.Reset()
		if err := tmpl.ExecuteTemplate(buf, name, table); err != nil {
			log.Fatalf("\n%v\n", err)
		}
		result[name] = strings.TrimSpace(buf.String())
	}

	return pretty(result)
}

type Table struct {
	model.TableInfo
	Schema       model.CIStr
	Columns      []*Column
	ForeignKeys  []*ForeignKey
	Indices      []*Index
	Engine       string
	AvgRowLength uint64
	KeyBlockSize uint64
	MinRows      uint64
	MaxRows      uint64
	RowFormat    string
	SQL          string
}

type Column struct {
	model.ColumnInfo
	Ordinal       int
	NotNull       *bool
	PrimaryKey    bool
	AutoIncrement bool
	Unique        bool
	Attribute     string
	DefaultValue  string
	OnUpdate      string
}

type ForeignKey struct {
	model.FKInfo
	RefSchema model.CIStr
	OnUpdate  string
	OnDelete  string
}

type Index struct {
	model.IndexInfo
	Fulltext bool
}

type AlterTable struct {
}

func conv(stmt *ast.CreateTableStmt) *Table {
	t := new(Table)
	t.Schema = stmt.Table.Schema
	t.Name = stmt.Table.Name

	for _, col := range stmt.Cols {
		ci := new(Column)
		ci.Name = col.Name.Name
		ci.FieldType = *col.Tp
		for _, opt := range col.Options {
			switch opt.Tp {
			case ast.ColumnOptionNoOption:
			case ast.ColumnOptionPrimaryKey:
				ci.PrimaryKey = true
			case ast.ColumnOptionNotNull:
				ci.NotNull = new(bool)
				*ci.NotNull = true
			case ast.ColumnOptionAutoIncrement:
				ci.AutoIncrement = true
			case ast.ColumnOptionDefaultValue:
				// log.Printf("DefaultValue: %v\n", opt.Expr.GetValue())
				if v := opt.Expr.GetValue(); v != nil {
					ci.DefaultValue = fmt.Sprintf("%v", v)
				} else if expr, ok := opt.Expr.(*ast.FuncCallExpr); ok {
					ci.DefaultValue = expr.FnName.O
				}
			case ast.ColumnOptionOnUpdate: // For Timestamp and Datetime only.
				if v := opt.Expr.GetValue(); v != nil {
					ci.OnUpdate = fmt.Sprintf("%v", v)
				} else if expr, ok := opt.Expr.(*ast.FuncCallExpr); ok {
					ci.OnUpdate = expr.FnName.O
				}
			case ast.ColumnOptionUniq, ast.ColumnOptionUniqIndex, ast.ColumnOptionUniqKey:
				ci.Unique = true
			case ast.ColumnOptionIndex:
			case ast.ColumnOptionKey:
			case ast.ColumnOptionNull:
				ci.NotNull = new(bool)
				*ci.NotNull = false
			case ast.ColumnOptionFulltext:
				//
			case ast.ColumnOptionComment:
				ci.Comment = opt.Expr.Text()
			}
		}

		var attrs []string
		if mysql.HasUnsignedFlag(col.Tp.Flag) {
			attrs = append(attrs, "UNSIGNED")
		}
		if mysql.HasZerofillFlag(col.Tp.Flag) {
			attrs = append(attrs, "ZEROFILL")
		}
		if mysql.HasBinaryFlag(col.Tp.Flag) {
			attrs = append(attrs, "BINARY")
		}
		ci.Attribute = strings.Join(attrs, " ")

		t.Columns = append(t.Columns, ci)
	}

	// Primary key or unique key as an index
	for _, col := range t.Columns {
		if col.PrimaryKey {
			idx := new(Index)
			// idx.Name.O = "PRIMARY"
			idx.Primary = true
			idx.Columns = append(idx.Columns, &model.IndexColumn{
				Name:   col.Name,
				Length: -1,
			})
			t.Indices = append(t.Indices, idx)
		}
	}
	for _, col := range t.Columns {
		if col.Unique {
			idx := new(Index)
			idx.Unique = true
			idx.Columns = append(idx.Columns, &model.IndexColumn{
				Name:   col.Name,
				Length: -1,
			})
			t.Indices = append(t.Indices, idx)
		}
	}

	for _, cst := range stmt.Constraints {
		switch cst.Tp {
		case ast.ConstraintNoConstraint:
		case ast.ConstraintPrimaryKey,
			ast.ConstraintKey, ast.ConstraintIndex,
			ast.ConstraintUniq, ast.ConstraintUniqKey, ast.ConstraintUniqIndex,
			ast.ConstraintFulltext:

			idx := new(Index)
			idx.Name.O = cst.Name
			idx.Name.L = strings.ToLower(cst.Name)
			idx.Primary = (cst.Tp == ast.ConstraintPrimaryKey)
			idx.Unique = (cst.Tp == ast.ConstraintUniq || cst.Tp == ast.ConstraintUniqIndex || cst.Tp == ast.ConstraintUniqKey)
			idx.Fulltext = (cst.Tp == ast.ConstraintFulltext)
			for _, icol := range cst.Keys {
				idx.Columns = append(idx.Columns, &model.IndexColumn{
					Name:   icol.Column.Name,
					Length: icol.Length,
				})
			}
			t.Indices = append(t.Indices, idx)
		case ast.ConstraintForeignKey:
			ref := cst.Refer
			fk := new(ForeignKey)
			fk.Name.O = cst.Name
			fk.Name.L = strings.ToLower(cst.Name)
			for _, icol := range cst.Keys {
				fk.Cols = append(fk.Cols, icol.Column.Name)
			}
			fk.RefSchema = ref.Table.Schema
			fk.RefTable = ref.Table.Name
			for _, icol := range ref.IndexColNames {
				fk.RefCols = append(fk.RefCols, icol.Column.Name)
			}
			if ref.OnUpdate != nil {
				switch ref.OnUpdate.ReferOpt {
				case ast.ReferOptionNoAction:
					fk.OnUpdate = "NO ACTION"
				case ast.ReferOptionSetNull:
					fk.OnUpdate = "SET NULL"
				case ast.ReferOptionRestrict:
					fk.OnUpdate = "RESTRICT"
				case ast.ReferOptionCascade:
					fk.OnUpdate = "CASCADE"
				}
			}
			if ref.OnDelete != nil {
				switch ref.OnDelete.ReferOpt {
				case ast.ReferOptionNoAction:
					fk.OnDelete = "NO ACTION"
				case ast.ReferOptionSetNull:
					fk.OnDelete = "SET NULL"
				case ast.ReferOptionRestrict:
					fk.OnDelete = "RESTRICT"
				case ast.ReferOptionCascade:
					fk.OnDelete = "CASCADE"
				}
			}
			t.ForeignKeys = append(t.ForeignKeys, fk)
		}
	}

	// Mark column(s) in the primary key
	for _, idx := range t.Indices {
		if !idx.Primary {
			continue
		}
		for _, icol := range idx.Columns {
			for _, col := range t.Columns {
				if icol.Name.String() == col.Name.String() {
					col.PrimaryKey = true
				}
			}
		}
	}

	for i := range t.Columns {
		t.Columns[i].Ordinal = i + 1
	}

	for _, opt := range stmt.Options {
		switch opt.Tp {
		case ast.TableOptionNone:
		case ast.TableOptionEngine:
			t.Engine = opt.StrValue
		case ast.TableOptionCharset:
			t.Charset = opt.StrValue
		case ast.TableOptionCollate:
			t.Collate = opt.StrValue
		case ast.TableOptionAutoIncrement:
			t.AutoIncID = int64(opt.UintValue)
		case ast.TableOptionComment:
			t.Comment = opt.StrValue
		case ast.TableOptionAvgRowLength:
			t.AvgRowLength = opt.UintValue
		case ast.TableOptionCheckSum:
		case ast.TableOptionCompression:
		case ast.TableOptionConnection:
		case ast.TableOptionPassword:
		case ast.TableOptionKeyBlockSize:
			t.KeyBlockSize = opt.UintValue
		case ast.TableOptionMaxRows:
			t.MaxRows = opt.UintValue
		case ast.TableOptionMinRows:
			t.MinRows = opt.UintValue
		case ast.TableOptionDelayKeyWrite:
		case ast.TableOptionRowFormat:
			switch opt.UintValue {
			case ast.RowFormatDefault:
				t.RowFormat = "DEFAULT"
			case ast.RowFormatDynamic:
				t.RowFormat = "DYNAMIC"
			case ast.RowFormatFixed:
				t.RowFormat = "FIXED"
			case ast.RowFormatCompressed:
				t.RowFormat = "COMPRESSED"
			case ast.RowFormatRedundant:
				t.RowFormat = "REDUNDANT"
			case ast.RowFormatCompact:
				t.RowFormat = "COMPACT"
			}
		case ast.TableOptionStatsPersistent:
		}
	}

	return t
}
