package repo

import (
	"fmt"
	"git-server/internal/conf"
	"git-server/internal/context"
	"git-server/internal/form"
	"git-server/internal/gitutil"
	"git-server/internal/osutil"
	"git-server/internal/pathutil"
	"git-server/internal/tool"
	"git-server/internal/type"
	"github.com/gogs/git-module"
	"github.com/pkg/errors"
	gouuid "github.com/satori/go.uuid"
	"github.com/unknwon/com"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
	log "unknwon.dev/clog/v2"
)

type UploadRepoFileOptions struct {
	LastCommitID string
	OldBranch    string
	NewBranch    string
	TreePath     string
	Message      string
	RepoLink     string
	Files        []*Upload // In UUID format
}

func UploadFileToServer(c *context.Context) {
	file, header, err := c.Req.FormFile("file")
	if err != nil {
		c.Error(500, "get file")
		return
	}
	defer file.Close()

	buf := make([]byte, 1024)
	n, _ := file.Read(buf)
	if n > 0 {
		buf = buf[:n]
	}
	//fileType := http.DetectContentType(buf)

	//if len(conf.Repository.Upload.AllowedTypes) > 0 {
	//	allowed := false
	//	for _, t := range conf.Repository.Upload.AllowedTypes {
	//		t := strings.Trim(t, " ")
	//		if t == "*/*" || t == fileType {
	//			allowed = true
	//			break
	//		}
	//	}
	//
	//	if !allowed {
	//		c.PlainText(http.StatusBadRequest, ErrFileTypeForbidden.Error())
	//		return
	//	}
	//}

	upload, err := NewUpload(header.Filename, buf, file)
	if err != nil {
		c.Error(500, "new upload")
		return
	}
	//log.Trace("New file uploaded by user[%d]: %s", c.UserID(), upload.UUID)
	c.JSONSuccess(map[string]string{
		"uuid":     upload.UUID,
		"fileName": header.Filename,
	})
}

// Upload represent a uploaded file to a repo to be deleted when moved
type Upload struct {
	UUID string
	Name string
}

// UploadLocalPath returns where uploads is stored in local file system based on given UUID.
func UploadLocalPath(uuid string) string {
	return path.Join(conf.Repository.LocalPath, "uploads", uuid[0:1], uuid[1:2], uuid)
}

// LocalPath returns where uploads are temporarily stored in local file system.
func (upload *Upload) LocalPath() string {
	return UploadLocalPath(upload.UUID)
}
func NewUpload(name string, buf []byte, file multipart.File) (_ *Upload, err error) {
	if tool.IsMaliciousPath(name) {
		return nil, fmt.Errorf("malicious path detected: %s", name)
	}

	upload := &Upload{
		UUID: gouuid.NewV4().String(),
		Name: name,
	}

	localPath := upload.LocalPath()
	if err = os.MkdirAll(path.Dir(localPath), os.ModePerm); err != nil {
		return nil, fmt.Errorf("mkdir all: %v", err)
	}

	fw, err := os.Create(localPath)
	if err != nil {
		return nil, fmt.Errorf("create: %v", err)
	}
	defer func() { _ = fw.Close() }()

	if _, err = fw.Write(buf); err != nil {
		return nil, fmt.Errorf("write: %v", err)
	} else if _, err = io.Copy(fw, file); err != nil {
		return nil, fmt.Errorf("copy: %v", err)
	}

	return upload, nil
}

