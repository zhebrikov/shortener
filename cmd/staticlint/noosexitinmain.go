package main

import (
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var noosexitinmainAnalyzer = &analysis.Analyzer{
	Name: "noosexitinmain",
	Doc:  "reports os.Exit, log.Fatal and panic calls outside main.main in package main",
	Run:  runNoOSExitInMain,
}

func runNoOSExitInMain(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Name() != "main" {
		return nil, nil
	}
	workspaceRoot, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	workspaceRoot, err = filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, err
	}

	for _, file := range pass.Files {
		fileName := pass.Fset.Position(file.Pos()).Filename
		absFileName, err := filepath.Abs(fileName)
		if err != nil || !strings.HasPrefix(absFileName, workspaceRoot+string(os.PathSeparator)) {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil || fn.Name.Name == "main" {
				continue
			}

			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "panic" {
					pass.Reportf(call.Pos(), "panic must not be called outside main.main")
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkgIdent, ok := selector.X.(*ast.Ident)
				if !ok {
					return true
				}
				obj := pass.TypesInfo.Uses[pkgIdent]
				pkgName, ok := obj.(*types.PkgName)
				if !ok {
					return true
				}
				switch pkgName.Imported().Path() {
				case "os":
					if selector.Sel.Name == "Exit" {
						pass.Reportf(call.Pos(), "os.Exit must not be called outside main.main")
					}
				case "log":
					if strings.HasPrefix(selector.Sel.Name, "Fatal") {
						pass.Reportf(call.Pos(), "log.Fatal must not be called outside main.main")
					}
				}
				return true
			})
		}
	}

	return nil, nil
}
