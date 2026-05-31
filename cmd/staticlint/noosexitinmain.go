package main

import (
	"go/ast"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var noosexitinmainAnalyzer = &analysis.Analyzer{
	Name: "noosexitinmain",
	Doc:  "reports direct os.Exit calls in main.main",
	Run:  runNoOSExitInMain,
}

func runNoOSExitInMain(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Name() != "main" {
		return nil, nil
	}
	if pass.Pkg.Path() != "github.com/zhebrikov/shortener" &&
		!strings.HasPrefix(pass.Pkg.Path(), "github.com/zhebrikov/shortener/") {
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
			if !ok || fn.Recv != nil || fn.Name.Name != "main" || fn.Body == nil {
				continue
			}

			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
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
				if pkgIdent.Name == "os" && selector.Sel.Name == "Exit" {
					pass.Reportf(call.Pos(), "direct os.Exit call is forbidden in main.main")
				}
				return true
			})
		}
	}

	return nil, nil
}
