// Copyright 2016 NDP Systèmes. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package generate

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"strings"

	"github.com/gleke/hexya/src/models/fieldtype"
	"github.com/gleke/hexya/src/tools/strutils"
	"golang.org/x/tools/go/packages"
)

// A PackageType describes a type of module
type PackageType int8

const (
	// Base is the PackageType for the base package of a module
	Base PackageType = iota
	// Models is the PackageType for the hexya/models package
	Models
)

// A ModuleInfo is a wrapper around packages.Package with additional data to
// describe a module.
type ModuleInfo struct {
	packages.Package
	ModType PackageType
	FSet    *token.FileSet
}

// NewModuleInfo returns a pointer to a new moduleInfo instance
func NewModuleInfo(pack *packages.Package, modType PackageType, fSet *token.FileSet) *ModuleInfo {
	return &ModuleInfo{
		Package: *pack,
		ModType: modType,
		FSet:    fSet,
	}
}

// GetModulePackages returns a slice of PackageInfo for packages that are hexya modules, that is:
// - A package that declares a "MODULE_NAME" constant
// - A package that is in a subdirectory of a package
// Also returns the 'hexya/models' package since all models are initialized there
func GetModulePackages(packs []*packages.Package) []*ModuleInfo {
	modules := make(map[string]*ModuleInfo)
	// We add to the modulePaths all packages which define a MODULE_NAME constant
	// and we check for 'hexya/models' package
	packages.Visit(packs, func(pack *packages.Package) bool {
		obj := pack.Types.Scope().Lookup("MODULE_NAME")
		if obj != nil {
			modules[pack.Types.Path()] = NewModuleInfo(pack, Base, pack.Fset)
			return true
		}
		if pack.PkgPath == ModelsPath {
			modules[pack.Types.Path()] = NewModuleInfo(pack, Models, pack.Fset)
		}
		return true
	}, func(pack *packages.Package) {})

	// Finally, we build up our result slice from modules map
	modSlice := make([]*ModuleInfo, len(modules))
	var i int
	for _, mod := range modules {
		modSlice[i] = mod
		i++
	}
	return modSlice
}

// A TypeData holds a Type string and optional import path for this type.
type TypeData struct {
	Type       string
	ImportPath string
}

// A FieldASTData is a holder for a field's data that will be used
// for pool code generation
type FieldASTData struct {
	Name        string
	JSON        string
	Help        string
	Description string
	Selection   map[string]string
	RelModel    string
	Type        TypeData
	FType       fieldtype.Type
	IsRS        bool
	MixinField  bool
	EmbedField  bool
	embed       bool
}

// A ParamData holds the name and type of a method parameter
type ParamData struct {
	Name     string
	Variadic bool
	Type     TypeData
}

// A MethodASTData is a holder for a method's data that will be used
// for pool code generation
type MethodASTData struct {
	Name      string
	Doc       string
	PkgPath   string
	Params    []ParamData
	Returns   []TypeData
	ToDeclare bool
}

// A ModelASTData holds fields and methods data of a Model
type ModelASTData struct {
	Name         string
	ModelType    string
	IsModelMixin bool
	Fields       map[string]FieldASTData
	Methods      map[string]MethodASTData
	Mixins       map[string]bool
	Embeds       map[string]bool
	Validated    bool
}

// newModelASTData returns an initialized ModelASTData instance
func newModelASTData(name string) ModelASTData {
	return ModelASTData{
		Name:         name,
		Fields:       defaultFields(name),
		IsModelMixin: ModelMixins[name],
		Methods:      make(map[string]MethodASTData),
		Mixins:       make(map[string]bool),
		Embeds:       make(map[string]bool),
		ModelType:    "",
	}
}

