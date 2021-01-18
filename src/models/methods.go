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

package models

import (
	"reflect"
	"sync"

	"github.com/gleke/hexya/src/models/security"
)

// unauthorizedMethods lists methods that should not be given execution permission by default
var unauthorizedMethods = map[string]bool{
	"Load":   true,
	"Create": true,
	"Write":  true,
	"Unlink": true,
}

// A MethodsCollection is a collection of methods for use in a model
type MethodsCollection struct {
	model        *Model
	registry     map[string]*Method
	bootstrapped bool
}

// get returns the Method with the given method name.
//
// The second return value is true if a method has been found.
// The third return value is true if the method has been found in this model
// and not in mixins.
func (mc *MethodsCollection) get(methodName string) (*Method, bool, bool) {
	var inMixin bool
	mi, ok := mc.registry[methodName]
	if !ok {
		if mc.bootstrapped {
			return nil, false, false
		}
		// We have not bootstrapped yet
		// We didn't find the method, but maybe it exists in mixins
		meth, found := mc.model.findMethodInMixin(methodName)
		if !found {
			return nil, false, false
		}
		// It exists in mixin. We add it to this method collection on the fly
		// so that we can set permissions on it before bootstrap.
		mi = copyMethod(mc.model, meth)
		mc.set(methodName, mi)
		inMixin = true
	}
	return mi, true, !inMixin
}

// Get returns the Method with the given method name.
//
// The second return value is true if a method has been found.
func (mc *MethodsCollection) Get(methodName string) (*Method, bool) {
	meth, exists, _ := mc.get(methodName)
	return meth, exists
}

// MustGet returns the Method of the given method. It panics if the
// method is not found.
func (mc *MethodsCollection) MustGet(methodName string) *Method {
	methInfo, exists := mc.Get(methodName)
	if !exists {
		log.Panic("Unknown method in model", "model", mc.model.name, "method", methodName)
	}
	return methInfo
}

// set adds the given Method to the MethodsCollection.
func (mc *MethodsCollection) set(methodName string, methInfo *Method) {
	if mc.model != methInfo.model {
		log.Panic("Trying to set a method from a different model", "mcModel", mc.model.name, "methModel", methInfo.model.name, "method", methodName)
	}
	mc.registry[methodName] = methInfo
	if !unauthorizedMethods[methodName] {
		methInfo.AllowGroup(security.GroupEveryone)
	}
}

// AllowAllToGroup grants the given group access to all the CRUD methods of this collection
func (mc *MethodsCollection) AllowAllToGroup(group *security.Group) {
	for mName := range unauthorizedMethods {
		meth, exists, _ := mc.get(mName)
		if !exists {
			log.Panic("Unknown method in model", "model", mc.model.name, "method", mName)
		}
		meth.AllowGroup(group)
	}
}

// RevokeAllFromGroup revokes permissions on all CRUD methods given by AllowAllToGroup
func (mc *MethodsCollection) RevokeAllFromGroup(group *security.Group) {
	for mName := range unauthorizedMethods {
		meth, exists, _ := mc.get(mName)
		if !exists {
			log.Panic("Unknown method in model", "model", mc.model.name, "method", mName)
		}
		meth.RevokeGroup(group)
	}
}

// newMethodsCollection returns a pointer to a new MethodsCollection
func newMethodsCollection() *MethodsCollection {
	mc := MethodsCollection{
		registry: make(map[string]*Method),
	}
	return &mc
}

// A callerGroup is the concatenation of a caller method and a security group
// It is used to lookup execution permissions.
type callerGroup struct {
	caller *Method
	group  *security.Group
}

// A Method is a definition of a model's method
type Method struct {
	sync.RWMutex
	name          string
	model         *Model
	methodType    reflect.Type
	topLayer      *methodLayer
	nextLayer     map[*methodLayer]*methodLayer
	groups        map[*security.Group]bool
	groupsCallers map[callerGroup]bool
}

// MethodType returns the methodType of a Method
func (m *Method) MethodType() reflect.Type {
	return m.methodType
}

// addMethodLayer adds the given layer to this Method.
func (m *Method) addMethodLayer(val reflect.Value) {
	m.Lock()
	defer m.Unlock()
	ml := methodLayer{
		funcValue: wrapFunctionForMethodLayer(val),
		method:    m,
	}
	if m.topLayer != nil {
		m.nextLayer[&ml] = m.topLayer
	}
	m.topLayer = &ml
}

func (m *Method) getNextLayer(methodLayer *methodLayer) *methodLayer {
	return m.nextLayer[methodLayer]
}

