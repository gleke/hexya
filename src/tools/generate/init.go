// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package generate

import (
	"github.com/gleke/hexya/src/tools/logging"
)

const (
	// HexyaPath is the go import path of the base hexya package
	HexyaPath = "github.com/gleke/hexya"
	// ModelsPath is the go import path of the hexya/models package
	ModelsPath = HexyaPath + "/src/models"
	// DatesPath is the go import path of the hexya/models/types/dates package
	DatesPath = HexyaPath + "/src/models/types/dates"
	// PoolPath is the go import path of the autogenerated pool package
	PoolPath = "github.com/gleke/pool"
	// PoolModelPackage is the name of the pool package with model data
	PoolModelPackage = "h"
	// PoolQueryPackage is the name of the pool package with query dat
	PoolQueryPackage = "q"
	// PoolInterfacesPackage is the name of the pool packages with all model interfaces
	PoolInterfacesPackage = "m"
)

var (
	log logging.Logger
	// ModelMixins are the names of the mixins declared in the models package
	ModelMixins = map[string]bool{
		"CommonMixin":    true,
		"BaseMixin":      true,
		"ModelMixin":     true,
		"TransientMixin": true,
	}
	// MethodsToAdd are methods that are declared directly in the generated code.
	// Usually this is because they can't be declared in base_model due to not convertible arg or return types.
	methodsToAdd = map[string]bool{
		"Aggregates": true,
	}
)

func init() {
	log = logging.GetLogger("tools/generate")
}
