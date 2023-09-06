package repo

import (
	"git-server/internal/context"
	"git-server/internal/form"
	"git-server/internal/gitutil"
	"git-server/internal/pathutil"
	"git-server/internal/sync"
	"git-server/internal/type"
	"github.com/pkg/errors"
	"path"
	"strings"
)

var repoWorkingPool = sync.NewExclusivePool()

func EditFilePost(c *context.Context, f form.EditRepoFile) {
	editFilePost(c, f, false)
}

func editFilePost(c *context.Context, f form.EditRepoFile, isNewFile bool) {
	c.PageIs("Edit")
	c.RequireHighlightJS()
	c.RequireSimpleMDE()
	c.Data["IsNewFile"] = isNewFile

	oldBranchName := c.Repo.BranchName
	branchName := oldBranchName
	oldTreePath := c.Repo.TreePath
	//lastCommit := f.LastCommit
	f.LastCommit = c.Repo.Commit.ID.String()

	if f.IsNewBrnach() {
		branchName = f.NewBranchName
	}

	f.TreePath = pathutil.Clean(f.TreePath)
	treeNames, treePaths := getParentTreeFields(f.TreePath)

	c.Data["ParentTreePath"] = path.Dir(c.Repo.TreePath)
	c.Data["TreePath"] = f.TreePath
	c.Data["TreeNames"] = treeNames
	c.Data["TreePaths"] = treePaths
	c.Data["BranchLink"] = c.Repo.RepoLink + "/src/" + branchName
	c.Data["FileContent"] = f.Content
	c.Data["commit_summary"] = f.CommitSummary
	c.Data["commit_message"] = f.CommitMessage
	c.Data["commit_choice"] = f.CommitChoice
	c.Data["new_branch_name"] = branchName
	c.Data["last_commit"] = f.LastCommit

	if f.TreePath == "" {
		c.JSON(500, _type.FaildResult(errors.New("repo.editor.filename_cannot_be_empty")))
		return
	}

	//if oldBranchName != branchName {
	//	if _, err := c.Repo.Repository.GetBranch(branchName); err == nil {
	//		c.FormErr("NewBranchName")
	//		c.RenderWithErr(c.Tr("repo.editor.branch_already_exists", branchName), tmplEditorEdit, &f)
	//		return
	//	}
	//}

	var newTreePath string
	for index, part := range treeNames {
		newTreePath = path.Join(newTreePath, part)
		entry, err := c.Repo.Commit.TreeEntry(newTreePath)
		if err != nil {
			if gitutil.IsErrRevisionNotExist(err) {
				// Means there is no item with that name, so we're good
				break
			}

			c.Error(500, "get tree entry")
			return
		}
		if index != len(treeNames)-1 {
			if !entry.IsTree() {
				c.JSON(500, _type.FaildResult(errors.New("TreePath repo.editor.directory_is_a_file")))
				return
			}
		} else {
			if entry.IsSymlink() {
				c.JSON(500, _type.FaildResult(errors.New("TreePath repo.editor.file_is_a_symlink")))
				return
			} else if entry.IsTree() {
				c.FormErr("TreePath")
				c.JSON(500, _type.FaildResult(errors.New("TreePath repo.editor.filename_is_a_directory")))
				return
			}
		}
	}

	if !isNewFile {
		_, err := c.Repo.Commit.TreeEntry(oldTreePath)
		if err != nil {
			if gitutil.IsErrRevisionNotExist(err) {
				c.JSON(500, _type.FaildResult(errors.New("repo.editor.file_editing_no_longer_exists")))
			} else {
				c.Error(500, "get tree entry")
			}
			return
		}
		//if lastCommit != c.Repo.CommitID {
		//	fmt.Println("c.Repo.CommitID", c.Repo.CommitID)
		//	files, err := c.Repo.Commit.FilesChangedAfter(lastCommit)
		//	if err != nil {
		//		c.Error(500, "get changed files")
		//		return
		//	}
		//
		//	for _, file := range files {
		//		if file == f.TreePath {
		//			c.JSON(500, _type.FaildResult(errors.New("repo.editor.file_changed_while_editing")))
		//			return
		//		}
		//	}
		//}
	}

	if oldTreePath != f.TreePath {
		// We have a new filename (rename or completely new file) so we need to make sure it doesn't already exist, can't clobber.
		entry, err := c.Repo.Commit.TreeEntry(f.TreePath)
		if err != nil {
			if !gitutil.IsErrRevisionNotExist(err) {
				c.Error(500, "get tree entry")
				return
			}
		}
		if entry != nil {
			c.JSON(500, _type.FaildResult(errors.New("repo.editor.file_already_exists")))
			return
		}
	}

	message := strings.TrimSpace(f.CommitSummary)
	if message == "" {
		if isNewFile {
			message = c.Tr("repo.editor.add", f.TreePath)
		} else {
			message = c.Tr("repo.editor.update", f.TreePath)
		}
	}

	f.CommitMessage = strings.TrimSpace(f.CommitMessage)
	if len(f.CommitMessage) > 0 {
		message += "\n\n" + f.CommitMessage
	}

	if err := UpdateRepoFile(UpdateRepoFileOptions{
		OldBranch:   oldBranchName,
		NewBranch:   branchName,
		OldTreeName: oldTreePath,
		NewTreeName: f.TreePath,
		Message:     message,
		RepoLink:    c.Repo.RepoLink,
		Content:     strings.ReplaceAll(f.Content, "\r", ""),
		IsNewFile:   isNewFile,
	}); err != nil {
		c.FormErr("TreePath")
		c.JSON(500, _type.FaildResult(errors.Errorf("%s %s", "repo.editor.fail_to_update_file", f.TreePath)))
		return
	}
	c.JSON(200, _type.SuccessResult("成功编辑文件"))

	//if f.IsNewBrnach() && c.Repo.PullRequest.Allowed {
	//	c.Redirect(c.Repo.PullRequestURL(oldBranchName, f.NewBranchName))
	//} else {
	//	c.Redirect(c.Repo.RepoLink + "/src/" + branchName + "/" + f.TreePath)
	//}
}

// getParentTreeFields returns list of parent tree names and corresponding tree paths
// based on given tree path.
func getParentTreeFields(treePath string) (treeNames, treePaths []string) {
	if treePath == "" {
		return treeNames, treePaths
	}

	treeNames = strings.Split(treePath, "/")
	treePaths = make([]string, len(treeNames))
	for i := range treeNames {
		treePaths[i] = strings.Join(treeNames[:i+1], "/")
	}
	return treeNames, treePaths
}
