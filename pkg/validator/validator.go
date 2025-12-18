package validator

import (
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

func Validate(data interface{}) error {
	return validate.Struct(data)
}

func GetValidator() *validator.Validate {
	return validate
}
