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
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"github.com/gleke/hexya/src/models/fieldtype"
)

// A RecordRef uniquely identifies a Record by giving its model and ID.
type RecordRef struct {
	ModelName string
	ID        int64
}

// RecordSet identifies a type that holds a set of records of
// a given model.
type RecordSet interface {
	sql.Scanner
	fmt.Stringer
	// ModelName returns the name of the model of this RecordSet
	ModelName() string
	// Ids returns the ids in this set of Records
	Ids() []int64
	// Env returns the current Environment of this RecordSet
	Env() Environment
	// Len returns the number of records in this RecordSet
	Len() int
	// IsValid returns true if this RecordSet has been initialized.
	IsValid() bool
	// IsEmpty returns true if this RecordSet has no records
	IsEmpty() bool
	// IsNotEmpty returns true if this RecordSet has at least one record
	IsNotEmpty() bool
	// Call executes the given method (as string) with the given arguments
	Call(string, ...interface{}) interface{}
	// Collection returns the underlying RecordCollection instance
	Collection() *RecordCollection
	// Get returns the value of the given fieldName for the first record of this RecordCollection.
	// It returns the type's zero value if the RecordCollection is empty.
	Get(FieldName) interface{}
	// Set sets field given by fieldName to the given value. If the RecordSet has several
	// Records, all of them will be updated. Each call to Set makes an update query in the
	// database. It panics if it is called on an empty RecordSet.
	Set(FieldName, interface{})
	// T translates the given string to the language specified by
	// the 'lang' key of rc.Env().Context(). If for any reason the
	// string cannot be translated, then src is returned.
	//
	// You MUST pass a string literal as src to have it extracted automatically (and not a variable)
	//
	// The translated string will be passed to fmt.Sprintf with the optional args
	// before being returned.
	T(string, ...interface{}) string
	// EnsureOne panics if this Recordset is not a singleton
	EnsureOne()
}

// A FieldName is a type that can represents a field in a model.
// It can yield the field name or the field's JSON name as a string
type FieldName interface {
	Name() string
	JSON() string
}

// fieldName is a simple implementation of FieldName
type fieldName struct {
	name string
	json string
}

// Name returns the field's name
func (f fieldName) Name() string {
	return f.name
}

// JSON returns the field's json name
func (f fieldName) JSON() string {
	return f.json
}

// NewFieldName returns a fieldName instance with the given name and json
func NewFieldName(name, json string) FieldName {
	return fieldName{name: name, json: json}
}

// FieldNames is a slice of FieldName that can be sorted
type FieldNames []FieldName

// Len returns the length of the FieldName slice
func (f FieldNames) Len() int {
	return len(f)
}

// Less returns true if f[i] < f[j]. FieldNames are ordered by JSON names
func (f FieldNames) Less(i, j int) bool {
	return f[i].JSON() < f[j].JSON()
}

// Swap i and j indexes
func (f FieldNames) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

// UnmarshalJSON for the FieldNames type
func (f *FieldNames) UnmarshalJSON(data []byte) error {
	var aux []string
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	for _, v := range aux {
		*f = append(*f, NewFieldName(v, v))
	}
	return nil
}

// Names returns a slice with the names of each field
func (f FieldNames) Names() []string {
	var res []string
	for _, fn := range f {
		res = append(res, fn.Name())
	}
	return res
}

// JSON returns a slice with the JSON names of each field
func (f FieldNames) JSON() []string {
	var res []string
	for _, fn := range f {
		res = append(res, fn.JSON())
	}
	return res
}

// A GroupAggregateRow holds a row of results of a query with a group by clause
// - Values holds the values of the actual query
// - Count is the number of lines aggregated into this one
// - Condition can be used to query the aggregated rows separately if needed
type GroupAggregateRow struct {
	Values    *ModelData
	Count     int
	Condition *Condition
}

// FieldContexts define the different contexts for a field, that will define different
// values for this field.
//
// The key is a context name and the value is a function that returns the context
// value for the given recordset.
type FieldContexts map[string]func(RecordSet) string

// A FieldMapper is an object that can convert itself into a FieldMap
type FieldMapper interface {
	// Underlying returns the object converted to a FieldMap.
	Underlying() FieldMap
}

// A Methoder can return a Method data object through its Underlying() method
type Methoder interface {
	Underlying() *Method
}

// A Modeler can return a Model data object through its Underlying() method
type Modeler interface {
	Underlying() *Model
}

// A Conditioner can return a Condition object through its Underlying() method
type Conditioner interface {
	Underlying() *Condition
}

// A RecordData can return a ModelData object through its Underlying() method
type RecordData interface {
	sql.Scanner
	Underlying() *ModelData
}

// A ModelData is used to hold values of an object instance for creating or
// updating a RecordSet. It is mainly designed to be embedded in a type-safe
// struct.
type ModelData struct {
	FieldMap
	ToCreate map[string][]*ModelData
	Model    *Model
}

var _ RecordData = new(ModelData)

// Scan implements sql.Scanner
func (md *ModelData) Scan(src interface{}) error {
	switch val := src.(type) {
	case nil:
		return nil
	case FieldMapper:
		md.FieldMap = val.Underlying()
	case map[string]interface{}:
		md.FieldMap = val
	default:
		return fmt.Errorf("unexpected type %T to represent RecordData: %s", src, src)
	}
	return nil
}

// Get returns the value of the given field.
//
// The field can be either its name or is JSON name.
func (md *ModelData) Get(field FieldName) interface{} {
	res, _ := md.FieldMap.Get(field)
	return res
}