// defaultFields returns the map of default fields for the model with the given name
func defaultFields(name string) map[string]FieldASTData {
	res := make(map[string]FieldASTData)
	idField := FieldASTData{
		Name: "ID",
		JSON: "id",
		Type: TypeData{
			Type: "int64",
		},
		FType: fieldtype.Integer,
	}
	res["ID"] = idField
	switch name {
	case "BaseMixin":
		res["CreateDate"] = FieldASTData{
			Name:        "CreateDate",
			JSON:        "create_date",
			Description: "Created On",
			Type: TypeData{
				Type:       "dates.DateTime",
				ImportPath: DatesPath,
			},
			FType: fieldtype.DateTime,
		}
		res["CreateUID"] = FieldASTData{
			Name:        "CreateUID",
			JSON:        "create_uid",
			Description: "Created By",
			Type:        TypeData{Type: "int64"},
			FType:       fieldtype.Integer,
		}
		res["WriteDate"] = FieldASTData{
			Name:        "WriteDate",
			JSON:        "write_date",
			Description: "Updated On",
			Type: TypeData{
				Type:       "dates.DateTime",
				ImportPath: DatesPath,
			},
			FType: fieldtype.DateTime,
		}
		res["WriteUID"] = FieldASTData{
			Name:        "WriteUID",
			JSON:        "write_uid",
			Description: "Updated By",
			Type:        TypeData{Type: "int64"},
			FType:       fieldtype.Integer,
		}
		res["LastUpdate"] = FieldASTData{
			Name:        "LastUpdate",
			JSON:        "__last_update",
			Description: "Last Updated On",
			Type: TypeData{
				Type:       "dates.DateTime",
				ImportPath: DatesPath,
			},
			FType: fieldtype.DateTime,
		}
		res["DisplayName"] = FieldASTData{
			Name:        "DisplayName",
			JSON:        "display_name",
			Description: "Display Name",
			Type:        TypeData{Type: "string"},
			FType:       fieldtype.Char,
		}
	case "ModelMixin":
		res["HexyaExternalID"] = FieldASTData{
			Name:        "HexyaExternalID",
			JSON:        "hexya_external_id",
			Description: "External ID",
			Type:        TypeData{Type: "string"},
			FType:       fieldtype.Char,
		}
		res["HexyaVersion"] = FieldASTData{
			Name:        "HexyaVersion",
			JSON:        "hexya_version",
			Description: "External Version",
			Type:        TypeData{Type: "int"},
			FType:       fieldtype.Integer,
		}
	}
	return res
}

// GetModelsASTData returns the ModelASTData of all models found when parsing program.
func GetModelsASTData(modules []*ModuleInfo) map[string]ModelASTData {
	return GetModelsASTDataForModules(modules, true)
}

// GetModelsASTDataForModules returns the MethodASTData for all methods in given modules.
// If validate is true, then only models that have been explicitly declared will appear in
// the result. Mixins and embeddings will be inflated too. Use this if you want validate the
// whole application.
func GetModelsASTDataForModules(modInfos []*ModuleInfo, validate bool) map[string]ModelASTData {
	modelsData := make(map[string]ModelASTData)
	for _, modInfo := range modInfos {
		fmt.Println("modInfo")
		fmt.Println(modInfo)
		for _, file := range modInfo.Syntax {
			fmt.Println("file")
			fmt.Println(file)
			ast.Inspect(file, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.CallExpr:
					fnctName, err := ExtractFunctionName(node)
					if err != nil {
						return true
					}
					switch {
					case fnctName == "addMethod":
						parseAddMethod(node, modInfo, &modelsData, false)
					case fnctName == "NewMethod":
						parseAddMethod(node, modInfo, &modelsData, true)
					case fnctName == "InheritModel":
						parseMixInModel(node, modInfo, &modelsData)
					case fnctName == "AddFields":
						parseAddFields(node, modInfo, &modelsData)
					case strutils.StartsAndEndsWith(fnctName, "New", "Model"):
						parseNewModel(node, &modelsData)
					}
				}
				return true
			})
		}
	}
	if !validate {
		// We don't want validation, so we exit early
		return modelsData
	}
	for modelName, md := range modelsData {
		// Delete models that have not been declared explicitly
		// Because it means we have a typing error
		if !md.Validated {
			delete(modelsData, modelName)
		}
		inflateMixins(modelName, &modelsData)
		inflateEmbeds(modelName, &modelsData)
	}
	return modelsData
}

