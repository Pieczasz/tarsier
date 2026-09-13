package resource

type Resource struct{}

func New(ctx interface{}, opts ...interface{}) (*Resource, error) { return &Resource{}, nil }

func WithAttributes(args ...interface{}) interface{} { return nil }

func WithFromEnv() interface{} { return nil }

func ServiceName(s string) interface{} { return s }
