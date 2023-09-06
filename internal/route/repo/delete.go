package repo

import (
	"fmt"
	"git-server/internal/context"
	"git-server/internal/form"
	"git-server/internal/pathutil"
	"git-server/internal/type"
	"github.com/gogs/git-module"
	"github.com/pkg/errors"
	"github.com/unknwon/com"
	"os"
	"path"
	"strings"
	"time"
	log "unknwon.dev/clog/v2"
)

func DeleteFilePost(c *context.Context, f form.DeleteRepoFile) {
	c.Data["BranchLink"] = c.Repo.RepoLink + "/src/" + c.Repo.BranchName

	c.Repo.TreePath = pathutil.Clean(c.Repo.TreePath)
	c.Data["TreePath"] = c.Repo.TreePath

	oldBranchName := c.Repo.BranchName
	branchName := oldBranchName

	if f.IsNewBrnach() {
		branchName = f.NewBranchName
	}
	c.Data["commit_summary"] = f.CommitSummary
	c.Data["commit_message"] = f.CommitMessage
	c.Data["commit_choice"] = f.CommitChoice
	c.Data["new_branch_name"] = branchName

	//if oldBranchName != branchName {
	//	if _, err := c.Repo.Repository.GetBranch(branchName); err == nil {
	//		c.FormErr("NewBranchName")
	//		c.RenderWithErr(c.Tr("repo.editor.branch_already_exists", branchName), tmplEditorDelete, &f)
	//		return
	//	}
	//}

	message := strings.TrimSpace(f.CommitSummary)
	if message == "" {
		message = c.Tr("repo.editor.delete", c.Repo.TreePath)
	}

	f.CommitMessage = strings.TrimSpace(f.CommitMessage)
	if len(f.CommitMessage) > 0 {
		message += "\n\n" + f.CommitMessage
	}

	if err := DeleteRepoFile(DeleteRepoFileOptions{
		LastCommitID: c.Repo.CommitID,
		OldBranch:    oldBranchName,
		NewBranch:    branchName,
		TreePath:     c.Repo.TreePath,
		Message:      message,
		RepoLink:     c.Repo.RepoLink,
	}); err != nil {
		log.Error("Failed to delete repo file: %v", err)
		c.JSON(500, _type.FaildResult(errors.Errorf("%s %v", "repo.editor.fail_to_delete_file")))
		return
	}
	c.JSON(200, _type.SuccessResult("成功删除文件"))

}

type DeleteRepoFileOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	TreePath     string
	Message      string
	RepoLink     string
}

func DeleteRepoFile(opts DeleteRepoFileOptions) (err error) {
	repoWorkingPool.CheckIn(com.ToStr(opts.RepoLink))
	defer repoWorkingPool.CheckOut(com.ToStr(opts.RepoLink))

	if err = DiscardLocalRepoBranchChanges(opts.RepoLink, opts.OldBranch); err != nil {
		return fmt.Errorf("discard local repo branch[%s] changes: %v", opts.OldBranch, err)
	} else if err = UpdateLocalCopyBranch(opts.RepoLink, opts.OldBranch); err != nil {
		return fmt.Errorf("update local copy branch[%s]: %v", opts.OldBranch, err)
	}

	//if opts.OldBranch != opts.NewBranch {
	//	if err := repo.CheckoutNewBranch(opts.OldBranch, opts.NewBranch); err != nil {
	//		return fmt.Errorf("checkout new branch[%s] from old branch[%s]: %v", opts.NewBranch, opts.OldBranch, err)
	//	}
	//}

	localPath := LocalCopyPath(opts.RepoLink)
	if err = os.Remove(path.Join(localPath, opts.TreePath)); err != nil {
		return fmt.Errorf("remove file %q: %v", opts.TreePath, err)
	}

	if err = git.Add(localPath, git.AddOptions{All: true}); err != nil {
		return fmt.Errorf("git add --all: %v", err)
	}

	err = git.CreateCommit(
		localPath,
		&git.Signature{
			Name:  "zhang",
			Email: "1571334850@qq.com",
			When:  time.Now(),
		},
		opts.Message,
	)
	if err != nil {
		return fmt.Errorf("commit changes to %q: %v", localPath, err)
	}
	err = git.Push(localPath, "origin", opts.NewBranch)
	//	git.PushOptions{
	//		CommandOptions: git.CommandOptions{
	//			Envs: ComposeHookEnvs(ComposeHookEnvsOptions{
	//				AuthUser:  doer,
	//				OwnerName: repo.MustOwner().Name,
	//				OwnerSalt: repo.MustOwner().Salt,
	//				RepoID:    repo.ID,
	//				RepoName:  repo.Name,
	//				RepoPath:  repo.RepoPath(),
	//			}),
	//		},
	//	},
	//)
	if err != nil {
		return fmt.Errorf("git push origin %s: %v", opts.NewBranch, err)
	}
	return nil
}
