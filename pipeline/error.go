package pipeline

import "fmt"

type NameError struct {
	OffendingName string
	Explanation   string
}

func (ne *NameError) Error() string {
	return fmt.Sprintf("Name Error: '%s' %s", ne.OffendingName, ne.Explanation)
}
