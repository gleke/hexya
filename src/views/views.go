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

package views

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/beevik/etree"
	"github.com/hexya-erp/hexya/src/i18n"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/tools/xmlutils"
)

// A ViewType defines the type of a view
type ViewType string

// View types
const (
	ViewTypeTree     ViewType = "tree"
	ViewTypeList     ViewType = "list"
	ViewTypeForm     ViewType = "form"
	ViewTypeGraph    ViewType = "graph"
	ViewTypeCalendar ViewType = "calendar"
	ViewTypeDiagram  ViewType = "diagram"
	ViewTypeGantt    ViewType = "gantt"
	ViewTypeKanban   ViewType = "kanban"
	ViewTypeSearch   ViewType = "search"
	ViewTypeQWeb     ViewType = "qweb"
)

// translatableAttributes is the list of XML attribute names the
// value of which needs to be translated.
var translatableAttributes = []string{"string", "help", "sum", "confirm", "placeholder"}

// Registry is the views collection of the application
var Registry *Collection

// MakeViewRef creates a ViewRef from a view id
func MakeViewRef(id string) ViewRef {
	view := Registry.GetByID(id)
	if view == nil {
		return ViewRef{}
	}
	return ViewRef{id, view.Name}
}

// ViewRef is an array of two strings representing a view:
// - The first one is the ID of the view
// - The second one is the name of the view
type ViewRef [2]string

// MarshalJSON is the JSON marshalling method of ViewRef.
// It marshals empty ViewRef into null instead of ["", ""].
func (vr ViewRef) MarshalJSON() ([]byte, error) {
	if vr[0] == "" {
		return json.Marshal(nil)
	}
	return json.Marshal([2]string{vr[0], vr[1]})
}

// UnmarshalJSON is the JSON unmarshalling method of ViewRef.
// It unmarshals null into an empty ViewRef.
func (vr *ViewRef) UnmarshalJSON(data []byte) error {
	var dst interface{}
	if err := json.Unmarshal(data, &dst); err == nil && dst == nil {
		*vr = ViewRef{"", ""}
		return nil
	}
	var dstArray [2]string
	if err := json.Unmarshal(data, &dstArray); err != nil {
		return err
	}
	*vr = dstArray
	return nil
}

// UnmarshalXMLAttr is the XML unmarshalling method of ViewRef.
// It unmarshals null into an empty ViewRef.
func (vr *ViewRef) UnmarshalXMLAttr(attr xml.Attr) error {
	*vr = MakeViewRef(attr.Value)
	return nil
}

// Value extracts ID of our ViewRef for storing in the database.
func (vr ViewRef) Value() (driver.Value, error) {
	return driver.Value(vr[0]), nil
}

// Scan fetches the name of our view from the ID
// stored in the database to fill the ViewRef.
func (vr *ViewRef) Scan(src interface{}) error {
	var source string
	switch s := src.(type) {
	case string:
		source = s
	case []byte:
		source = string(s)
	default:
		return fmt.Errorf("invalid type for ViewRef: %T", src)
	}
	*vr = MakeViewRef(source)
	return nil
}

// ID returns the ID of the current view reference
func (vr ViewRef) ID() string {
	return vr[0]
}

// Name returns the name of the current view reference
func (vr ViewRef) Name() string {
	return vr[1]
}

// IsNull returns true if this ViewRef references no view
func (vr ViewRef) IsNull() bool {
	return vr[0] == "" && vr[1] == ""
}

var _ driver.Valuer = &ViewRef{}
var _ sql.Scanner = &ViewRef{}
var _ json.Marshaler = &ViewRef{}
var _ json.Unmarshaler = &ViewRef{}
var _ xml.UnmarshalerAttr = &ViewRef{}

// ViewTuple is an array of two strings representing a view:
// - The first one is the ID of the view
// - The second one is the view type corresponding to the view ID
type ViewTuple struct {
	ID   string   `xml:"id,attr"`
	Type ViewType `xml:"type,attr"`
}

// MarshalJSON is the JSON marshalling method of ViewTuple.
// It marshals ViewTuple into a list [id, type].
func (vt ViewTuple) MarshalJSON() ([]byte, error) {
	return json.Marshal([2]string{vt.ID, string(vt.Type)})
}

