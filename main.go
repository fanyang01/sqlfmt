package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"text/template"
	"unicode"

	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/parser"
)

var (
	fmap = template.FuncMap{}
)

func init() {
	fmap["add"] = func(x, y int) int { return x + y }
	fmap["sqlType"] = func(s string) string {
		for i, r := range s {
			if !unicode.IsLetter(r) && r != ' ' {
				return s[:i]
			}
		}
		return s
	}
	fmap["nstr"] = func(s string) string {
		if s == "" {
			return "null"
		}
		return fmt.Sprintf("%q", s)
	}
	fmap["upper"] = strings.ToUpper
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	b, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	parser := parser.New()
	node, err := parser.ParseOneStmt(string(b), "", "")
	if err != nil {
		log.Fatal(err)
	} else if _, ok := node.(*ast.CreateTableStmt); !ok {
		log.Fatal("unsupported SQL statement")
	}

	stmt := node.(*ast.CreateTableStmt)
	table := conv(stmt)
	tmpl := template.New("root").Funcs(fmap)
	template.Must(tmpl.New("sql.tmpl").Parse(sqlTemplate))

	buf := new(bytes.Buffer)
	if err = tmpl.ExecuteTemplate(buf, "sql.tmpl", table); err != nil {
		log.Fatalf("\n%v\n", err)
	}

	sql := pretty(buf.String())
	fmt.Printf("\n%s", sql)
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
}

type Column struct {
	model.ColumnInfo
	NotNull       bool
	PrimaryKey    bool
	AutoIncrement bool
	Unique        bool
}

type ForeignKey struct {
	model.FKInfo
	RefSchema model.CIStr
}

type Index struct {
	model.IndexInfo
	Fulltext bool
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
				ci.NotNull = true
			case ast.ColumnOptionAutoIncrement:
				ci.AutoIncrement = true
			case ast.ColumnOptionDefaultValue:
				ci.DefaultValue = opt.Expr.GetValue()
			case ast.ColumnOptionUniq, ast.ColumnOptionUniqIndex, ast.ColumnOptionUniqKey:
				ci.Unique = true
			case ast.ColumnOptionIndex:
			case ast.ColumnOptionKey:
			case ast.ColumnOptionNull:
			case ast.ColumnOptionOnUpdate: // For Timestamp and Datetime only.
			case ast.ColumnOptionFulltext:
				//
			case ast.ColumnOptionComment:
				ci.Comment = opt.Expr.Text()
			}
		}
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
			t.ForeignKeys = append(t.ForeignKeys, fk)
		}
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
			t.RowFormat = opt.StrValue
		case ast.TableOptionStatsPersistent:
		}
	}

	return t
}
