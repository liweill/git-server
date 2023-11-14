package doc

import (
	"fmt"
	"git-server/internal/auth"
	"git-server/internal/conf"
	"git-server/internal/context"
	"git-server/internal/form"
	"git-server/internal/route/repo"
	"github.com/go-macaron/binding"
	"github.com/urfave/cli"
	"gopkg.in/macaron.v1"
	"log"
	"os"
)

var Web = cli.Command{
	Name:  "web",
	Usage: "Start web server",
	Description: `Gogs web server is the only thing you need to run,
and it takes care of all the other things for you`,
	Action: runWeb,
}

func runWeb(c *cli.Context) error {
	err := conf.Init()
	if err != nil {
		log.Fatalf("init config error: %v", err)
		os.Exit(-1)
	}
	auth.Init()
	fmt.Println(conf.AppPath())
	m := macaron.Classic()
	bindIgnErr := binding.BindIgnErr
	m.Use(macaron.Renderer())
	m.Group("", func() {
		m.Group("/:username/:reponame", func() {
			m.Get("", repo.Home)
			m.Get("/src/*", repo.Home)
			m.Get("/raw/*", repo.SingleDownload)
			m.Get("/commits/*", repo.RefCommits)
			m.Post("/createBranch", bindIgnErr(form.CreateBranch{}), repo.CreateBranch)
			m.Group("", func() {
				m.Post("/_edit/*", bindIgnErr(form.EditRepoFile{}), repo.EditFilePost)
				m.Post("/_upload/*", bindIgnErr(form.UploadRepoFile{}), repo.UploadFilePost)
				m.Post("/upload-file", repo.UploadFileToServer)
				m.Post("/upload-remove", bindIgnErr(form.RemoveUploadFile{}), repo.RemoveUploadFileFromServer)
				m.Post("/_delete/*", bindIgnErr(form.DeleteRepoFile{}), repo.DeleteFilePost)
			})
			m.Group("", func() {
				m.Get("/commit/:sha([a-f0-9]{7,40})$", repo.Diff)
				m.Get("/compare/:before\\.\\.\\.:after", repo.CompareAndPullRequest)
				m.Post("/compare/:before\\.\\.\\.:after", repo.CompareAndPullRequestPost)
			})
			m.Group("/branches", func() {
				m.Get("", repo.Branches)
				m.Get("/all", repo.AllBranches)
			})
			m.Group("/pulls", func() {
				m.Post("/commits", bindIgnErr(form.PullRequest{}), repo.ViewPullCommits)
				m.Post("/merge", bindIgnErr(form.PullRequest{}), repo.MergePullRequest)
				m.Post("/files", bindIgnErr(form.PullRequest{}), repo.ViewPullFiles)
				m.Post("", bindIgnErr(form.PullRequest{}), repo.PrepareViewPullInfo)
			})
			m.Group("/settings", func() {
				m.Group("/branches", func() {
					m.Get("", repo.SettingsBranches)
					m.Get("/default_branch", repo.UpdateDefaultBranch)
					m.Combo("/*").Get(repo.SettingsProtectedBranch).
						Post(bindIgnErr(form.ProtectedBranch{}), repo.SettingsProtectedBranchPost)
				})
			})
		}, context.RepoAssignment(), context.RepoRef())
		m.Group("/repo", func() {
			m.Post("/create", bindIgnErr(form.Repo{}), repo.CreatePost)
			m.Post("/delete", bindIgnErr(form.Repo{}), repo.DeletePost)
		})
		m.Group("/repos", func() {
			m.Post("", bindIgnErr(form.ListRepo{}), repo.ListRepo)
			m.Delete("", bindIgnErr(form.Repo{}), repo.DeleteRepo)
		})
		// ***************************
		// ----- HTTP Git routes -----
		// ***************************
		m.Group("/:username/:reponame", func() {
			m.Route("/*", "GET,POST,OPTIONS", repo.HTTPContexter(), repo.HTTP)
		})
	},
		context.Contexter(),
	)
	m.Run()
	return nil
}