// UnmarshalJSON method for ViewTuple
func (vt *ViewTuple) UnmarshalJSON(data []byte) error {
	var src interface{}
	err := json.Unmarshal(data, &src)
	if err != nil {
		return err
	}
	switch s := src.(type) {
	case []interface{}:
		vID, _ := s[0].(string)
		vt.ID = vID
		vt.Type = ViewType(s[1].(string))
	default:
		return errors.New("unexpected type in ViewTuple Unmarshal")
	}
	return nil
}

var _ json.Marshaler = ViewTuple{}
var _ json.Unmarshaler = &ViewTuple{}

// A Collection is a view collection
type Collection struct {
	sync.RWMutex
	views             map[string]*View
	orderedViews      map[string][]*View
	rawInheritedViews []*ViewXML
}

// NewCollection returns a pointer to a new
// Collection instance
func NewCollection() *Collection {
	res := Collection{
		views:        make(map[string]*View),
		orderedViews: make(map[string][]*View),
	}
	return &res
}

// Add adds the given view to our Collection
func (vc *Collection) Add(v *View) {
	vc.Lock()
	defer vc.Unlock()
	var index int8
	for i, view := range vc.orderedViews[v.Model] {
		index = int8(i)
		if view.Priority > v.Priority {
			break
		}
	}
	vc.views[v.ID] = v
	if index == int8(len(vc.orderedViews)-1) {
		vc.orderedViews[v.Model] = append(vc.orderedViews[v.Model], v)
		return
	}
	endElems := make([]*View, len(vc.orderedViews[v.Model][index:]))
	copy(endElems, vc.orderedViews[v.Model][index:])
	vc.orderedViews[v.Model] = append(append(vc.orderedViews[v.Model][:index], v), endElems...)
}

// GetByID returns the View with the given id
func (vc *Collection) GetByID(id string) *View {
	return vc.views[id]
}

// GetAll returns a list of all views of this Collection.
// Views are returned in an arbitrary order
func (vc *Collection) GetAll() []*View {
	res := make([]*View, len(vc.views))
	var i int
	for _, view := range vc.views {
		res[i] = view
		i++
	}
	return res
}

// GetFirstViewForModel returns the first view of type viewType for the given model
func (vc *Collection) GetFirstViewForModel(model string, viewType ViewType) *View {
	for _, view := range vc.orderedViews[model] {
		if view.Type == viewType {
			return view
		}
	}
	return vc.defaultViewForModel(model, viewType)
}

// defaultViewForModel returns a default view for the given model and type
func (vc *Collection) defaultViewForModel(model string, viewType ViewType) *View {
	xmlStr := fmt.Sprintf(`<%s></%s>`, viewType, viewType)
	arch, err := xmlutils.XMLToDocument(xmlStr)
	if err != nil {
		log.Panic("unable to create default view", "error", err, "view", xmlStr)
	}
	view := View{
		Model:  model,
		Type:   viewType,
		Fields: []string{},
		arch:   arch,
		arches: make(map[string]*etree.Document),
	}
	if _, ok := models.Registry.MustGet(model).Fields().Get("name"); ok {
		xmlStr = fmt.Sprintf(`<%s><field name="name"/></%s>`, viewType, viewType)
		arch, err = xmlutils.XMLToDocument(xmlStr)
		if err != nil {
			log.Panic("unable to create default view", "error", err, "view", xmlStr)
		}
		view.Fields = []string{"name"}
		view.arch = arch
	}
	view.translateArch()
	return &view
}

// GetAllViewsForModel returns a list with all views for the given model
func (vc *Collection) GetAllViewsForModel(model string) []*View {
	var res []*View
	for _, view := range vc.views {
		if view.Model == model {
			res = append(res, view)
		}
	}
	return res
}

// LoadFromEtree loads the given view given as Element
// into this collection.
func (vc *Collection) LoadFromEtree(element *etree.Element) {
	xmlBytes, err := xmlutils.ElementToXML(element)
	if err != nil {
		log.Panic("Unable to convert element to XML", "error", err)
	}
	var viewXML ViewXML
	if err = xml.Unmarshal(xmlBytes, &viewXML); err != nil {
		log.Panic("Unable to unmarshal element", "error", err, "bytes", string(xmlBytes))
	}
	if viewXML.InheritID != "" {
		// Update an existing view.
		// Put in raw inherited view for now, as the base view may not exist yet.
		vc.rawInheritedViews = append(vc.rawInheritedViews, &viewXML)
		return
	}
	// Create a new view
	vc.createNewViewFromXML(&viewXML)
}