// inflateEmbeds populates the given model with fields from the embedded type
func inflateEmbeds(modelName string, modelsData *map[string]ModelASTData) {
	for emb := range (*modelsData)[modelName].Embeds {
		relModel := (*modelsData)[modelName].Fields[emb].RelModel
		inflateEmbeds(relModel, modelsData)
		for fieldName, field := range (*modelsData)[relModel].Fields {
			if _, exists := (*modelsData)[modelName].Fields[fieldName]; exists {
				continue
			}
			embeddedField := field
			embeddedField.EmbedField = true
			(*modelsData)[modelName].Fields[fieldName] = embeddedField
		}
	}
}

// inflateMixins populates the given model with fields
// and methods defined in its mixins
func inflateMixins(modelName string, modelsData *map[string]ModelASTData) {
	for mixin := range (*modelsData)[modelName].Mixins {
		inflateMixins(mixin, modelsData)
		for fieldName, field := range (*modelsData)[mixin].Fields {
			if fieldName == "ID" {
				continue
			}
			field.MixinField = true
			(*modelsData)[modelName].Fields[fieldName] = field
		}
		for methodName, method := range (*modelsData)[mixin].Methods {
			method.ToDeclare = true
			(*modelsData)[modelName].Methods[methodName] = method
		}
	}
}

// parseMixInModel updates the mixin tree with the given node which is a InheritModel function
func parseMixInModel(node *ast.CallExpr, modInfo *ModuleInfo, modelsData *map[string]ModelASTData) {
	fNode := node.Fun.(*ast.SelectorExpr)
	modelName, err := extractModel(fNode.X, modInfo)
	if err != nil {
		if _, ok := err.(generalMixinError); ok {
			return
		}
		log.Panic("Unable to extract model while visiting AST", "error", err, "node", modInfo.FSet.Position(node.Pos()))
	}
	mixinModel, err := extractModel(node.Args[0], modInfo)
	if err != nil {
		log.Panic("Unable to extract mixin model while visiting AST", "error", err)
	}
	if _, exists := (*modelsData)[modelName]; !exists {
		(*modelsData)[modelName] = newModelASTData(modelName)
	}
	(*modelsData)[modelName].Mixins[mixinModel] = true
}

// parseNewModel parses the given node which is a NewXXXModel function
func parseNewModel(node *ast.CallExpr, modelsData *map[string]ModelASTData) {
	fName, _ := ExtractFunctionName(node)
	modelName := strings.Trim(node.Args[0].(*ast.BasicLit).Value, "\"`")
	modelType := strings.TrimSuffix(strings.TrimPrefix(fName, "New"), "Model")

	setModelData(modelsData, modelName, modelType)
}

// setModelData adds a model with the given name and type to the given modelsData
func setModelData(modelsData *map[string]ModelASTData, modelName string, modelType string) {
	model, exists := (*modelsData)[modelName]
	if !exists {
		model = newModelASTData(modelName)
	}
	if modelName != "CommonMixin" {
		model.Mixins["CommonMixin"] = true
	}
	switch modelType {
	case "":
		model.Mixins["BaseMixin"] = true
		model.Mixins["ModelMixin"] = true
	case "Transient":
		model.Mixins["BaseMixin"] = true
	}
	model.ModelType = modelType
	model.Validated = true
	(*modelsData)[modelName] = model
}

// ExtractFunctionName returns the name of the called function
// in the given call expression.
func ExtractFunctionName(node *ast.CallExpr) (string, error) {
	var fName string
	switch nf := node.Fun.(type) {
	case *ast.SelectorExpr:
		fName = nf.Sel.Name
	case *ast.Ident:
		fName = nf.Name
	default:
		return "", errors.New("unexpected node type")
	}
	return fName, nil
}

