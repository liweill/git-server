package repo

import (
	"fmt"
	"git-server/internal/conf"
	"git-server/internal/osutil"
	"github.com/gogs/git-module"
	"github.com/pkg/errors"
	"github.com/unknwon/com"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type UpdateRepoFileOptions struct {
	OldBranch   string
	NewBranch   string
	OldTreeName string
	NewTreeName string
	Message     string
	Content     string
	RepoLink    string
	IsNewFile   bool
}

// isRepositoryGitPath returns true if given path is or resides inside ".git"
// path of the repository.
//
// TODO(unknwon): Move to repoutil during refactoring for this file.
func isRepositoryGitPath(path string) bool {
	path = strings.ToLower(path)
	return strings.HasSuffix(path, ".git") ||
		strings.Contains(path, ".git/") ||
		strings.Contains(path, `.git\`) ||
		// Windows treats ".git." the same as ".git"
		strings.HasSuffix(path, ".git.") ||
		strings.Contains(path, ".git./") ||
		strings.Contains(path, `.git.\`)
}
func LocalCopyPath(repoLink string) string {
	return filepath.Join(conf.Repository.LocalPath, "localRepo", repoLink)
}

func DiscardLocalRepoBranchChanges(repoLink, branch string) error {
	return discardLocalRepoBranchChanges(LocalCopyPath(repoLink), branch)
}

// discardLocalRepoBranchChanges discards local commits/changes of
// given branch to make sure it is even to remote branch.
func discardLocalRepoBranchChanges(localPath, branch string) error {
	if !com.IsExist(localPath) {
		return nil
	}

	// No need to check if nothing in the repository.
	if !git.RepoHasBranch(localPath, branch) {
		return nil
	}

	rev := "origin/" + branch
	if err := git.Reset(localPath, rev, git.ResetOptions{Hard: true}); err != nil {
		return fmt.Errorf("reset [revision: %s]: %v", rev, err)
	}
	return nil
}
func repoPath(repoLink string) string {
	return filepath.Join(conf.Repository.Root, repoLink) + ".git"
}
func UpdateLocalCopyBranch(repoLink, branch string) error {
	return updateLocalCopyBranch(repoPath(repoLink), LocalCopyPath(repoLink), branch, false)
}

// UpdateLocalCopy fetches latest changes of given branch from repoPath to localPath.
// It creates a new clone if local copy does not exist, but does not checks out to a
// specific branch if the local copy belongs to a wiki.
// For existing local copy, it checks out to target branch by default, and safe to
// assume subsequent operations are against target branch when caller has confidence
// about no race condition.
func updateLocalCopyBranch(repoPath, localPath, branch string, isWiki bool) (err error) {
	if !osutil.IsExist(localPath) {
		// Checkout to a specific branch fails when wiki is an empty repository.
		if isWiki {
			branch = ""
		}
		if err = git.Clone(repoPath, localPath, git.CloneOptions{
			Branch:  branch,
			Timeout: time.Duration(300) * time.Second,
		}); err != nil {
			return fmt.Errorf("git clone [branch: %s]: %v", branch, err)
		}
		return nil
	}

	gitRepo, err := git.Open(localPath)
	if err != nil {
		return fmt.Errorf("open repository: %v", err)
	}

	if err = gitRepo.Fetch(git.FetchOptions{
		Prune: true,
	}); err != nil {
		return fmt.Errorf("fetch: %v", err)
	}

	if err = gitRepo.Checkout(branch); err != nil {
		return fmt.Errorf("checkout [branch: %s]: %v", branch, err)
	}

	// Reset to align with remote in case of force push.
	rev := "origin/" + branch
	if err = gitRepo.Reset(rev, git.ResetOptions{
		Hard: true,
	}); err != nil {
		return fmt.Errorf("reset [revision: %s]: %v", rev, err)
	}
	return nil
}

// UpdateRepoFile adds or updates a file in repository.
func UpdateRepoFile(opts UpdateRepoFileOptions) (err error) {
	// 🚨 SECURITY: Prevent uploading files into the ".git" directory
	if isRepositoryGitPath(opts.NewTreeName) {
		return errors.Errorf("bad tree path %q", opts.NewTreeName)
	}

	repoWorkingPool.CheckIn(com.ToStr(opts.RepoLink))
	defer repoWorkingPool.CheckOut(com.ToStr(opts.RepoLink))

	if err = DiscardLocalRepoBranchChanges(opts.RepoLink, opts.OldBranch); err != nil {
		return fmt.Errorf("discard local repo branch[%s] changes: %v", opts.OldBranch, err)
	} else if err = UpdateLocalCopyBranch(opts.RepoLink, opts.OldBranch); err != nil {
		return fmt.Errorf("update local copy branch[%s]: %v", opts.OldBranch, err)
	}

	//repoPath := repo.RepoPath()
	localPath := LocalCopyPath(opts.RepoLink)

	//if opts.OldBranch != opts.NewBranch {
	//	// Directly return error if new branch already exists in the server
	//	if git.RepoHasBranch(repoPath, opts.NewBranch) {
	//		return dberrors.BranchAlreadyExists{Name: opts.NewBranch}
	//	}
	//
	//	// Otherwise, delete branch from local copy in case out of sync
	//	if git.RepoHasBranch(localPath, opts.NewBranch) {
	//		if err = git.DeleteBranch(localPath, opts.NewBranch, git.DeleteBranchOptions{
	//			Force: true,
	//		}); err != nil {
	//			return fmt.Errorf("delete branch %q: %v", opts.NewBranch, err)
	//		}
	//	}
	//
	//	if err := repo.CheckoutNewBranch(opts.OldBranch, opts.NewBranch); err != nil {
	//		return fmt.Errorf("checkout new branch[%s] from old branch[%s]: %v", opts.NewBranch, opts.OldBranch, err)
	//	}
	//}

	oldFilePath := path.Join(localPath, opts.OldTreeName)
	filePath := path.Join(localPath, opts.NewTreeName)
	if err = os.MkdirAll(path.Dir(filePath), os.ModePerm); err != nil {
		return err
	}

	// If it's meant to be a new file, make sure it doesn't exist.
	if opts.IsNewFile {
		if com.IsExist(filePath) {
			return errors.Errorf("repository file already exists [file_name: %s]", filePath)
		}
	}

	// Ignore move step if it's a new file under a directory.
	// Otherwise, move the file when name changed.
	if osutil.IsFile(oldFilePath) && opts.OldTreeName != opts.NewTreeName {
		if err = git.Move(localPath, opts.OldTreeName, opts.NewTreeName); err != nil {
			return fmt.Errorf("git mv %q %q: %v", opts.OldTreeName, opts.NewTreeName, err)
		}
	}

	if err = os.WriteFile(filePath, []byte(opts.Content), 0600); err != nil {
		return fmt.Errorf("write file: %v", err)
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
		return fmt.Errorf("commit changes on %q: %v", localPath, err)
	}

	err = git.Push(localPath, "origin", opts.NewBranch)
	//git.PushOptions{
	//	CommandOptions: git.CommandOptions{
	//		Envs: ComposeHookEnvs(ComposeHookEnvsOptions{
	//			AuthUser:  doer,
	//			OwnerName: repo.MustOwner().Name,
	//			OwnerSalt: repo.MustOwner().Salt,
	//			RepoID:    repo.ID,
	//			RepoName:  repo.Name,
	//			RepoPath:  repo.RepoPath(),
	//		}),
	//	},
	//},
	//)
	if err != nil {
		return fmt.Errorf("git push origin %s: %v", opts.NewBranch, err)
	}
	return nil
}
