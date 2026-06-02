package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

const generateResetComment = "generate:reset"

func main() {
	root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedModule,
		Dir:  root,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		log.Fatal(err)
	}

	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			continue
		}
		if pkg.PkgPath == "github.com/zhebrikov/shortener/cmd/reset" {
			continue
		}
		if len(pkg.GoFiles) == 0 {
			continue
		}

		structs := findResetableStructs(pkg)
		if len(structs) == 0 {
			continue
		}

		content, err := generateFile(pkg, structs)
		if err != nil {
			log.Fatalf("generate %s: %v", pkg.PkgPath, err)
		}

		outPath := filepath.Join(filepath.Dir(pkg.GoFiles[0]), "reset.gen.go")
		if err := os.WriteFile(outPath, content, 0o644); err != nil {
			log.Fatalf("write %s: %v", outPath, err)
		}
		log.Printf("generated %s", outPath)
	}
}

type resetableStruct struct {
	name *types.Named
}

func findResetableStructs(pkg *packages.Package) []resetableStruct {
	var result []resetableStruct

	for _, file := range pkg.Syntax {
		if strings.HasSuffix(pkg.Fset.Position(file.Pos()).Filename, "reset.gen.go") {
			continue
		}

		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}

			for i, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := typeSpec.Type.(*ast.StructType); !ok {
					continue
				}
				if !hasGenerateResetComment(genDecl, typeSpec, i) {
					continue
				}

				obj := pkg.Types.Scope().Lookup(typeSpec.Name.Name)
				if obj == nil {
					continue
				}
				named, ok := obj.Type().(*types.Named)
				if !ok {
					continue
				}

				result = append(result, resetableStruct{name: named})
			}
		}
	}

	return result
}

func hasGenerateResetComment(genDecl *ast.GenDecl, typeSpec *ast.TypeSpec, specIndex int) bool {
	if commentSaysGenerateReset(typeSpec.Doc) || commentSaysGenerateReset(typeSpec.Comment) {
		return true
	}
	if specIndex == 0 && commentSaysGenerateReset(genDecl.Doc) {
		return true
	}
	return false
}

func commentSaysGenerateReset(comment *ast.CommentGroup) bool {
	if comment == nil {
		return false
	}
	for _, c := range comment.List {
		text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if text == generateResetComment {
			return true
		}
	}
	return false
}

type fileData struct {
	Package string
	Methods []methodData
}

type methodData struct {
	Receiver string
	TypeName string
	Body     string
}

type stmtData struct {
	Indent int
	Access string
	Value  string
}

func generateFile(pkg *packages.Package, structs []resetableStruct) ([]byte, error) {
	resetableNames := make(map[string]struct{}, len(structs))
	for _, s := range structs {
		resetableNames[s.name.Obj().Name()] = struct{}{}
	}

	methods := make([]methodData, 0, len(structs))
	for _, s := range structs {
		method, err := buildResetMethod(pkg.Types, s.name, resetableNames)
		if err != nil {
			return nil, err
		}
		methods = append(methods, method)
	}

	var buf bytes.Buffer
	if err := codeTemplates.ExecuteTemplate(&buf, "file", fileData{
		Package: pkg.Name,
		Methods: methods,
	}); err != nil {
		return nil, err
	}

	return format.Source(buf.Bytes())
}

func buildResetMethod(pkg *types.Package, named *types.Named, resetableNames map[string]struct{}) (methodData, error) {
	structType, ok := named.Underlying().(*types.Struct)
	if !ok {
		return methodData{}, fmt.Errorf("%s is not a struct", named.Obj().Name())
	}

	recv := strings.ToLower(named.Obj().Name()[:1])
	var body bytes.Buffer

	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)
		access := fmt.Sprintf("%s.%s", recv, field.Name())
		if err := emitFieldReset(&body, pkg, access, field.Type(), 1, resetableNames); err != nil {
			return methodData{}, err
		}
	}

	return methodData{
		Receiver: recv,
		TypeName: named.Obj().Name(),
		Body:     body.String(),
	}, nil
}

func emitTemplate(buf *bytes.Buffer, name string, data any) error {
	return codeTemplates.ExecuteTemplate(buf, name, data)
}