// parseAddFields parses the given node which is an AddFields function
func parseAddFields(node *ast.CallExpr, modInfo *ModuleInfo, modelsData *map[string]ModelASTData) {
	fNode := node.Fun.(*ast.SelectorExpr)
	modelName, err := extractModel(fNode.X, modInfo)
	if err != nil {
		log.Panic("Unable to extract model while visiting AST", "error", err)
	}
	if _, exists := (*modelsData)[modelName]; !exists {
		(*modelsData)[modelName] = newModelASTData(modelName)
	}
	var fields *ast.CompositeLit
	switch n := node.Args[0].(type) {
	case *ast.CompositeLit:
		fields = n
	case *ast.Ident:
		fields = n.Obj.Decl.(*ast.ValueSpec).Values[0].(*ast.CompositeLit)
	}
	for _, f := range fields.Elts {
		fDef := f.(*ast.KeyValueExpr)
		fieldName := strings.Trim(fDef.Key.(*ast.BasicLit).Value, "\"`")
		var typeStr string

		switch ft := fDef.Value.(*ast.CompositeLit).Type.(type) {
		case *ast.Ident:
			typeStr = strings.TrimSuffix(ft.Name, "Field")
		case *ast.SelectorExpr:
			typeStr = strings.TrimSuffix(ft.Sel.Name, "Field")
		}
		var importPath string
		if typeStr == "Date" || typeStr == "DateTime" {
			importPath = DatesPath
		}

		var fieldParams []ast.Expr
		switch fd := fDef.Value.(type) {
		case *ast.Ident:
			fieldParams = fd.Obj.Decl.(*ast.CompositeLit).Elts
		case *ast.CompositeLit:
			fieldParams = fd.Elts
		}
		fType := fieldtype.Type(strings.ToLower(typeStr))
		fData := FieldASTData{
			Name:  fieldName,
			FType: fType,
			Type: TypeData{
				Type:       fType.DefaultGoType().String(),
				ImportPath: importPath,
			},
		}
		for _, elem := range fieldParams {
			fElem := elem.(*ast.KeyValueExpr)
			fData = parseFieldAttribute(fElem, fData, modInfo)
			if fData.embed {
				(*modelsData)[modelName].Embeds[fieldName] = true
			}
		}
		(*modelsData)[modelName].Fields[fieldName] = fData
	}
}

// parseFieldAttribute parses the given KeyValueExpr of a field definition
func parseFieldAttribute(fElem *ast.KeyValueExpr, fData FieldASTData, modInfo *ModuleInfo) FieldASTData {
	switch fElem.Key.(*ast.Ident).Name {
	case "JSON":
		fData.JSON = parseStringValue(fElem.Value)
	case "Help":
		fData.Help = parseStringValue(fElem.Value)
	case "String":
		fData.Description = parseStringValue(fElem.Value)
	case "Selection":
		fData.Selection = extractSelection(fElem.Value)
	case "RelationModel":
		modName, err := extractModel(fElem.Value, modInfo)
		if err != nil {
			log.Panic("Unable to parse RelationModel", "field", fData.Name, "error", err)
		}
		fData.RelModel = modName
		fData.IsRS = true
	case "GoType":
		fData.Type = getTypeData(fElem.Value.(*ast.CallExpr).Args[0], modInfo)
	case "Embed":
		if fElem.Value.(*ast.Ident).Name == "true" {
			fData.embed = true
		}
	}
	return fData
}

// parseStringValue returns the value of a string expr which can be a literal
// or an identifier for a string.
func parseStringValue(expr ast.Expr) string {
	var str string
	switch v := expr.(type) {
	case *ast.BasicLit:
		str = v.Value
	case *ast.Ident:
		str = parseStringValue(v.Obj.Decl.(*ast.ValueSpec).Values[0])
	}
	return strings.Trim(str, "\"`")
}

