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

	fmt.Println("digraph shutdown_graph {")
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
	fmt.Println("}")
}

func parseFile(filename string) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		log.Println(err)
		return
	}

	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if sel.Sel.Name == "Add" {
					// This is a simplistic check. A real implementation would need to
					// track the type of the receiver of the 'Add' call to ensure
					// it's a *LifecycleParallel.
					if len(call.Args) > 1 {
						appName := getAppName(call.Args[0])
						if appName != "" {
							for _, dep := range call.Args[2:] {
								depName := getAppName(dep)
								if depName != "" {
									fmt.Printf(`  "%s" -> "%s";`, appName, depName)
								}
							}
						}
					}
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
