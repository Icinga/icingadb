package types

import (
	"fmt"
	"strings"
)

// Name returns the declared name of type t.
func Name(t any) string {
	s := strings.TrimLeft(fmt.Sprintf("%T", t), "*")

	return s[strings.LastIndex(s, ".")+1:]
}
