package validator

import (
	"reflect"

	"github.com/go-playground/validator/v10"
	"github.com/shopspring/decimal"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	// Allow decimal.Decimal to be used with standard validation tags (gt, gte, lt, lte, required).
	// The custom type func converts decimal to float64 so the built-in comparators apply.
	validate.RegisterCustomTypeFunc(func(field reflect.Value) interface{} {
		if val, ok := field.Interface().(decimal.Decimal); ok {
			f, _ := val.Float64()
			return f
		}
		return nil
	}, decimal.Decimal{})
}

func Validate(data interface{}) error {
	return validate.Struct(data)
}

func GetValidator() *validator.Validate {
	return validate
}
