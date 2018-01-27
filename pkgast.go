package pkgast

import (
	"bytes"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

func ImportPackage(importPath string) (*ast.Package, error) {
	for _, base := range getSearchPath() {
		pkgPath := filepath.Join(base, filepath.FromSlash(importPath))

		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, pkgPath, func(info os.FileInfo) bool {
			return !strings.HasSuffix(info.Name(), "_test.go")
		}, 0)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, errors.Wrapf(err, "pkgast: broken package %q", pkgPath)
		}
		if len(pkgs) != 1 {
			return nil, errors.Errorf("pkgast: ambiguous packages, found %d packages", len(pkgs))
		}
		for _, pkg := range pkgs {
			return pkg, nil
		}
	}
	return nil, errors.Errorf("pkgast: package not found %q", importPath)
}

func ScanDecl(pkg *ast.Package, f func(ast.Decl) bool) {
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			if !f(decl) {
				return
			}
		}
	}
}

func ExportType(pkg *ast.Package, expr ast.Expr) ast.Expr {
	switch expr := expr.(type) {
	case *ast.Ident:
		if !expr.IsExported() {
			return expr
		}
		return &ast.SelectorExpr{Sel: expr, X: ast.NewIdent(pkg.Name)}
	case *ast.StarExpr:
		return &ast.StarExpr{X: ExportType(pkg, expr.X)}
	case *ast.ArrayType:
		return &ast.ArrayType{Elt: ExportType(pkg, expr.Elt), Len: expr.Len}
	case *ast.MapType:
		return &ast.MapType{Key: ExportType(pkg, expr.Key), Value: ExportType(pkg, expr.Value)}
	case *ast.ChanType:
		return &ast.ChanType{Begin: expr.Begin, Arrow: expr.Arrow, Dir: expr.Dir, Value: ExportType(pkg, expr.Value)}
	case *ast.FuncType:
		fn := *expr
		fn.Params = ExportField(pkg, fn.Params)
		fn.Results = ExportField(pkg, fn.Results)
		return &fn
	case *ast.Ellipsis:
		return &ast.Ellipsis{Ellipsis: expr.Ellipsis, Elt: ExportType(pkg, expr.Elt)}
	default:
		return expr
	}
}

func ExportField(pkg *ast.Package, fields *ast.FieldList) *ast.FieldList {
	if fields == nil {
		return nil
	}
	efields := *fields
	for i, field := range efields.List {
		efields.List[i].Type = ExportType(pkg, field.Type)
	}
	return &efields
}

func ExportFunc(pkg *ast.Package, fn *ast.FuncDecl) *ast.FuncDecl {
	efn := *fn
	efn.Recv = nil
	efn.Type.Params = ExportField(pkg, efn.Type.Params)
	efn.Type.Results = ExportField(pkg, efn.Type.Results)
	return &efn
}

func GetMethods(pkg *ast.Package, name string) []*ast.FuncDecl {
	var methods []*ast.FuncDecl
	ScanDecl(pkg, func(decl ast.Decl) bool {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			return true
		}
		if funcDecl.Recv == nil {
			return true
		}
		rt := funcDecl.Recv.List[0]
		if TypeName(rt.Type) == name && funcDecl.Name.IsExported() {
			methods = append(methods, funcDecl)
		}
		return true
	})
	return methods
}

func FindTypeByName(pkg *ast.Package, name string) ast.Expr {
	var t ast.Expr
	ScanDecl(pkg, func(decl ast.Decl) bool {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			return true
		}
		if genDecl.Tok != token.TYPE {
			return true
		}
		for _, spec := range genDecl.Specs {
			typeSpec := spec.(*ast.TypeSpec)
			if typeSpec.Name.Name == name {
				t = typeSpec.Type
				return false
			}
		}
		return true
	})
	return t
}

func FindInterface(pkg *ast.Package, name string) *ast.InterfaceType {
	it, ok := FindTypeByName(pkg, name).(*ast.InterfaceType)
	if !ok {
		return nil
	}
	return it
}

func FindStruct(pkg *ast.Package, name string) *ast.StructType {
	st, ok := FindTypeByName(pkg, name).(*ast.StructType)
	if !ok {
		return nil
	}
	return st
}

func getSearchPath() []string {
	var searchPath []string
	if wd, err := os.Getwd(); err == nil {
		searchPath = append(searchPath, filepath.Join(wd, "vendor"))
	}
	for _, gopath := range filepath.SplitList(build.Default.GOPATH) {
		searchPath = append(searchPath, filepath.Join(gopath, "src"))
	}
	searchPath = append(searchPath, filepath.Join(build.Default.GOROOT, "src"))
	return searchPath
}

func TypeName(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	var b bytes.Buffer
	if err := printer.Fprint(&b, token.NewFileSet(), expr); err != nil {
		panic(err)
	}
	return b.String()
}
