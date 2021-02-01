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
	"fmt"
	"reflect"

	"github.com/gleke/hexya/src/models/operator"
)

// Expression separation symbols
const (
	ExprSep    = "."
	sqlSep     = "__"
	ContextSep = "|"
)

// A predicate of a condition in the form 'Field = arg'
type predicate struct {
	exprs    []FieldName
	operator operator.Operator
	arg      interface{}
	cond     *Condition
	isOr     bool
	isNot    bool
	isCond   bool
}

// Field returns the field name of this predicate
func (p predicate) Field() FieldName {
	return joinFieldNames(p.exprs, ExprSep)
}

// Operator returns the operator of this predicate
func (p predicate) Operator() operator.Operator {
	return p.operator
}

// Argument returns the argument of this predicate
func (p predicate) Argument() interface{} {
	return p.arg
}

// AlterField changes the field of this predicate
func (p *predicate) AlterField(f FieldName) *predicate {
	if f == nil || f.Name() == "" {
		log.Panic("AlterField must be called with a field name", "field", f)
	}
	p.exprs = splitFieldNames(f, ExprSep)
	return p
}

// AlterOperator changes the operator of this predicate
func (p *predicate) AlterOperator(op operator.Operator) *predicate {
	p.operator = op
	return p
}

// AlterArgument changes the argument of this predicate
func (p *predicate) AlterArgument(arg interface{}) *predicate {
	p.arg = arg
	return p
}

// A Condition represents a WHERE clause of an SQL query.
type Condition struct {
	predicates []predicate
}

// newCondition returns a new condition struct
func newCondition() *Condition {
	c := &Condition{}
	return c
}

// And completes the current condition with a simple AND clause : c.And().nextCond => c AND nextCond.
//
// No brackets are added so AND precedence over OR applies.
func (c Condition) And() *ConditionStart {
	res := ConditionStart{cond: c}
	return &res
}

// AndCond completes the current condition with the given cond as an AND clause
// between brackets : c.And(cond) => (c) AND (cond)
func (c Condition) AndCond(cond *Condition) *Condition {
	if !cond.IsEmpty() {
		c.predicates = append(c.predicates, predicate{cond: cond, isCond: true})
	}
	return &c
}

// AndNot completes the current condition with a simple AND NOT clause :
// c.AndNot().nextCond => c AND NOT nextCond
//
// No brackets are added so AND precedence over OR applies.
func (c Condition) AndNot() *ConditionStart {
	res := ConditionStart{cond: c}
	res.nextIsNot = true
	return &res
}

// AndNotCond completes the current condition with an AND NOT clause between
// brackets : c.AndNot(cond) => (c) AND NOT (cond)
func (c Condition) AndNotCond(cond *Condition) *Condition {
	if !cond.IsEmpty() {
		c.predicates = append(c.predicates, predicate{cond: cond, isCond: true, isNot: true})
	}
	return &c
}

// Or completes the current condition both with a simple OR clause : c.Or().nextCond => c OR nextCond
//
// No brackets are added so AND precedence over OR applies.
func (c Condition) Or() *ConditionStart {
	res := ConditionStart{cond: c}
	res.nextIsOr = true
	return &res
}

// OrCond completes the current condition both with an OR clause between
// brackets : c.Or(cond) => (c) OR (cond)
func (c Condition) OrCond(cond *Condition) *Condition {
	if !cond.IsEmpty() {
		c.predicates = append(c.predicates, predicate{cond: cond, isCond: true, isOr: true})
	}
	return &c
}

// OrNot completes the current condition both with a simple OR NOT clause : c.OrNot().nextCond => c OR NOT nextCond
//
// No brackets are added so AND precedence over OR applies.
func (c Condition) OrNot() *ConditionStart {
	res := ConditionStart{cond: c}
	res.nextIsNot = true
	res.nextIsOr = true
	return &res
}

// OrNotCond completes the current condition both with an OR NOT clause between
// brackets : c.OrNot(cond) => (c) OR NOT (cond)
func (c Condition) OrNotCond(cond *Condition) *Condition {
	if !cond.IsEmpty() {
		c.predicates = append(c.predicates, predicate{cond: cond, isCond: true, isOr: true, isNot: true})
	}
	return &c
}

// Serialize returns the condition as a list which mimics Odoo domains.
func (c Condition) Serialize() []interface{} {
	return serializePredicates(c.predicates)
}

// HasField returns true if the given field is in at least one of the
// the predicates of this condition or of one of its nested conditions.
func (c Condition) HasField(f *Field) bool {
	preds := c.PredicatesWithField(f)
	return len(preds) > 0
}

