package sqlfmt

import (
	"bytes"
	"log"
	"regexp"
	"strings"
	"text/template"

	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/model"
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

func Format(sql string) string {
	parser := parser.New()
	node, err := parser.ParseOneStmt(sql, "", "")
	if err != nil {
		log.Fatal(err)
	} else if _, ok := node.(*ast.CreateTableStmt); !ok {
		log.Fatal("unsupported SQL statement")
	}

	stmt := node.(*ast.CreateTableStmt)
	table := conv(stmt)

	fmap := template.FuncMap{}
	fmap["add"] = func(x, y int) int { return x + y }
	fmap["upper"] = strings.ToUpper
	fmap["fmtType"] = fmtType

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

type tableInfo struct {
	model.TableInfo
	Schema       model.CIStr
	Columns      []*columnInfo
	ForeignKeys  []*foreignKeyInfo
	Indices      []*indexInfo
	Engine       string
	AvgRowLength uint64
	KeyBlockSize uint64
	MinRows      uint64
	MaxRows      uint64
	RowFormat    string
}

type columnInfo struct {
	model.ColumnInfo
	NotNull       bool
	PrimaryKey    bool
	AutoIncrement bool
	Unique        bool
}

type foreignKeyInfo struct {
	model.FKInfo
	RefSchema model.CIStr
}

type indexInfo struct {
	model.IndexInfo
	Fulltext bool
}

func conv(stmt *ast.CreateTableStmt) *tableInfo {
	t := new(tableInfo)
	t.Schema = stmt.Table.Schema
	t.Name = stmt.Table.Name

	for _, col := range stmt.Cols {
		ci := new(columnInfo)
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
			idx := new(indexInfo)
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
			idx := new(indexInfo)
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

			idx := new(indexInfo)
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
			fk := new(foreignKeyInfo)
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
