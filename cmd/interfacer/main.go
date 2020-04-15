package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"log"
	"os"
	"strings"

	"github.com/orisano/impast"
	"golang.org/x/tools/imports"
)

func main() {
	interfaceName := flag.String("out", "", "generate interface name (required)")
	pkgName := flag.String("pkg", "", "generate interface package name")

	flag.Parse()

	log.SetFlags(0)
	log.SetPrefix("interfacer: ")

	if *interfaceName == "" {
		log.Print("-out is must be required")
		flag.Usage()
		os.Exit(2)
	}

	impast.DefaultImporter.EnableCache = true

	var m []*ast.FuncDecl
	for _, t := range flag.Args() {
		index := strings.LastIndexByte(t, '.')
		if index == -1 {
			log.Fatalf("invalid type: %v", t)
		}
		pkgPath := t[:index]
		typeName := t[index+1:]

		pkg, err := impast.ImportPackage(pkgPath)
		if err != nil {
			log.Fatalf("failed to import package (%v): %v", pkgPath, err)
		}

		methods, err := impast.GetMethodsDeep(pkg, typeName)
		if err != nil {
			log.Fatalf("failed to get methods %v.%v: %v", pkg.Name, typeName, err)
		}
		m = intersectionMethods(m, methods)
	}

	it := &ast.InterfaceType{
		Methods: &ast.FieldList{},
	}
	for _, method := range m {
		it.Methods.List = append(it.Methods.List, &ast.Field{
			Type:  method.Type,
			Names: []*ast.Ident{method.Name},
		})
	}
	decl := &ast.GenDecl{
		Tok: token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: ast.NewIdent(*interfaceName),
				Type: it,
			},
		},
	}

	if *pkgName != "" {
		var b bytes.Buffer
		fmt.Fprintln(&b, `// Code generated by 'interfacer'; DO NOT EDIT.`)
		fmt.Fprintf(&b, "package %v\n\n", *pkgName)

		for _, p := range impast.DefaultImporter.Loaded() {
			fmt.Fprintf(&b, "import %q\n", p)
		}
		printer.Fprint(&b, token.NewFileSet(), decl)

		src, err := imports.Process("", b.Bytes(), &imports.Options{
			Comments: true,
		})
		if err != nil {
			log.Fatalf("failed to goimports: %v", err)
		}
		os.Stdout.Write(src)
	} else {
		printer.Fprint(os.Stdout, token.NewFileSet(), decl)
		fmt.Println()
	}
}

func intersectionMethods(a, b []*ast.FuncDecl) []*ast.FuncDecl {
	if a == nil {
		return b
	}
	c := a[:0]
	for _, x := range a {
		for len(b) > 0 && less(b[0], x) {
			b = b[1:]
		}
		if len(b) > 0 && equal(x, b[0]) {
			c = append(c, x)
		}
	}
	return c
}

func less(a, b *ast.FuncDecl) bool {
	if a.Name.Name != b.Name.Name {
		return a.Name.Name < b.Name.Name
	}
	return signature(a) < signature(b)
}

func equal(a, b *ast.FuncDecl) bool {
	ok := !less(a, b) && !less(b, a)
	return ok
}

func signature(f *ast.FuncDecl) string {
	args := types(f.Type.Params)
	results := types(f.Type.Results)

	return fmt.Sprintf("(%s)(%s)", strings.Join(args, ","), strings.Join(results, ","))
}

func types(fl *ast.FieldList) []string {
	var ts []string
	if fl == nil {
		return ts
	}
	for _, el := range fl.List {
		t := impast.TypeName(el.Type)
		if len(el.Names) == 0 {
			ts = append(ts, t)
		} else {
			for range el.Names {
				ts = append(ts, t)
			}
		}
	}
	return ts
}
