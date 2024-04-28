package multierr

import (
	"fmt"
	"strings"
)

type MultiErr struct {
	errors []error
}

func NewMultiErr() *MultiErr {
	return &MultiErr{}
}

func (m *MultiErr) ContainsError() bool {
	return m.errors != nil && len(m.errors) > 0
}

func (m *MultiErr) Add(err error) *MultiErr {
	if err != nil {
		m.errors = append(m.errors, err)
	}
	return m
}

func (m *MultiErr) AddAll(errs ...error) *MultiErr {
	for _, err := range errs {
		m.Add(err)
	}
	return m
}

func (m *MultiErr) AddAllFromMultiErr(m2 *MultiErr) *MultiErr {
	if m2 != nil {
		m.AddAll(m2.errors...)
	}
	return m
}

func (m *MultiErr) ToError() error {
	if m.ContainsError() {
		var errMessages []string
		for _, err := range m.errors {
			errMessages = append(errMessages, err.Error())
		}
		return fmt.Errorf(strings.Join(errMessages, "; "))
	}
	return nil
}

func MergeErrors(errs ...error) error {
	if errs == nil {
		return nil
	}
	errMessages := make([]string, 0)
	for _, err := range errs {
		if err != nil {
			errMessages = append(errMessages, fmt.Sprintf("%s", err))
		}
	}
	if len(errMessages) > 0 {
		return fmt.Errorf(strings.Join(errMessages, "; "))
	}
	return nil
}
