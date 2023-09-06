package form

import (
	"github.com/go-macaron/binding"
	"gopkg.in/macaron.v1"
)

type Repo struct {
	Code     string
	RepoName string
}

type Upload struct {
	UUID string
	Name string
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

type EditRepoFile struct {
	TreePath      string `binding:"Required;MaxSize(500)"`
	Content       string `binding:"Required"`
	CommitSummary string `binding:"MaxSize(100)"`
	CommitMessage string
	CommitChoice  string `binding:"Required;MaxSize(50)"`
	NewBranchName string `binding:"AlphaDashDotSlash;MaxSize(100)"`
	LastCommit    string
}

func (f *EditRepoFile) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

func (f *EditRepoFile) IsNewBrnach() bool {
	return f.CommitChoice == "commit-to-new-branch"
}

type UploadRepoFile struct {
	TreePath      string `binding:"MaxSize(500)"`
	CommitSummary string `binding:"MaxSize(100)"`
	CommitMessage string
	CommitChoice  string `binding:"Required;MaxSize(50)"`
	NewBranchName string `binding:"AlphaDashDot;MaxSize(100)"`
	Files         []Upload
}

func (f *UploadRepoFile) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

func (f *UploadRepoFile) IsNewBrnach() bool {
	return f.CommitChoice == "commit-to-new-branch"
}

type RemoveUploadFile struct {
	UUID string
	Name string
}

func (f *RemoveUploadFile) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

type DeleteRepoFile struct {
	CommitSummary string `binding:"MaxSize(100)"`
	CommitMessage string
	CommitChoice  string `binding:"Required;MaxSize(50)"`
	NewBranchName string `binding:"AlphaDashDot;MaxSize(100)"`
}

func (f *DeleteRepoFile) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}

func (f *DeleteRepoFile) IsNewBrnach() bool {
	return f.CommitChoice == "commit-to-new-branch"
}
