package make

import (
	// alpha-build machinery imports non-go code to create standard build targets.
	// This is required by go mod to act as a dependency magnet.
	_ "github.com/openshift/library-go/alpha-build-machinery"
)
