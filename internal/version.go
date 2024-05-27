package internal

import (
	"github.com/icinga/icinga-go-library/version"
)

// Version contains version and Git commit information.
//
// The placeholders are replaced on `git archive` using the `export-subst` attribute.
var Version = version.Version("1.2.0", "$Format:%(describe)$", "$Format:%H$")
