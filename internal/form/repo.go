package form

import (
	"github.com/go-macaron/binding"
	"gopkg.in/macaron.v1"
)

type Repo struct {
	Code     string
	RepoName string
}

func (f *Repo) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

type ListRepo struct {
	Code string
}

func (f *ListRepo) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}
