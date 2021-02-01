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

	"github.com/gleke/hexya/src/tools/logging"
	"github.com/gleke/hexya/src/tools/strutils"
	"github.com/jmoiron/sqlx"
)

var (
	log logging.Logger
	// Views is a map to store views created automatically.
	// It will be processed by the views package and added to the views registry.
	Views map[*Model][]string
)

func init() {
	log = logging.GetLogger("models")
	sqlx.NameMapper = strutils.SnakeCase
	// DB drivers
	adapters = make(map[string]dbAdapter)
	registerDBAdapter("postgres", new(postgresAdapter))
	// model registry
	Registry = newModelCollection()
	Views = make(map[*Model][]string)
	recordSetWrappers = make(map[string]reflect.Type)
	modelDataWrappers = make(map[string]reflect.Type)
	// declare base and common mixins
	declareCommonMixin()
	declareBaseMixin()
	declareModelMixin()
}