func emitFieldReset(buf *bytes.Buffer, pkg *types.Package, access string, typ types.Type, indent int, resetableNames map[string]struct{}) error {
	if ptr, ok := typ.Underlying().(*types.Pointer); ok {
		return emitPointerReset(buf, pkg, access, ptr.Elem(), indent, resetableNames)
	}

	data := stmtData{Indent: indent, Access: access}

	if hasResetMethod(typ) || isMarkedResetable(typ, resetableNames) {
		return emitTemplate(buf, "stmt.reset", data)
	}

	switch t := typ.Underlying().(type) {
	case *types.Basic:
		data.Value = zeroLiteral(t)
		return emitTemplate(buf, "stmt.assign", data)
	case *types.Slice:
		return emitTemplate(buf, "stmt.slice_clear", data)
	case *types.Array:
		data.Value = typeString(typ, pkg) + "{}"
		return emitTemplate(buf, "stmt.assign", data)
	case *types.Map:
		return emitTemplate(buf, "stmt.clear", data)
	case *types.Struct:
		data.Value = typeString(typ, pkg) + "{}"
		return emitTemplate(buf, "stmt.assign", data)
	case *types.Interface, *types.Chan:
		data.Value = "nil"
		return emitTemplate(buf, "stmt.assign", data)
	default:
		data.Value = typeString(typ, pkg) + "{}"
		return emitTemplate(buf, "stmt.assign", data)
	}
}

func emitPointerReset(buf *bytes.Buffer, pkg *types.Package, access string, elem types.Type, indent int, resetableNames map[string]struct{}) error {
	data := stmtData{Indent: indent, Access: access}

	if hasResetMethod(elem) || isMarkedResetable(elem, resetableNames) {
		return emitTemplate(buf, "stmt.ptr_resetter", data)
	}

	switch t := elem.Underlying().(type) {
	case *types.Basic:
		data.Value = zeroLiteral(t)
		return emitTemplate(buf, "stmt.ptr_assign", data)
	case *types.Slice:
		return emitTemplate(buf, "stmt.ptr_slice_clear", data)
	case *types.Array:
		data.Value = typeString(elem, pkg) + "{}"
		return emitTemplate(buf, "stmt.ptr_assign", data)
	case *types.Map:
		return emitTemplate(buf, "stmt.ptr_clear", data)
	case *types.Struct:
		data.Value = typeString(elem, pkg) + "{}"
		return emitTemplate(buf, "stmt.ptr_assign", data)
	case *types.Interface, *types.Chan:
		data.Value = "nil"
		return emitTemplate(buf, "stmt.ptr_assign", data)
	default:
		data.Value = typeString(elem, pkg) + "{}"
		return emitTemplate(buf, "stmt.ptr_assign", data)
	}
}

func hasResetMethod(typ types.Type) bool {
	if methodNamed(typ, "Reset") {
		return true
	}
	return methodNamed(types.NewPointer(typ), "Reset")
}

func isMarkedResetable(typ types.Type, resetableNames map[string]struct{}) bool {
	if name, ok := namedTypeName(typ); ok {
		_, ok := resetableNames[name]
		return ok
	}
	return false
}

func namedTypeName(typ types.Type) (string, bool) {
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}
	named, ok := typ.(*types.Named)
	if !ok {
		return "", false
	}
	return named.Obj().Name(), true
}

func methodNamed(typ types.Type, name string) bool {
	mset := types.NewMethodSet(typ)
	for i := 0; i < mset.Len(); i++ {
		if mset.At(i).Obj().Name() == name {
			sig, ok := mset.At(i).Obj().Type().(*types.Signature)
			if !ok {
				continue
			}
			if sig.Params().Len() == 0 && sig.Results().Len() == 0 {
				return true
			}
		}
	}
	return false
}

func zeroLiteral(b *types.Basic) string {
	switch b.Kind() {
	case types.Bool:
		return "false"
	case types.String:
		return `""`
	case types.UnsafePointer:
		return "nil"
	default:
		return "0"
	}
}

func typeString(typ types.Type, pkg *types.Package) string {
	return types.TypeString(typ, func(p *types.Package) string {
		if p == pkg {
			return ""
		}
		return p.Name()
	})
}
