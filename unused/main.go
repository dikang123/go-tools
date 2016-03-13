package main

import (
	"flag"
	"fmt"
	"go/token"
	"go/types"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/kisielk/gotool"
	"golang.org/x/tools/go/loader"
)

var exitCode int

var (
	fConstants bool
	fFunctions bool
	fTypes     bool
	fVariables bool
)

func init() {
	flag.BoolVar(&fConstants, "c", true, "Report unused constants")
	flag.BoolVar(&fFunctions, "f", true, "Report unused functions and methods")
	flag.BoolVar(&fTypes, "t", true, "Report unused types")
	flag.BoolVar(&fVariables, "v", true, "Report unused variables")
}

func main() {
	flag.Parse()
	// FIXME check flag.NArgs
	paths := gotool.ImportPaths([]string{flag.Arg(0)})
	conf := loader.Config{AllowErrors: true}
	for _, path := range paths {
		conf.ImportWithTests(path)
	}
	lprog, err := conf.Load()
	if err != nil {
		log.Fatal(err)
	}

	defs := map[types.Object]bool{}
	for _, path := range paths {
		pkg := lprog.Package(path)
		if pkg == nil {
			log.Println("Couldn't load package", path)
			continue
		}
		for _, obj := range pkg.Defs {
			if obj == nil {
				continue
			}
			if obj, ok := obj.(*types.Var); ok &&
				pkg.Pkg.Scope() != obj.Parent() && !obj.IsField() {
				// Skip variables that aren't package variables or struct fields
				continue
			}
			defs[obj] = false
		}
		for _, obj := range pkg.Uses {
			defs[obj] = true
		}
	}
	var reports Reports
	for obj, used := range defs {
		// TODO methods that satisfy an interface are used
		// TODO methods + reflection
		// TODO exported constants in function bodies need to be used
		if !checkFlags(obj) {
			continue
		}
		if used || obj.Name() == "_" {
			continue
		}
		if obj.Exported() {
			f := lprog.Fset.Position(obj.Pos()).Filename
			if !strings.HasSuffix(f, "_test.go") || strings.HasPrefix(obj.Name(), "Test") || strings.HasPrefix(obj.Name(), "Benchmark") {
				continue
			}
		}
		if obj.Pkg().Name() == "main" && obj.Name() == "main" {
			continue
		}
		if obj, ok := obj.(*types.Func); ok && obj.Name() == "init" {
			sig := obj.Type().(*types.Signature)
			if sig.Recv() == nil {
				continue
			}
		}
		reports = append(reports, Report{obj.Pos(), obj.Name()})
	}
	sort.Sort(reports)
	for _, report := range reports {
		fmt.Printf("%s: %s is unused\n", lprog.Fset.Position(report.pos), report.name)
	}

	os.Exit(exitCode)
}

func checkFlags(obj types.Object) bool {
	if _, ok := obj.(*types.Func); ok && !fFunctions {
		return false
	}
	if _, ok := obj.(*types.Var); ok && !fVariables {
		return false
	}
	if _, ok := obj.(*types.Const); ok && !fConstants {
		return false
	}
	if _, ok := obj.(*types.TypeName); ok && !fTypes {
		return false
	}
	return true
}

type Report struct {
	pos  token.Pos
	name string
}
type Reports []Report

func (l Reports) Len() int           { return len(l) }
func (l Reports) Less(i, j int) bool { return l[i].pos < l[j].pos }
func (l Reports) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
