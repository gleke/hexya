// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package actions

import (
	"github.com/gleke/hexya/src/i18n"
	"github.com/gleke/hexya/src/tools/logging"
)

var log logging.Logger

// BootStrap actions.
// This function must be called prior to any access to the actions Registry.
func BootStrap() {
	for _, a := range Registry.actions {
		a.Sanitize()
		// Populate translations
		if a.names == nil {
			a.names = make(map[string]string)
		}
		for _, lang := range i18n.Langs {
			nameTrans := i18n.TranslateResourceItem(lang, a.XMLID, a.Name)
			a.names[lang] = nameTrans
		}
	}
}

func init() {
	log = logging.GetLogger("actions")
	Registry = NewCollection()
}
