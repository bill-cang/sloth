package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"github.com/fatih/structtag"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"text/template"
)

const funGetter = "get"
const funSetter = "set"
const tagName = "gorm"

var (
	outStruct     = flag.String("out", "", "需要输出的结构体 ; must be set")
	outFun        = flag.String("fun", "", "需要生成的方法(Set|Get); must be set")
	mod           = flag.String("mod", "", "自定义模板(绝对路径); The file name must be sloth_getter.tmp|sloth_setter.tmp; Not required")
	output        = flag.String("output", "", "output file name; default srcdir/<type>_sloth.go")
	columnCompile = regexp.MustCompile("column:([\\w]+);?")
	autoFunc      []string

	customSetter, customGetter string
)

// Usage is a replacement usage function for the flags package.
func Usage() {
	fmt.Fprintf(os.Stderr, "Usage of sloth:\n")
	fmt.Fprintf(os.Stderr, "\tsloth [flags] -out T [structs]\n")
	fmt.Fprintf(os.Stderr, "\tsloth [flags] -fun T [functions] # Must be a single package\n")
	fmt.Fprintf(os.Stderr, "\tsloth [flags] -mod T [dir] # Must be a single package\n")
	fmt.Fprintf(os.Stderr, "For more information, see:\n")
	fmt.Fprintf(os.Stderr, "\thttps://gitee.com/dwdcth/sloth.git\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("sloth: ")
	flag.Usage = Usage
	flag.Parse()

	if len(*outStruct) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	types := strings.Split(*outStruct, ",")
	autoFunc = strings.Split(*outFun, ",")

	// We accept either one directory or a list of files. Which do we have?
	args := flag.Args()
	if len(args) == 0 {
		// Default: process whole package in current directory.
		args = []string{"."}
	}

	// Parse the package once.
	var dir string
	g := Generator{
		buf: make(map[string]*bytes.Buffer),
		//structInfo: make(map[string]StructFieldInfoArr), //一定不能初始化
		walkMark: make(map[string]bool),
	}
	if len(args) == 1 && isDirectory(args[0]) {
		dir = args[0]
	} else {
		dir = filepath.Dir(args[0])
	}

	isCustomTemplate(*mod)
	//ParseStruct(dir, nil, "access")
	g.parsePackage(args)
	// Print the header and package clause.
	// Run generate for each type.
	for i, typeName := range types {
		g.generate(typeName)
		// funSetter to file.
		outputName := *output
		if outputName == "" {
			baseName := fmt.Sprintf("%s_sloth.go", types[i])
			outputName = filepath.Join(dir, strings.ToLower(baseName))
		}
		buf, ok := g.buf[typeName]
		if !ok {
			panic(fmt.Sprintf("generate struct %s failed.", typeName))
		}
		var src = (buf).Bytes()
		err := ioutil.WriteFile(outputName, src, 0644)
		if err != nil {
			log.Fatalf("writing output: %s", err)
		}
	}

}

//use custom template
func isCustomTemplate(mod string) {
	if mod == "" {
		return
	}
	modDir, err := filepath.Abs(mod)
	if err != nil {
		panic(fmt.Sprintf("Abs returns an absolute representation of %s, err : %+v", mod, err))
		return
	}
	modDir = strings.Replace(modDir, "\\", "/", -1)
	tmps := []string{"sloth_getter.tmp", "sloth_setter.tmp"}
	for _, tmp := range tmps {
		tmpAbsAddr := strings.Join([]string{modDir, tmp}, "/")
		redr, err := ioutil.ReadFile(tmpAbsAddr)
		if err != nil {
			panic(fmt.Sprintf("Abs %s read failed, err =%+v.", tmpAbsAddr, err))
			return
		}
		tp := string(redr)
		//log.Printf("[isCustomTemplate] the mod =%s.", tp)
		if strings.Contains(tmp, funGetter) {
			customGetter = tp
		} else {
			customSetter = tp
		}
	}
}

// isDirectory reports whether the named file is a directory.
func isDirectory(name string) bool {
	info, err := os.Stat(name)
	if err != nil {
		log.Fatal(err)
	}
	return info.IsDir()
}

// Generator holds the state of the analysis. Primarily used to buffer
// the output for format.Source.
type Generator struct {
	buf        map[string]*bytes.Buffer // Accumulated output.
	pkg        *Package                 // Package we are scanning.
	structInfo map[string]StructFieldInfoArr
	walkMark   map[string]bool
}

func (g *Generator) Printf(structName, format string, args ...interface{}) {
	buf, ok := g.buf[structName]
	if !ok {
		buf = bytes.NewBufferString("")
		g.buf[structName] = buf
	}
	fmt.Fprintf(buf, format, args...)
}

// File holds a single parsed file and associated data.
type File struct {
	pkg     *Package  // Package to which this file belongs.
	file    *ast.File // Parsed AST.
	fileSet *token.FileSet
	// These fields are reset for each type being generated.
	typeName string // Name of the constant type.
}

type Package struct {
	name  string
	defs  map[*ast.Ident]types.Object
	files []*File
}

// parsePackage analyzes the single package constructed from the patterns and tags.
// parsePackage exits if there is an error.
func (g *Generator) parsePackage(patterns []string) {
	/*	cfg := &packages.Config{
		Mode:  packages.LoadSyntax,
		Tests: false,
	}*/
	cfg := &packages.Config{
		Mode:  packages.LoadAllSyntax,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		log.Fatal(err)
	}
	if len(pkgs) != 1 {
		log.Fatalf("error: %d packages found", len(pkgs))
	}
	g.addPackage(pkgs[0])
}

// addPackage adds a type checked Package and its syntax files to the generator.
func (g *Generator) addPackage(pkg *packages.Package) {
	g.pkg = &Package{
		name:  pkg.Name,
		defs:  pkg.TypesInfo.Defs,
		files: make([]*File, len(pkg.Syntax)),
	}

	for i, file := range pkg.Syntax {
		g.pkg.files[i] = &File{
			file:    file,
			pkg:     g.pkg,
			fileSet: pkg.Fset,
		}
	}
}

// generate produces the String method for the named type.
func (g *Generator) generate(typeName string) {
	tpts := getTemplate()
	for _, file := range g.pkg.files { //按包来的，读取包下的所有文件
		// Set the state for this run of the walker.
		file.typeName = typeName
		//ast.Print(file.fileSet, file.file)
		if file.file != nil {

			//自定义模板检查
			/*			if *mdo != "" {

						}*/

			structInfo, err := ParseStruct(file.file, file.fileSet, tagName)
			if err != nil {
				fmt.Println("失败:" + err.Error())
				return
			}

			for stName, info := range structInfo {
				if stName != typeName {
					continue
				}
				g.Printf(stName, "// Code generated by \"sloth %s\"; DO NOT EDIT.\n", strings.Join(os.Args[1:], " "))
				g.Printf(stName, "\n")
				g.Printf(stName, "package %s\n", g.pkg.name)
				g.Printf(stName, "\n")
				for _, field := range info {
					for _, access := range autoFunc {
						switch access {
						case funSetter:
							g.Printf(stName, "%s\n", genSetter(tpts[1], stName, field.Name, field.column, field.Type))
						case funGetter:
							g.Printf(stName, "%s\n", genGetter(tpts[0], stName, field.Name, field.Type))
						}
					}
				}
			}

		}
	}

}

var (
	//go:embed model/sloth_getter.tmp
	getterTemplate string
	//go:embed model/sloth_setter.tmp
	setterTemplate string
)

func getTemplate() []*template.Template {
	tptGetter := template.New("getter")
	if customSetter != "" {
		tptGetter = template.Must(tptGetter.Parse(customGetter))
	} else {
		tptGetter = template.Must(tptGetter.Parse(getterTemplate))
	}

	tptSetter := template.New("setter")
	if customSetter != "" {
		tptSetter = template.Must(tptSetter.Parse(customSetter))
	} else {
		tptSetter = template.Must(tptSetter.Parse(setterTemplate))
	}

	return []*template.Template{tptGetter, tptSetter}
}

type StructFieldInfo struct {
	Name   string
	Type   string
	Access []string
	column string
}
type StructFieldInfoArr = []StructFieldInfo

func ParseStruct(file *ast.File, fileSet *token.FileSet, tagName string) (structMap map[string]StructFieldInfoArr, err error) {
	structMap = make(map[string]StructFieldInfoArr)

	collectStructs := func(x ast.Node) bool {
		ts, ok := x.(*ast.TypeSpec)
		if !ok || ts.Type == nil {
			return true
		}

		// 获取结构体名称
		structName := ts.Name.Name

		s, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}
		fileInfos := make([]StructFieldInfo, 0)
		for _, field := range s.Fields.List {
			kind := reflect.TypeOf(file).Kind()
			if kind == reflect.Struct || kind == reflect.Array || kind == reflect.UnsafePointer || len(field.Names) == 0 {
				continue
			}
			name := field.Names[0].Name
			info := StructFieldInfo{Name: name}
			var typeNameBuf bytes.Buffer
			err := printer.Fprint(&typeNameBuf, fileSet, field.Type)
			if err != nil {
				fmt.Println("获取类型失败:", err)
				return true
			}
			info.Type = typeNameBuf.String()
			if field.Tag != nil { // 有tag
				tag := field.Tag.Value
				tag = strings.Trim(tag, "`")
				tags, err := structtag.Parse(tag)
				if err != nil {
					return true
				}
				filedTag, err := tags.Get(tagName)
				submatch := columnCompile.FindStringSubmatch(filedTag.Value())
				if len(submatch) > 0 {
					info.column = submatch[1]
				}

			} else {
				firstChar := name[0:1]
				if strings.ToUpper(firstChar) == firstChar { //大写
					info.Access = []string{funGetter, funSetter}
				} else { // 小写
					info.Access = []string{funGetter}
				}
			}
			fileInfos = append(fileInfos, info)
		}
		structMap[structName] = fileInfos
		return false
	}

	ast.Inspect(file, collectStructs)

	return structMap, nil
}

func genSetter(tpt *template.Template, structName, fieldName, column, typeName string) string {
	res := bytes.NewBufferString("")
	tpt.Execute(res, map[string]string{
		"Receiver": strings.ToLower(structName[0:1]),
		"Struct":   structName,
		"Field":    fieldName,
		"Type":     typeName,
		"Column":   column,
	})
	return res.String()
}

func genGetter(t *template.Template, structName, fieldName, typeName string) string {
	res := bytes.NewBufferString("")
	t.Execute(res, map[string]string{
		"Receiver": strings.ToLower(structName[0:1]),
		"Struct":   structName,
		"Field":    fieldName,
		"Type":     typeName,
	})
	return res.String()
}
