package main

import (
	"flag"
	"go/ast"
	"go/printer"
	"go/token"
	"log"
	"os"

	"github.com/orisano/impast"
)

func main() {
	pkgPath := flag.String("pkg", "", "package path")
	interfaceName := flag.String("type", "", "interface type")
	flag.Parse()

	pkg, err := impast.ImportPackage(*pkgPath)
	if err != nil {
		log.Fatal(err)
	}

	it := impast.FindInterface(pkg, *interfaceName)
	if it == nil {
		log.Fatalf("interface not found %q", *interfaceName)
	}

	mockName := ast.NewIdent(*interfaceName + "Mock")
	st := &ast.StructType{Fields: &ast.FieldList{}}
	methods := impast.GetRequires(it)
	for i := range methods {
		methods[i].Type = impast.ExportType(pkg, methods[i].Type)
	}
	for _, method := range methods {
		st.Fields.List = append(st.Fields.List, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(method.Names[0].Name + "Mock")},
			Type:  method.Type,
		})
	}
	genDecl := &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{&ast.TypeSpec{
			Type: st,
			Name: mockName,
		}},
	}
	printer.Fprint(os.Stdout, token.NewFileSet(), genDecl)
	os.Stdout.WriteString("\n\n")

	recvName := ast.NewIdent("mo")

	for _, method := range methods {
		funcDecl := genMockFuncDecl(mockName, recvName, method)
		printer.Fprint(os.Stdout, token.NewFileSet(), funcDecl)
		os.Stdout.WriteString("\n\n")
	}
}

func genMockFuncDecl(mock, recv *ast.Ident, method *ast.Field) *ast.FuncDecl {
	ft := impast.AutoNaming(method.Type.(*ast.FuncType))
	expr := &ast.CallExpr{
		Fun:  &ast.SelectorExpr{X: recv, Sel: ast.NewIdent(method.Names[0].Name + "Mock")},
		Args: flattenName(ft.Params),
	}
	if len(ft.Params.List) >= 1 {
		if _, variadic := ft.Params.List[len(ft.Params.List)-1].Type.(*ast.Ellipsis); variadic {
			expr.Ellipsis = token.Pos(1)
		}
	}
	var stmt ast.Stmt
	if ft.Results == nil {
		stmt = &ast.ExprStmt{X: expr}
	} else {
		stmt = &ast.ReturnStmt{Results: []ast.Expr{expr}}
	}

	funcDecl := &ast.FuncDecl{
		Name: method.Names[0],
		Recv: &ast.FieldList{List: []*ast.Field{
			{
				Names: []*ast.Ident{recv},
				Type:  &ast.StarExpr{X: mock},
			},
		}},
		Type: ft,
		Body: &ast.BlockStmt{List: []ast.Stmt{stmt}},
	}
	return funcDecl
}

func flattenName(fields *ast.FieldList) []ast.Expr {
	var names []ast.Expr
	for _, field := range fields.List {
		for _, name := range field.Names {
			names = append(names, name)
		}
	}
	return names
}