// createNewViewFromXML creates and register a new view with the given XML
func (vc *Collection) createNewViewFromXML(viewXML *ViewXML) {
	priority := uint8(16)
	if viewXML.Priority != 0 {
		priority = viewXML.Priority
	}
	name := strings.Replace(viewXML.ID, "_", ".", -1)
	if viewXML.Name != "" {
		name = viewXML.Name
	}

	arch, err := xmlutils.XMLToDocument(viewXML.Arch)
	if err != nil {
		log.Panic("unable to create view from XML", "error", err, "view", viewXML.Arch)
	}
	view := View{
		ID:          viewXML.ID,
		Name:        name,
		Model:       viewXML.Model,
		Priority:    priority,
		arch:        arch,
		FieldParent: viewXML.FieldParent,
		SubViews:    make(map[string]SubViews),
		arches:      make(map[string]*etree.Document),
	}
	vc.Add(&view)
}

// View is the internal definition of a view in the application
type View struct {
	ID          string
	Name        string
	Model       string
	Type        ViewType
	Priority    uint8
	arch        *etree.Document
	FieldParent string
	Fields      []string
	SubViews    map[string]SubViews
	arches      map[string]*etree.Document
}

// A SubViews is a holder for embedded views of a field
type SubViews map[ViewType]*View

// populateFieldNames scans arch, extract field names and put them in the fields slice
func (v *View) populateFieldNames() {
	fieldElems := v.arch.FindElements("//field")
	for _, f := range fieldElems {
		v.Fields = append(v.Fields, f.SelectAttr("name").Value)
	}
}

// Arch returns the arch XML string of this view for the given language.
// Call with empty string to get the default language's arch
func (v *View) Arch(lang string) *etree.Document {
	res, ok := v.arches[lang]
	if !ok || lang == "" {
		res = v.arch
	}
	return res
}

// setViewType sets the Type field with the view type
// scanned from arch
func (v *View) setViewType() {
	v.Type = ViewType(v.arch.Root().Tag)
}

// extractSubViews recursively scans arch for embedded views,
// extract them from arch and add them to SubViews.
func (v *View) extractSubViews(model *models.Model, fInfos map[string]*models.FieldInfo) {
	archElem := v.arch.Copy()
	fieldElems := archElem.FindElements("//field")
	for _, f := range fieldElems {
		if xmlutils.HasParentTag(f, "field") {
			// Discard fields of embedded views
			continue
		}
		fieldName := f.SelectAttr("name").Value
		for i, childElement := range f.ChildElements() {
			if _, exists := v.SubViews[fieldName]; !exists {
				v.SubViews[fieldName] = make(SubViews)
			}
			childDoc := etree.NewDocument()
			childDoc.SetRoot(xmlutils.CopyElement(childElement))
			childView := View{
				ID:       fmt.Sprintf("%s_childview_%s_%d", v.ID, fieldName, i),
				arch:     childDoc,
				SubViews: make(map[string]SubViews),
				arches:   make(map[string]*etree.Document),
				Model:    fInfos[model.JSONizeFieldName(fieldName)].Relation,
			}
			childView.postProcess()
			v.SubViews[fieldName][childView.Type] = &childView
		}
		// Remove all children elements.
		// We do it in a separate loop on tokens to remove text and comments too.
		numChild := len(f.Child)
		for j := 0; j < numChild; j++ {
			f.RemoveChild(f.Child[0])
		}
	}
	v.arch = archElem
}

// postProcess executes all actions that are needed the view for bootstrapping
func (v *View) postProcess() {
	model := models.Registry.MustGet(v.Model)
	fInfos := model.FieldsGet()

	v.setViewType()
	v.extractSubViews(model, fInfos)
	v.updateFieldNames(model)
	v.populateFieldNames()
	v.AddOnchanges(fInfos)
	v.SanitizeSearchView()
	v.translateArch()
}