// PredicatesWithField returns all predicates of this condition (including
// nested conditions) that concern the given field.
func (c Condition) PredicatesWithField(f *Field) []*predicate {
	var res []*predicate
	for i, pred := range c.predicates {
		if len(pred.exprs) > 0 {
			if joinFieldNames(pred.exprs, ExprSep).JSON() == f.json {
				res = append(res, &c.predicates[i])
			}
		}
		if pred.cond != nil {
			res = append(res, c.predicates[i].cond.PredicatesWithField(f)...)
		}
	}
	return res
}

// String method for the Condition. Recursively print all predicates.
func (c Condition) String() string {
	var res string
	for _, p := range c.predicates {
		if p.isOr {
			res += "OR "
		} else {
			res += "AND "
		}
		if p.isNot {
			res += "NOT "
		}
		if p.isCond {
			res += fmt.Sprintf("(\n%s\n)\n", p.cond.String())
			continue
		}
		res += fmt.Sprintf("%s %s %v\n", joinFieldNames(p.exprs, ExprSep).Name(), p.operator, p.arg)
	}
	return res
}

// Underlying returns the underlying Condition (i.e. itself)
func (c Condition) Underlying() *Condition {
	return &c
}

var _ Conditioner = Condition{}

// A ConditionStart is an object representing a Condition when
// we just added a logical operator (AND, OR, ...) and we are
// about to add a predicate.
type ConditionStart struct {
	cond      Condition
	nextIsOr  bool
	nextIsNot bool
}

// Field adds a field path (dot separated) to this condition
func (cs ConditionStart) Field(name FieldName) *ConditionField {
	newExprs := splitFieldNames(name, ExprSep)
	cp := ConditionField{cs: cs}
	cp.exprs = append(cp.exprs, newExprs...)
	return &cp
}

// FilteredOn adds a condition with a table join on the given field and
// filters the result with the given condition
func (cs ConditionStart) FilteredOn(field FieldName, condition *Condition) *Condition {
	res := cs.cond
	for i, p := range condition.predicates {
		condition.predicates[i].exprs = append([]FieldName{field}, p.exprs...)
	}
	condition.predicates[0].isOr = cs.nextIsOr
	condition.predicates[0].isNot = cs.nextIsNot
	res.predicates = append(res.predicates, condition.predicates...)
	return &res
}

// A ConditionField is a partial Condition when we have set
// a field name in a predicate and are about to add an operator.
type ConditionField struct {
	cs    ConditionStart
	exprs []FieldName
}

// JSON returns the json field name of this ConditionField
func (c ConditionField) JSON() string {
	return joinFieldNames(c.exprs, ExprSep).JSON()
}

// Name method for ConditionField
func (c ConditionField) Name() string {
	return joinFieldNames(c.exprs, ExprSep).Name()
}

var _ FieldName = ConditionField{}

// AddOperator adds a condition value to the condition with the given operator and data
// If multi is true, a recordset will be converted into a slice of int64
// otherwise, it will return an int64 and panic if the recordset is not
// a singleton.
//
// This method is low level and should be avoided. Use operator methods such as Equals()
// instead.
func (c ConditionField) AddOperator(op operator.Operator, data interface{}) *Condition {
	cond := c.cs.cond
	data = sanitizeArgs(data, op.IsMulti())
	if data != nil && op.IsMulti() && reflect.ValueOf(data).Kind() == reflect.Slice && reflect.ValueOf(data).Len() == 0 {
		// field in [] => ID = -1
		cond.predicates = []predicate{{
			exprs:    []FieldName{ID},
			operator: operator.Equals,
			arg:      -1,
		}}
		return &cond
	}
	cond.predicates = append(cond.predicates, predicate{
		exprs:    c.exprs,
		operator: op,
		arg:      data,
		isNot:    c.cs.nextIsNot,
		isOr:     c.cs.nextIsOr,
	})
	return &cond
}

// sanitizeArgs returns the given args suitable for SQL query
// In particular, retrieves the ids of a recordset if args is one.
// If multi is true, a recordset will be converted into a slice of int64
// otherwise, it will return an int64 and panic if the recordset is not
// a singleton
func sanitizeArgs(args interface{}, multi bool) interface{} {
	if rs, ok := args.(RecordSet); ok {
		if multi {
			return rs.Ids()
		}
		if len(rs.Ids()) > 1 {
			log.Panic("Trying to extract a single ID from a non singleton", "args", args)
		}
		if len(rs.Ids()) == 0 {
			return nil
		}
		return rs.Ids()[0]
	}
	return args
}

// Equals appends the '=' operator to the current Condition
func (c ConditionField) Equals(data interface{}) *Condition {
	return c.AddOperator(operator.Equals, data)
}

// NotEquals appends the '!=' operator to the current Condition
func (c ConditionField) NotEquals(data interface{}) *Condition {
	return c.AddOperator(operator.NotEquals, data)
}

// Greater appends the '>' operator to the current Condition
func (c ConditionField) Greater(data interface{}) *Condition {
	return c.AddOperator(operator.Greater, data)
}