// extractSelection returns a map with the keys and values of the Selection
// specified by expr.
func extractSelection(expr ast.Expr) map[string]string {
	res := make(map[string]string)
	switch e := expr.(type) {
	case *ast.CompositeLit:
		for _, elt := range e.Elts {
			elem := elt.(*ast.KeyValueExpr)
			key := elem.Key.(*ast.BasicLit).Value
			value := strings.Trim(elem.Value.(*ast.BasicLit).Value, "\"`")
			res[key] = value
		}
	}
	return res
}

// parseAddMethod parses the given node which is an addMethod function
func parseAddMethod(node *ast.CallExpr, modInfo *ModuleInfo, modelsData *map[string]ModelASTData, toDeclare bool) {
	fNode := node.Fun.(*ast.SelectorExpr)
	modelName, err := extractModel(fNode.X, modInfo)
	if err != nil {
		log.Panic("Unable to extract model while visiting AST", "error", err)
	}
	methodName := strings.Trim(node.Args[0].(*ast.BasicLit).Value, "\"`")

	var (
		funcType *ast.FuncType
		doc      string
	)
	switch fd := node.Args[1].(type) {
	case *ast.Ident:
		funcDecl := fd.Obj.Decl.(*ast.FuncDecl)
		funcType = funcDecl.Type
		doc = funcDecl.Doc.Text()
	case *ast.FuncLit:
		funcType = fd.Type
	}
	if _, exists := (*modelsData)[modelName]; !exists {
		(*modelsData)[modelName] = newModelASTData(modelName)
	}
	methData := MethodASTData{
		Name:      methodName,
		Doc:       formatDocString(doc),
		PkgPath:   modInfo.PkgPath,
		Params:    extractParams(funcType, modInfo),
		Returns:   extractReturnType(funcType, modInfo),
		ToDeclare: toDeclare,
	}
	(*modelsData)[modelName].Methods[methodName] = methData
}

// A generalMixinError is returned if the mixin is
// a general mixin set in NewXXXXModel function.
type generalMixinError struct{}

// Error method for generalMixinError
func (gme generalMixinError) Error() string {
	return "General Mixin Error"
}

var _ error = generalMixinError{}

// extractModel returns the string name of the model of the given ident variable
// ident must point to the expr which represents a model
// Returns an error if it cannot determine the model
func extractModel(ident ast.Expr, modInfo *ModuleInfo) (string, error) {
	switch idt := ident.(type) {
	case *ast.Ident:
		// Method is called on an identifier without selector such as
		// user.addMethod. In this case, we try to find out the model from
		// the identifier declaration.
		switch decl := idt.Obj.Decl.(type) {
		case *ast.AssignStmt:
			// The declaration is also an assignment
			switch rd := decl.Rhs[0].(type) {
			case *ast.CallExpr:
				// The assignment is a call to a function
				var fnIdent *ast.Ident
				switch ft := rd.Fun.(type) {
				case *ast.Ident:
					fnIdent = ft
				case *ast.SelectorExpr:
					fnIdent = ft.Sel
				default:
					return "", fmt.Errorf("unexpected function identifier: %v (%T)", rd.Fun, rd.Fun)
				}
				switch fnIdent.Name {
				case "Get", "MustGet", "NewModel", "NewMixinModel", "NewTransientModel", "NewManualModel":
					return strings.Trim(rd.Args[0].(*ast.BasicLit).Value, "\"`"), nil
				case "CreateModel", "getOrCreateModel":
					// This is a call from inside a NewXXXXModel function
					return "", generalMixinError{}
				default:
					return extractModelNameFromFunc(rd, modInfo)
				}
			case *ast.Ident:
				// The assignment is another identifier, we go to the declaration of this new ident.
				return extractModel(rd, modInfo)
			default:
				return "", fmt.Errorf("unmanaged type %T at %s for %s", rd, modInfo.FSet.Position(rd.Pos()), idt.Name)
			}
		}
	case *ast.CallExpr:
		return extractModelNameFromFunc(idt, modInfo)
	default:
		return "", fmt.Errorf("unmanaged call. ident: %s (%T)", idt, idt)
	}
	return "", errors.New("unmanaged situation")
}