// invertedLayers returns the list of method layers starting
// from the base methods and going up all inherited layers
func (m *Method) invertedLayers() []*methodLayer {
	var layersInv []*methodLayer
	for cl := m.topLayer; cl != nil; cl = m.getNextLayer(cl) {
		layersInv = append([]*methodLayer{cl}, layersInv...)
	}
	return layersInv
}

// AllowGroup grants the execution permission on this method to the given group
// If callers are defined, then the permission is granted only when this method
// is called from one of the callers, otherwise it is granted from any caller.
func (m *Method) AllowGroup(group *security.Group, callers ...Methoder) *Method {
	m.Lock()
	defer m.Unlock()
	if len(callers) == 0 {
		m.groups[group] = true
		return m
	}
	for _, caller := range callers {
		m.groupsCallers[callerGroup{caller: caller.Underlying(), group: group}] = true
	}
	return m
}

// RevokeGroup revokes the execution permission on the method to the given group
// if it has been given previously, otherwise does nothing.
// Note that this methods revokes all permissions, whatever the caller.
func (m *Method) RevokeGroup(group *security.Group) *Method {
	m.Lock()
	defer m.Unlock()
	delete(m.groups, group)
	for cg := range m.groupsCallers {
		if cg.group == group {
			delete(m.groupsCallers, cg)
		}
	}
	return m
}

// Underlying returns the underlysing method data object
func (m *Method) Underlying() *Method {
	return m
}

// Call executes the given method with the given parameters
// and returns (only) the first returned value
func (m *Method) Call(rc *RecordCollection, params ...interface{}) interface{} {
	return rc.Call(m.name, params...)
}

// CallMulti executes the given method with the given parameters
// and returns all returned value as []interface{}.
func (m *Method) CallMulti(rc *RecordCollection, params ...interface{}) []interface{} {
	return rc.CallMulti(m.name, params...)
}

var _ Methoder = new(Method)

// methodLayer is one layer of a method, that is one function defined in a module
type methodLayer struct {
	method    *Method
	mixedIn   bool
	funcValue reflect.Value
}

// copyMethod creates a new method without any method layer for
// the given model by taking data from the given method.
func copyMethod(m *Model, method *Method) *Method {
	return &Method{
		model:         m,
		name:          method.name,
		methodType:    method.methodType,
		nextLayer:     make(map[*methodLayer]*methodLayer),
		groups:        make(map[*security.Group]bool),
		groupsCallers: make(map[callerGroup]bool),
	}
}

// wrapFunctionForMethodLayer take the given fnct Value and wrap it in a
// func(RecordCollection, args...) function Value suitable for use in a
// methodLayer.
func wrapFunctionForMethodLayer(fnctVal reflect.Value) reflect.Value {
	wrapperType := reflect.TypeOf(func(*RecordCollection, ...interface{}) []interface{} { return nil })
	if fnctVal.Type() == wrapperType {
		// fnctVal is already wrapped, we just return it
		return fnctVal
	}
	methodLayerFunction := func(rc *RecordCollection, args ...interface{}) []interface{} {
		argZeroType := fnctVal.Type().In(0)
		argsVals := make([]reflect.Value, len(args)+1)
		argsVals[0] = reflect.New(argZeroType).Elem()
		switch argZeroType {
		case reflect.TypeOf(new(RecordCollection)):
			argsVals[0].Set(reflect.ValueOf(rc))
		default:
			modelName := argZeroType.Name()[:len(argZeroType.Name())-3]
			argsVals[0].Set(reflect.ValueOf(rc.Wrap(modelName)))
		}
		for i := 0; i < fnctVal.Type().NumIn()-1; i++ {
			if len(args) < i+1 && fnctVal.Type().IsVariadic() && i == fnctVal.Type().NumIn()-2 {
				// Handle variadic function call without last argument
				break
			}
			argsVals[i+1] = convertFunctionArg(fnctVal.Type().In(i+1), args[i])
		}

		var retVal []reflect.Value
		if fnctVal.Type().IsVariadic() && len(argsVals) == fnctVal.Type().NumIn() {
			retVal = fnctVal.CallSlice(argsVals)
		} else {
			retVal = fnctVal.Call(argsVals)
		}

		res := make([]interface{}, len(retVal))
		for i, val := range retVal {
			res[i] = val.Interface()
		}
		return res
	}
	return reflect.ValueOf(methodLayerFunction)
}