// GreaterOrEqual appends the '>=' operator to the current Condition
func (c ConditionField) GreaterOrEqual(data interface{}) *Condition {
	return c.AddOperator(operator.GreaterOrEqual, data)
}

// Lower appends the '<' operator to the current Condition
func (c ConditionField) Lower(data interface{}) *Condition {
	return c.AddOperator(operator.Lower, data)
}

// LowerOrEqual appends the '<=' operator to the current Condition
func (c ConditionField) LowerOrEqual(data interface{}) *Condition {
	return c.AddOperator(operator.LowerOrEqual, data)
}

// Like appends the 'LIKE' operator to the current Condition
func (c ConditionField) Like(data interface{}) *Condition {
	return c.AddOperator(operator.Like, data)
}

// ILike appends the 'ILIKE' operator to the current Condition
func (c ConditionField) ILike(data interface{}) *Condition {
	return c.AddOperator(operator.ILike, data)
}

// Contains appends the 'LIKE %%' operator to the current Condition
func (c ConditionField) Contains(data interface{}) *Condition {
	return c.AddOperator(operator.Contains, data)
}

// NotContains appends the 'NOT LIKE %%' operator to the current Condition
func (c ConditionField) NotContains(data interface{}) *Condition {
	return c.AddOperator(operator.NotContains, data)
}

// IContains appends the 'ILIKE %%' operator to the current Condition
func (c ConditionField) IContains(data interface{}) *Condition {
	return c.AddOperator(operator.IContains, data)
}

// NotIContains appends the 'NOT ILIKE %%' operator to the current Condition
func (c ConditionField) NotIContains(data interface{}) *Condition {
	return c.AddOperator(operator.NotIContains, data)
}

// In appends the 'IN' operator to the current Condition
func (c ConditionField) In(data interface{}) *Condition {
	return c.AddOperator(operator.In, data)
}

// NotIn appends the 'NOT IN' operator to the current Condition
func (c ConditionField) NotIn(data interface{}) *Condition {
	return c.AddOperator(operator.NotIn, data)
}

// ChildOf appends the 'child of' operator to the current Condition
func (c ConditionField) ChildOf(data interface{}) *Condition {
	return c.AddOperator(operator.ChildOf, data)
}

// IsNull checks if the current condition field is null
func (c ConditionField) IsNull() *Condition {
	return c.AddOperator(operator.Equals, nil)
}

// IsNotNull checks if the current condition field is not null
func (c ConditionField) IsNotNull() *Condition {
	return c.AddOperator(operator.NotEquals, nil)
}

// IsEmpty check the condition arguments are empty or not.
func (c *Condition) IsEmpty() bool {
	switch {
	case c == nil:
		return true
	case len(c.predicates) == 0:
		return true
	case len(c.predicates) == 1 && c.predicates[0].cond != nil && c.predicates[0].cond.IsEmpty():
		return true
	}
	return false
}

// getAllExpressions returns a list of all exprs used in this condition,
// and recursively in all subconditions.
// Expressions are given in field json format
func (c Condition) getAllExpressions(mi *Model) [][]FieldName {
	var res [][]FieldName
	for _, p := range c.predicates {
		res = append(res, p.exprs)
		if p.cond != nil {
			res = append(res, p.cond.getAllExpressions(mi)...)
		}
	}
	return res
}

// substituteExprs recursively replaces condition exprs that match substs keys
// with the corresponding substs values.
func (c *Condition) substituteExprs(mi *Model, substs map[FieldName][]FieldName) {
	for i, p := range c.predicates {
		for k, v := range substs {
			if len(p.exprs) > 0 && joinFieldNames(p.exprs, ExprSep) == k {
				c.predicates[i].exprs = v
			}
		}
		if p.cond != nil {
			p.cond.substituteExprs(mi, substs)
		}
	}
}

// substituteChildOfOperator recursively replaces in the condition the
// predicates with ChildOf operator by the predicates to actually execute.
func (c *Condition) substituteChildOfOperator(rc *RecordCollection) {
	for i, p := range c.predicates {
		if p.cond != nil {
			p.cond.substituteChildOfOperator(rc)
		}
		if p.operator != operator.ChildOf {
			continue
		}
		recModel := rc.model.getRelatedModelInfo(joinFieldNames(p.exprs, ExprSep))
		if !recModel.hasParentField() {
			// If we have no parent field, then we fetch only the "parent" record
			c.predicates[i].operator = operator.Equals
			continue
		}
		var parentIds []int64
		rc.Env().Cr().Select(&parentIds, adapters[db.DriverName()].childrenIdsQuery(recModel.tableName), p.arg)
		c.predicates[i].operator = operator.In
		c.predicates[i].arg = parentIds
	}
}

// A ClientEvaluatedString is a string that contains code that will be evaluated by the client
type ClientEvaluatedString string
