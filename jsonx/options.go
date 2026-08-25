package jsonx

type Options struct {
	MaxDepth            int
	EscapeHTML          bool
	SortKeys            bool
	DisallowUnknown     bool
	RequireFields       bool
	NoCoerce            bool
	AllowComments       bool
	NumberAsFloat64     bool
	RejectDuplicateKeys bool
}
type Option func(*Options)

func defaults() Options { return Options{MaxDepth: 512, EscapeHTML: true, NumberAsFloat64: true} }
func resolve(options []Option) Options {
	o := defaults()
	for _, apply := range options {
		if apply != nil {
			apply(&o)
		}
	}
	if o.MaxDepth <= 0 {
		o.MaxDepth = 512
	}
	return o
}

var sharedOptions = defaults()

func resolveShared(options []Option) Options {
	for _, apply := range options {
		if apply != nil {
			apply(&sharedOptions)
		}
	}
	return sharedOptions
}
func WithOptions(value Options) Option {
	return func(o *Options) {
		*o = value
		if o.MaxDepth <= 0 {
			o.MaxDepth = 512
		}
	}
}
func MaxDepth(n int) Option               { return func(o *Options) { o.MaxDepth = n } }
func EscapeHTML(enabled bool) Option      { return func(o *Options) { o.EscapeHTML = enabled } }
func SortKeys(enabled bool) Option        { return func(o *Options) { o.SortKeys = enabled } }
func DisallowUnknown(enabled bool) Option { return func(o *Options) { o.DisallowUnknown = enabled } }
func RequireFields(enabled bool) Option   { return func(o *Options) { o.RequireFields = enabled } }
func NoCoerce(enabled bool) Option        { return func(o *Options) { o.NoCoerce = enabled } }
func AllowComments(enabled bool) Option   { return func(o *Options) { o.AllowComments = enabled } }
func NumberAsFloat64(enabled bool) Option { return func(o *Options) { o.NumberAsFloat64 = enabled } }
func RejectDuplicateKeys(enabled bool) Option {
	return func(o *Options) { o.RejectDuplicateKeys = enabled }
}