// convertFunctionArg converts the given argument to match that of fnctArgType.
func convertFunctionArg(fnctArgType reflect.Type, arg interface{}) reflect.Value {
	var val reflect.Value
	switch at := arg.(type) {
	case Conditioner:
		if fnctArgType.Kind() == reflect.Interface {
			// Target is a Conditioner nothing to change
			return reflect.ValueOf(at)
		}
		if fnctArgType == reflect.TypeOf(new(Condition)) {
			// Target is a pointer to an untyped Condition
			return reflect.ValueOf(at.Underlying())
		}
		val = reflect.New(fnctArgType).Elem()
		val.Field(0).Set(reflect.ValueOf(at.Underlying()))
		return val
	case RecordData:
		var fm RecordData
		if fnctArgType == reflect.TypeOf(fm) {
			// Target is a RecordData nothing to change
			return reflect.ValueOf(at)
		}
		if fnctArgType == reflect.TypeOf(new(ModelData)) {
			// Target is a *ModelData so we send Underlying
			return reflect.ValueOf(at.Underlying())
		}
		// => Target is a typed RecordData
		if md, ok := at.(*ModelData); ok {
			// Given arg is a ModelData, so we wrap it
			val = reflect.ValueOf(md.Wrap())
			return val
		}
		// Given arg is already a typed ModelData
		return reflect.ValueOf(arg)
	case RecordSet:
		if fnctArgType == reflect.TypeOf((*RecordSet)(nil)).Elem() {
			return reflect.ValueOf(at)
		}
		if fnctArgType == reflect.TypeOf(new(RecordCollection)) {
			return reflect.ValueOf(at.Collection())
		}
		return reflect.ValueOf(at.Collection().Wrap())
	case nil:
		return reflect.Zero(fnctArgType)
	default:
		return reflect.ValueOf(arg)
	}
}

// addMethod is an alias for NewMethod used in this package's base_model and
// that is treated differently by code generation.
//
// Do NOT factorize code with NewMethod or the code generation will fail
func (m *Model) addMethod(methodName string, fnct interface{}) *Method {
	meth, exists, inModel := m.methods.get(methodName)
	if exists && !inModel {
		// We are trying to add an existing mixin method as a new method
		log.Panic("Call to NewMethod (addMethod) with an existing method name", "model", m.name, "method", methodName)
	}
	// meth might not exist if it has not been declared in pool package
	if !exists {
		meth = m.AddEmptyMethod(methodName)
	}
	meth.finalize(fnct)
	return meth
}

// NewMethod is used in modules to declare a new method for this model.
func (m *Model) NewMethod(methodName string, fnct interface{}) *Method {
	meth, exists, inModel := m.methods.get(methodName)
	if exists && !inModel {
		// We are trying to add an existing mixin method as a new method
		log.Panic("Call to NewMethod (NewMethod) with an existing method name", "model", m.name, "method", methodName)
	}
	// meth might not exist if it has not been declared in pool package
	if !exists {
		meth = m.AddEmptyMethod(methodName)
	}
	meth.finalize(fnct)
	return meth
}

// AddEmptyMethod creates a new method without function layer
// The resulting method cannot be called until finalize is called
func (m *Model) AddEmptyMethod(methodName string) *Method {
	if m.methods.bootstrapped {
		log.Panic("Create/ExtendMethod must be run before BootStrap", "model", m.name, "method", methodName)
	}
	meth, exists, inModel := m.methods.get(methodName)
	if exists && inModel {
		log.Panic("Call to NewMethod (AddEmptyMethod) with an existing method name", "model", m.name, "method", methodName)
	}
	if !exists {
		meth = &Method{
			model:         m,
			name:          methodName,
			nextLayer:     make(map[*methodLayer]*methodLayer),
			groups:        make(map[*security.Group]bool),
			groupsCallers: make(map[callerGroup]bool),
		}
	}
	m.methods.set(methodName, meth)
	return meth
}

// finalize adds the given fnct as first method layer to this method
func (m *Method) finalize(fnct interface{}) *Method {
	if m.topLayer != nil {
		log.Panic("Call to NewMethod (finalize) with an existing method name", "model", m.model.name, "method", m.name)
	}
	m.checkMethodAndFnctType(fnct)
	val := reflect.ValueOf(fnct)
	m.addMethodLayer(val)
	m.methodType = val.Type()
	return m
}

// Extend adds the given fnct function as a new layer on this method.
// fnct must be of the same signature as the first layer of this method.
func (m *Method) Extend(fnct interface{}) *Method {
	m.checkMethodAndFnctType(fnct)
	val := reflect.ValueOf(fnct)
	if m.methodType != nil {
		// It can happen that methodType is nil if the first layer is defined after
		// this extension (such as in a mixin).
		m.checkSignaturesMatch(val)
	}
	m.methodType = val.Type()
	m.addMethodLayer(val)
	return m
}

