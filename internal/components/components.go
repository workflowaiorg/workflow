// Package components lists the components wf carries, each in a package of its own.
package components

import (
	"github.com/workflowaiorg/workflow/internal/component"
	"github.com/workflowaiorg/workflow/internal/components/backend"
)

// All are the components wf add offers, in the order it lists them.
var All = []component.Component{backend.Component}