// extractModelNameFromFunc extracts the model name from a h.ModelName()
// expression or an error if this is not a pool function.
func extractModelNameFromFunc(ce *ast.CallExpr, modInfo *ModuleInfo) (string, error) {
	switch ft := ce.Fun.(type) {
	case *ast.Ident:
		// func is called without selector, then it is not from pool
		return "", errors.New("function call without selector")
	case *ast.SelectorExpr:
		switch ftt := ft.X.(type) {
		case *ast.Ident:
			if ftt.Name != PoolModelPackage && ftt.Name != "Registry" {
				return extractModel(ftt, modInfo)
			}
			return ft.Sel.Name, nil
		case *ast.CallExpr:
			return extractModel(ftt, modInfo)
		default:
			return "", fmt.Errorf("selector is of not managed type: %T", ftt)
		}
	}
	return "", errors.New("unparsable function call")
}

// extractParams extracts the parameters of the given FuncType
func extractParams(ft *ast.FuncType, modInfo *ModuleInfo) []ParamData {
	var params []ParamData
	for i, pl := range ft.Params.List {
		if i == 0 {
			// pass the first argument (rs)
			continue
		}
		for _, nn := range pl.Names {
			var variadic bool
			typ := pl.Type
			if el, ok := typ.(*ast.Ellipsis); ok {
				typ = el.Elt
				variadic = true
			}
			params = append(params, ParamData{
				Name:     nn.Name,
				Variadic: variadic,
				Type:     getTypeData(typ, modInfo)})
		}
	}
	return params
}

// getTypeData returns a TypeData instance representing the typ AST Expression
func getTypeData(typ ast.Expr, modInfo *ModuleInfo) TypeData {
	typStr := types.TypeString(modInfo.TypesInfo.TypeOf(typ), (*types.Package).Name)
	if strings.Contains(typStr, "invalid type") {
		// Maybe this is a pool type that is not yet defined
		byts := bytes.Buffer{}
		printer.Fprint(&byts, modInfo.FSet, typ)
		typStr = byts.String()
	}
	importPath := computeExportPath(modInfo.TypesInfo.TypeOf(typ))
	if strings.Contains(importPath, PoolPath) {
		importPath = ""
	}

	importPathTokens := strings.Split(importPath, ".")
	if len(importPathTokens) > 0 {
		importPath = strings.Join(importPathTokens[:len(importPathTokens)-1], ".")
	}
	return TypeData{
		Type:       typStr,
		ImportPath: importPath,
	}
}

// extractReturnType returns the return type of the first returned value
// of the given FuncType as a string and an import path if needed.
func extractReturnType(ft *ast.FuncType, modInfo *ModuleInfo) []TypeData {
	var res []TypeData
	if ft.Results != nil {
		for _, l := range ft.Results.List {
			res = append(res, getTypeData(l.Type, modInfo))
		}
	}
	return res
}

// computeExportPath returns the import path of the given type
func computeExportPath(typ types.Type) string {
	var res string
	switch typTyped := typ.(type) {
	case *types.Struct, *types.Named:
		res = types.TypeString(typTyped, (*types.Package).Path)
	case *types.Pointer:
		res = computeExportPath(typTyped.Elem())
	case *types.Slice:
		res = computeExportPath(typTyped.Elem())
	}
	return res
}

// formatDocString formats the given string by stripping whitespaces at the
// beginning of each line and prepend "// ". It also strips empty lines at
// the beginning.
func formatDocString(doc string) string {
	var res string
	var dataStarted bool
	for _, line := range strings.Split(doc, "\n") {
		line = strings.TrimSpace(line)
		if line == "" && !dataStarted {
			continue
		}
		dataStarted = true
		res += fmt.Sprintf("// %s\n", line)
	}
	return strings.TrimRight(res, "/ \n")
}