// UpdateFieldNames changes the field names in the view to the column names.
// If a field name is already column names then it does nothing.
func (v *View) updateFieldNames(model *models.Model) {
	for _, fieldTag := range v.arch.FindElements("//field") {
		if xmlutils.HasParentTag(fieldTag, "field") {
			// Discard fields of embedded views
			continue
		}
		fieldName := fieldTag.SelectAttr("name").Value
		fieldJSON := model.JSONizeFieldName(fieldName)
		fieldTag.RemoveAttr("name")
		fieldTag.CreateAttr("name", fieldJSON)
	}
	for _, labelTag := range v.arch.FindElements("//label") {
		if labelTag.SelectAttr("for") == nil || xmlutils.HasParentTag(labelTag, "field") {
			continue
		}
		fieldName := labelTag.SelectAttr("for").Value
		fieldJSON := model.JSONizeFieldName(fieldName)
		labelTag.RemoveAttr("for")
		labelTag.CreateAttr("for", fieldJSON)
	}
}

// AddOnchanges adds onchange=1 for each field in the view which has an OnChange
// method defined
func (v *View) AddOnchanges(fInfos map[string]*models.FieldInfo) {
	for fieldName, fInfo := range fInfos {
		if !fInfo.OnChange {
			continue
		}
		for _, elt := range v.arch.FindElements(fmt.Sprintf("//field[@name='%s']", fieldName)) {
			if elt.SelectAttr("on_change") == nil {
				elt.CreateAttr("on_change", "1")
			}
		}
	}
}

// SanitizeSearchView adds the missing domain attribute if it does not exist
func (v *View) SanitizeSearchView() {
	if v.Type != ViewTypeSearch {
		return
	}
	for _, fieldTag := range v.arch.FindElements("//field") {
		if fieldTag.SelectAttrValue("domain", "") == "" {
			fieldTag.CreateAttr("domain", "[]")
		}
	}
}

// translateArch populates the arches map with all the translations
func (v *View) translateArch() {
	labels := v.TranslatableStrings()
	for _, lang := range i18n.Langs {
		tArchElt := v.arch.Copy()
		for _, label := range labels {
			attrElts := tArchElt.FindElements(fmt.Sprintf("//[@%s]", label.Attribute))
			for i, attrElt := range attrElts {
				if attrElt.SelectAttrValue(label.Attribute, "") != label.Value {
					continue
				}
				transLabel := i18n.TranslateResourceItem(lang, v.ID, label.Value)
				attrElts[i].RemoveAttr(label.Attribute)
				attrElts[i].CreateAttr(label.Attribute, transLabel)
			}
		}
		v.arches[lang] = tArchElt
	}
}

// updateViewFromXML updates this view with the given XML
// viewXML must have an InheritID
func (v *View) updateViewFromXML(viewXML *ViewXML) {
	specDoc, err := xmlutils.XMLToDocument(viewXML.Arch)
	if err != nil {
		log.Panic("Unable to read inheritance specs", "error", err, "arch", viewXML.Arch)
	}
	newArch, err := xmlutils.ApplyExtensions(v.arch, specDoc)
	if err != nil {
		log.Panic("Error while applying view extension specs", "error", err, "specView", viewXML.ID, "specs", viewXML.Arch, "view", v.ID, "arch", v.arch)
	}
	v.arch = newArch
}

// A TranslatableAttribute is a reference to an attribute in a
// XML view definition that can be translated.
type TranslatableAttribute struct {
	Attribute string
	Value     string
}

// TranslatableStrings returns the list of all the strings in the
// view arch that must be translated.
func (v *View) TranslatableStrings() []TranslatableAttribute {
	var labels []TranslatableAttribute
	for _, tagName := range translatableAttributes {
		elts := v.arch.FindElements(fmt.Sprintf("[@%s]", tagName))
		for _, elt := range elts {
			label := elt.SelectAttrValue(tagName, "")
			if label == "" {
				continue
			}
			labels = append(labels, TranslatableAttribute{Attribute: tagName, Value: label})
		}
	}
	return labels
}

// ViewXML is used to unmarshal the XML definition of a View
type ViewXML struct {
	ID          string `xml:"id,attr"`
	Name        string `xml:"name,attr"`
	Model       string `xml:"model,attr"`
	Priority    uint8  `xml:"priority,attr"`
	Arch        string `xml:",innerxml"`
	InheritID   string `xml:"inherit_id,attr"`
	FieldParent string `xml:"field_parent,attr"`
}

// LoadFromEtree reads the view given etree.Element, creates or updates the view
// and adds it to the view registry if it not already.
func LoadFromEtree(element *etree.Element) {
	Registry.LoadFromEtree(element)
}
