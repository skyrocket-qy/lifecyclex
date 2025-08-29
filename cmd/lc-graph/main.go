package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: lc-graph <path>")
	}

	path := os.Args[1]

	if _, err := os.Stdout.WriteString("digraph shutdown_graph {\n"); err != nil {
		log.Fatal(err)
	}

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".go" {
			parseFile(path)
		}

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	if _, err := os.Stdout.WriteString("}\n"); err != nil {
		log.Fatal(err)
	}
}

func parseFile(filename string) {
	fset := token.NewFileSet()

	node, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		log.Println(err)

		return
	}

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Add" {
			return true
		}

		if len(call.Args) <= 1 {
			return true
		}

		appName := getAppName(call.Args[0])
		if appName == "" {
			return true
		}

		for _, dep := range call.Args[2:] {
			depName := getAppName(dep)
			if depName != "" {
				if _, err := os.Stdout.WriteString(fmt.Sprintf(`  "%s" -> "%s";`, appName, depName) + "\n"); err != nil {
					log.Fatal(err)
				}
			}
		}

		return true
	})
}

func getAppName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.BasicLit:
		return v.Value
	case *ast.Ident:
		return v.Name
	}

	return ""
}