func UploadFilePost(c *context.Context, f form.UploadRepoFile) {
	//c.PageIs("Upload")
	//renderUploadSettings(c)

	oldBranchName := c.Repo.BranchName
	branchName := oldBranchName
	if f.IsNewBrnach() {
		branchName = f.NewBranchName
	}

	f.TreePath = pathutil.Clean(f.TreePath)
	treeNames, treePaths := getParentTreeFields(f.TreePath)
	if len(treeNames) == 0 {
		// We must at least have one element for user to input.
		treeNames = []string{""}
	}

	c.Data["TreePath"] = f.TreePath
	c.Data["TreeNames"] = treeNames
	c.Data["TreePaths"] = treePaths
	c.Data["BranchLink"] = c.Repo.RepoLink + "/src/" + branchName
	c.Data["commit_summary"] = f.CommitSummary
	c.Data["commit_message"] = f.CommitMessage
	c.Data["commit_choice"] = f.CommitChoice
	c.Data["new_branch_name"] = branchName

	//if c.HasError() {
	//	c.Success(tmplEditorUpload)
	//	return
	//}

	if oldBranchName != branchName {
		if _, err := GetBranch(branchName, repoPath(c.Repo.RepoLink)); err == nil {
			c.JSON(500, _type.FaildResult(errors.New("repo.editor.branch_already_exists")))
			return
		}
	}

	var newTreePath string
	for _, part := range treeNames {
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

		// User can only upload files to a directory.
		if !entry.IsTree() {
			c.JSON(500, _type.FaildResult(errors.New("repo.editor.directory_is_a_file")))
			return
		}
	}

	message := strings.TrimSpace(f.CommitSummary)
	if message == "" {
		message = c.Tr("repo.editor.upload_files_to_dir", f.TreePath)
	}

	f.CommitMessage = strings.TrimSpace(f.CommitMessage)
	if len(f.CommitMessage) > 0 {
		message += "\n\n" + f.CommitMessage
	}
	uploads := process(f)
	if err := UploadRepoFiles(UploadRepoFileOptions{
		LastCommitID: c.Repo.CommitID,
		OldBranch:    oldBranchName,
		NewBranch:    branchName,
		TreePath:     f.TreePath,
		RepoLink:     c.Repo.RepoLink,
		Message:      message,
		Files:        uploads,
	}); err != nil {
		log.Error("Failed to upload files: %v", err)
		c.JSON(500, _type.FaildResult(errors.New("repo.editor.unable_to_upload_files")))
		return
	}
	c.JSON(200, _type.SuccessResult("成功上传文件"))
	//if f.IsNewBrnach() && c.Repo.PullRequest.Allowed {
	//	c.Redirect(c.Repo.PullRequestURL(oldBranchName, f.NewBranchName))
	//} else {
	//	c.Redirect(c.Repo.RepoLink + "/src/" + branchName + "/" + f.TreePath)
	//}
}
func process(f form.UploadRepoFile) []*Upload {
	uploads := make([]*Upload, 0)
	for i := 0; i < len(f.Files); i++ {
		upload := &Upload{
			UUID: f.Files[i].UUID,
			Name: f.Files[i].Name,
		}
		uploads = append(uploads, upload)
	}
	return uploads
}
func UploadRepoFiles(opts UploadRepoFileOptions) error {
	if len(opts.Files) == 0 {
		return nil
	}

	// 🚨 SECURITY: Prevent uploading files into the ".git" directory
	if isRepositoryGitPath(opts.TreePath) {
		return errors.Errorf("bad tree path %q", opts.TreePath)
	}
	//uploads, err := GetUploadsByUUIDs([]string{"12"})
	//if err != nil {
	//	return fmt.Errorf("get uploads by UUIDs[%v]: %v", opts.Files, err)
	//}
	uploads := opts.Files
	repoWorkingPool.CheckIn(com.ToStr(opts.RepoLink))
	defer repoWorkingPool.CheckOut(com.ToStr(opts.RepoLink))
	var err error
	if err = DiscardLocalRepoBranchChanges(opts.RepoLink, opts.OldBranch); err != nil {
		return fmt.Errorf("discard local repo branch[%s] changes: %v", opts.OldBranch, err)
	} else if err = UpdateLocalCopyBranch(opts.RepoLink, opts.OldBranch); err != nil {
		return fmt.Errorf("update local copy branch[%s]: %v", opts.OldBranch, err)
	}
	localPath := LocalCopyPath(opts.RepoLink)
	if opts.OldBranch != opts.NewBranch {
		if err := CheckoutNewBranch(opts.OldBranch, opts.NewBranch, localPath); err != nil {
			return fmt.Errorf("checkout new branch[%s] from old branch[%s]: %v", opts.NewBranch, opts.OldBranch, err)
		}
	}

	dirPath := path.Join(localPath, opts.TreePath)
	if err = os.MkdirAll(dirPath, os.ModePerm); err != nil {
		return err
	}

	// Copy uploaded files into repository
	for _, upload := range uploads {
		tmpPath := upload.LocalPath()
		if !osutil.IsFile(tmpPath) {
			continue
		}

		upload.Name = pathutil.Clean(upload.Name)

		// 🚨 SECURITY: Prevent uploading files into the ".git" directory
		if isRepositoryGitPath(upload.Name) {
			continue
		}

		targetPath := path.Join(dirPath, upload.Name)
		if err = com.Copy(tmpPath, targetPath); err != nil {
			return fmt.Errorf("copy: %v", err)
		}
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

	if err != nil {
		return fmt.Errorf("git push origin %s: %v", opts.NewBranch, err)
	}
	return DeleteUploads(uploads...)
}

func GetUploadsByUUIDs(uuids []string) ([]*Upload, error) {
	if len(uuids) == 0 {
		return []*Upload{}, nil
	}

	// Silently drop invalid uuids.
	uploads := make([]*Upload, 0, len(uuids))
	return uploads, nil
}

func DeleteUploads(uploads ...*Upload) (err error) {
	if len(uploads) == 0 {
		return nil
	}
	//ids := make([]int64, len(uploads))
	//for i := 0; i < len(uploads); i++ {
	//	ids[i] = uploads[i].ID
	//}

	for _, upload := range uploads {
		localPath := upload.LocalPath()
		if !osutil.IsFile(localPath) {
			continue
		}

		if err := os.Remove(localPath); err != nil {
			return fmt.Errorf("remove upload: %v", err)
		}
	}
	return nil
}

func RemoveUploadFileFromServer(c *context.Context, f form.RemoveUploadFile) {
	if f.UUID == "" {
		c.Status(http.StatusNoContent)
		return
	}
	upload := &Upload{
		UUID: f.UUID,
		Name: f.Name,
	}
	if err := DeleteUploadByUUID(upload); err != nil {
		c.JSON(500, _type.FaildResult(errors.Errorf("%v %s", err, "delete upload by UUID")))
		return
	}
	c.JSON(200, _type.SuccessResult("success"))
}
func DeleteUploadByUUID(upload *Upload) error {

	if err := DeleteUpload(upload); err != nil {
		return fmt.Errorf("delete upload: %v", err)
	}

	return nil
}
func DeleteUpload(u *Upload) error {
	return DeleteUploads(u)
}