// checkSignaturesMatch panics if the signature of fnctVal doesn't match
// with the signature of the method.
func (m *Method) checkSignaturesMatch(fnctVal reflect.Value) {
	if m.methodType.NumIn() != fnctVal.Type().NumIn() {
		log.Panic("Number of args do not match", "model", m.model.name, "method", m.name,
			"no_arguments", fnctVal.Type().NumIn(), "expected", m.methodType.NumIn())
	}
	for i := 1; i < m.methodType.NumIn(); i++ {
		if !checkTypesMatch(m.methodType.In(i), fnctVal.Type().In(i)) {
			log.Panic("Function signature does not match", "model", m.model.name, "method", m.name,
				"argument", i, "expected", m.methodType.In(i), "received", fnctVal.Type().In(i))
		}
	}
	if m.methodType.NumOut() != fnctVal.Type().NumOut() {
		log.Panic("Number of returns do not match", "model", m.model.name, "method", m.name,
			"no_arguments", fnctVal.Type().NumOut(), "expected", m.methodType.NumOut())
	}
	for i := 0; i < m.methodType.NumOut(); i++ {
		if !checkTypesMatch(m.methodType.Out(i), fnctVal.Type().Out(i)) {
			log.Panic("Function return type does not match", "model", m.model.name, "method", m.name,
				"expected", m.methodType.Out(i), "received", fnctVal.Type().Out(i))
		}
	}
	if m.methodType.IsVariadic() != fnctVal.Type().IsVariadic() {
		log.Panic("Variadic mismatch", "model", m.name, "method", m.name,
			"base_is_variadic", m.methodType.IsVariadic(), "ext_is_variadic", fnctVal.Type().IsVariadic())
	}
}

// checkTypesMatch returns true if both given types match
// Two types match if :
// - both types are the same
// - type2 implements type1 or vice-versa
// - if one type is a pointer to a RecordCollection and the second
// one implements the RecordSet interface.
// - if one type is a FieldMap and the other implements FieldMapper
// - if one type is a Condition and the other implements Conditioner
func checkTypesMatch(type1, type2 reflect.Type) bool {
	if type1 == type2 {
		return true
	}
	if type1 == reflect.TypeOf(new(RecordCollection)) && type2.Implements(reflect.TypeOf((*RecordSet)(nil)).Elem()) {
		return true
	}
	if type2 == reflect.TypeOf(new(RecordCollection)) && type1.Implements(reflect.TypeOf((*RecordSet)(nil)).Elem()) {
		return true
	}
	if type1 == reflect.TypeOf(FieldMap{}) && type2.Implements(reflect.TypeOf((*FieldMapper)(nil)).Elem()) {
		return true
	}
	if type2 == reflect.TypeOf(FieldMap{}) && type1.Implements(reflect.TypeOf((*FieldMapper)(nil)).Elem()) {
		return true
	}
	if type2.Kind() == reflect.Interface && type1.Implements(type2) {
		return true
	}
	if type1.Kind() == reflect.Interface && type2.Implements(type1) {
		return true
	}
	return false
}

// findMethodInMixin recursively goes through all mixins
// to find the method with the given name. Returns true if
// it found one, false otherwise.
func (m *Model) findMethodInMixin(methodName string) (*Method, bool) {
	for _, mixin := range m.mixins {
		if method, ok := mixin.methods.Get(methodName); ok {
			return method, true
		}
		if method, ok := mixin.findMethodInMixin(methodName); ok {
			return method, true
		}
	}
	return nil, false
}

// checkMethodAndFnctType checks whether the given arguments are valid for
// NewMethod or Extend. It panics if this is not the case
func (m *Method) checkMethodAndFnctType(fnct interface{}) {
	if m.model.methods.bootstrapped {
		log.Panic("Create/ExtendMethod must be run before BootStrap", "model", m.name, "method", m.name)
	}
	val := reflect.ValueOf(fnct)
	if val.Kind() != reflect.Func {
		log.Panic("fnct parameter must be a function", "model", m.name, "method", m.name, "fnct", fnct)
	}
	funcType := val.Type()
	if funcType.NumIn() == 0 || !funcType.In(0).Implements(reflect.TypeOf((*RecordSet)(nil)).Elem()) {
		log.Panic("Function must have a `RecordSet` as first argument to be used as method.",
			"model", m.model.name, "method", m.name, "type", funcType.In(0))
	}
}

// Name returns the name of the method
func (m *Method) Name() string {
	return m.name
}