// Has returns true if this ModelData has values for the given field.
//
// The field can be either its name or is JSON name.
func (md *ModelData) Has(field FieldName) bool {
	if _, ok := md.FieldMap.Get(field); ok {
		return true
	}
	if _, ok := md.ToCreate[field.JSON()]; ok {
		return true
	}
	return false
}

// Set sets the given field with the given value.
// If the field already exists, then it is updated with value.
// Otherwise, a new entry is inserted.
//
// It returns the given ModelData so that calls can be chained
func (md *ModelData) Set(field FieldName, value interface{}) *ModelData {
	md.FieldMap.Set(field, value)
	return md
}

// Unset removes the value of the given field if it exists.
//
// It returns the given ModelData so that calls can be chained
func (md *ModelData) Unset(field FieldName) *ModelData {
	md.FieldMap.Delete(field)
	delete(md.ToCreate, field.JSON())
	return md
}

// Create stores the related ModelData to be used to create
// a related record on the fly and link it to this field.
//
// This method can be called multiple times to create multiple records
func (md *ModelData) Create(field FieldName, related *ModelData) *ModelData {
	fi := md.Model.getRelatedFieldInfo(field)
	if related.Model != fi.relatedModel {
		log.Panic("create data must be of the model of the relation field", "fieldModel", fi.relatedModel, "dataModel", related.Model)
	}
	md.ToCreate[field.JSON()] = append(md.ToCreate[field.JSON()], related)
	return md
}

// Copy returns a copy of this ModelData
func (md *ModelData) Copy() *ModelData {
	ntc := make(map[string][]*ModelData)
	for k, v := range md.ToCreate {
		ntc[k] = v
	}
	return &ModelData{
		Model:    md.Model,
		FieldMap: md.FieldMap.Copy(),
		ToCreate: ntc,
	}
}

// MergeWith updates this ModelData with the given other ModelData.
// If a key of the other ModelData already exists here, the value is overridden,
// otherwise, the key is inserted with its json name.
func (md *ModelData) MergeWith(other *ModelData) {
	// 1. We unset all entries existing in other to remove both FieldMap and ToCreate entries
	for field := range other.FieldMap {
		if md.Has(md.Model.FieldName(field)) {
			md.Unset(md.Model.FieldName(field))
		}
	}
	for field := range other.ToCreate {
		if md.Has(md.Model.FieldName(field)) {
			md.Unset(md.Model.FieldName(field))
		}
	}
	// 2. We set other values in md
	md.FieldMap.MergeWith(other.FieldMap, other.Model)
	for field, toCreate := range other.ToCreate {
		md.ToCreate[field] = append(md.ToCreate[field], toCreate...)
	}
}

// FieldNames returns the ModelData keys as a slice of FieldNames.
func (md *ModelData) FieldNames() FieldNames {
	return md.FieldMap.FieldNames(md.Model)
}

// MarshalJSON function for ModelData. Returns the FieldMap.
func (md *ModelData) MarshalJSON() ([]byte, error) {
	return json.Marshal(md.FieldMap)
}

// Underlying returns the ModelData
func (md *ModelData) Underlying() *ModelData {
	return md
}

// fixFieldValue changes the given value for the given field by applying several fixes
func fixFieldValue(v interface{}, fi *Field) interface{} {
	if _, ok := v.(bool); ok && fi.fieldType != fieldtype.Boolean {
		// Client returns false when empty
		v = reflect.Zero(fi.structField.Type).Interface()
	}
	if _, ok := v.([]byte); ok && fi.fieldType == fieldtype.Float {
		// DB can return numeric types as []byte
		switch fi.structField.Type.Kind() {
		case reflect.Float64:
			if res, err := strconv.ParseFloat(string(v.([]byte)), 64); err == nil {
				v = res
			}
		case reflect.Float32:
			if res, err := strconv.ParseFloat(string(v.([]byte)), 32); err == nil {
				v = float32(res)
			}
		}
	}
	if _, ok := v.(float64); ok && fi.fieldType == fieldtype.Integer {
		// JSON unmarshals int to float64. Convert back to the Go type of fi.
		val := reflect.ValueOf(v)
		typ := fi.structField.Type
		val = val.Convert(typ)
		v = val.Interface()
	}
	return v
}

// NewModelData returns a pointer to a new instance of ModelData
// for the given model. If FieldMaps are given they are added to
// the ModelData.
func NewModelData(model Modeler, fm ...FieldMap) *ModelData {
	fMap := make(FieldMap)
	for _, f := range fm {
		for k, v := range f {
			fi := model.Underlying().getRelatedFieldInfo(model.Underlying().FieldName(k))
			v = fixFieldValue(v, fi)
			fMap[fi.json] = v
		}
	}
	return &ModelData{
		FieldMap: fMap,
		ToCreate: make(map[string][]*ModelData),
		Model:    model.Underlying(),
	}
}

// NewModelDataFromRS creates a pointer to a new instance of ModelData.
// If FieldMaps are given they are added to the ModelData.
//
// Unlike NewModelData, this method translates relation fields in64 and
// []int64 values as RecordSets
func NewModelDataFromRS(rs RecordSet, fm ...FieldMap) *ModelData {
	fMap := make(FieldMap)
	for _, f := range fm {
		for k, v := range f {
			fi := rs.Collection().Model().getRelatedFieldInfo(rs.Collection().Model().FieldName(k))
			if fi.isRelationField() {
				v = rs.Collection().convertToRecordSet(v, fi.relatedModelName)
			}
			v = fixFieldValue(v, fi)
			fMap[fi.json] = v
		}
	}
	return &ModelData{
		FieldMap: fMap,
		ToCreate: make(map[string][]*ModelData),
		Model:    rs.Collection().model,
	}
}
