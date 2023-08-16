// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"errors"
	"git-server/internal/conf"
	"git-server/internal/repoutil"
	_type "git-server/internal/type"
	"github.com/gogs/git-module"
	"net/url"
	"strings"

	"gopkg.in/macaron.v1"
)

type PullRequest struct {
	Allowed  bool
	SameRepo bool
	HeadInfo string // [<user>:]<branch>
}

type Repository struct {
	IsWatching   bool
	IsViewBranch bool
	IsViewTag    bool
	IsViewCommit bool
	Commit       *git.Commit
	Tag          *git.Tag
	GitRepo      *git.Repository
	BranchName   string
	TagName      string
	TreePath     string
	CommitID     string
	RepoLink     string
	CloneLink    repoutil.CloneLink
	CommitsCount int64

	PullRequest *PullRequest
}

// MakeURL accepts a string or url.URL as argument and returns escaped URL prepended with repository URL.
func (r *Repository) MakeURL(location any) string {
	switch location := location.(type) {
	case string:
		tempURL := url.URL{
			Path: r.RepoLink + "/" + location,
		}
		return tempURL.String()
	case url.URL:
		location.Path = r.RepoLink + "/" + location.Path
		return location.String()
	default:
		panic("location type must be either string or url.URL")
	}
}

// [0]: issues, [1]: wiki
func RepoAssignment(pages ...bool) macaron.Handler {
	return func(c *Context) {
		var (
			err error
		)
		ownerName := c.Params(":username")
		repoName := strings.TrimSuffix(c.Params(":reponame"), ".git")
		c.Repo.RepoLink = ownerName + "/" + repoName

		gitRepo, err := git.Open(repoutil.RepoPath(ownerName, repoName))
		if err != nil {
			//open repository
			c.JSON(500, _type.FaildResult(err))
			return
		}
		c.Repo.GitRepo = gitRepo
		//tags, err := c.Repo.GitRepo.Tags()
		//if err != nil {
		//	c.Error(500, "get tags")
		//	return
		//}
		//c.Data["Tags"] = tags

		//c.Data["TagName"] = c.Repo.TagName
		branches, err := c.Repo.GitRepo.Branches()
		if err != nil {
			//c.Error(500, "get branches")
			result := _type.FaildResult(errors.New("this repo is bare"))
			c.JSON(500, result)
			return
		}
		c.Data["Branches"] = branches
	}
}

// RepoRef handles repository reference name including those contain `/`.
func RepoRef() macaron.Handler {
	return func(c *Context) {
		var (
			refName string
			err     error
		)

		// Get default branch.
		if c.Params("*") == "" {
			refName = conf.Repository.DefaultBranch
			if !c.Repo.GitRepo.HasBranch(refName) {
				branches, err := c.Repo.GitRepo.Branches()
				if err != nil {
					c.JSON(500, _type.FaildResult(err))
					return
				}
				refName = branches[0]
			}
			c.Repo.Commit, err = c.Repo.GitRepo.BranchCommit(refName)
			if err != nil {
				//get branch commit
				c.JSON(500, _type.FaildResult(err))
				return
			}
			c.Repo.CommitID = c.Repo.Commit.ID.String()
			c.Repo.IsViewBranch = true

		} else {
			hasMatched := false
			parts := strings.Split(c.Params("*"), "/")
			for i, part := range parts {
				refName = strings.TrimPrefix(refName+"/"+part, "/")
				if c.Repo.GitRepo.HasBranch(refName) ||
					c.Repo.GitRepo.HasTag(refName) {
					if i < len(parts)-1 {
						c.Repo.TreePath = strings.Join(parts[i+1:], "/")
					}
					hasMatched = true
					break
				}
			}
			if !hasMatched && len(parts[0]) == 40 {
				refName = parts[0]
				c.Repo.TreePath = strings.Join(parts[1:], "/")
			}

			if c.Repo.GitRepo.HasBranch(refName) {
				c.Repo.IsViewBranch = true

				c.Repo.Commit, err = c.Repo.GitRepo.BranchCommit(refName)
				if err != nil {
					c.Error(500, "get branch commit")
					return
				}
				c.Repo.CommitID = c.Repo.Commit.ID.String()

			} else if c.Repo.GitRepo.HasTag(refName) {
				c.Repo.IsViewTag = true
				c.Repo.Commit, err = c.Repo.GitRepo.TagCommit(refName)
				if err != nil {
					//get tag commit
					c.JSON(500, _type.FaildResult(err))
					return
				}
				c.Repo.CommitID = c.Repo.Commit.ID.String()
			} else if len(refName) == 40 {
				c.Repo.IsViewCommit = true
				c.Repo.CommitID = refName

				c.Repo.Commit, err = c.Repo.GitRepo.CatFileCommit(refName)
				if err != nil {
					c.JSON(500, _type.FaildResult(err))
					return
				}
			} else {
				c.JSON(500, _type.FaildResult(errors.New("not find")))
				return
			}
		}
		c.Repo.BranchName = refName
	}
}
