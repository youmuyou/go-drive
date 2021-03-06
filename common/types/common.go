package types

import "context"

type M map[string]interface{}
type SM map[string]string

type TaskCtx interface {
	context.Context
	Progress(loaded int64, abs bool)
	Total(total int64, abs bool)
	Canceled() bool
}

type IDisposable interface {
	Dispose() error
}

type IStatistics interface {
	// Status returns the name, status of this component
	Status() (string, SM, error)
}

type FormItemOption struct {
	Name     string `json:"name" i18n:""`
	Title    string `json:"title" i18n:""`
	Value    string `json:"value"`
	Disabled bool   `json:"disabled"`
}

type FormItem struct {
	Label        string           `json:"label" i18n:""`
	Type         string           `json:"type"`
	Field        string           `json:"field"`
	Required     bool             `json:"required"`
	Description  string           `json:"description" i18n:""`
	Disabled     bool             `json:"disabled"`
	Options      []FormItemOption `json:"options"`
	DefaultValue string           `json:"default_value"`
}
