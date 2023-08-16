// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-macaron/cache"
	"github.com/go-macaron/csrf"
	"github.com/go-macaron/session"
	"gopkg.in/macaron.v1"
	log "unknwon.dev/clog/v2"
)

// Context represents context of a request.
type Context struct {
	*macaron.Context
	Cache   cache.Cache
	csrf    csrf.CSRF
	Flash   *session.Flash
	Session session.Store

	Link        string // Current request URL
	IsLogged    bool
	IsBasicAuth bool
	IsTokenAuth bool

	Repo *Repository
}

// RawTitle sets the "Title" field in template data.
func (c *Context) RawTitle(title string) {
	c.Data["Title"] = title
}

// Title localizes the "Title" field in template data.
func (c *Context) Title(locale string) {
	c.RawTitle(c.Tr(locale))
}

// PageIs sets "PageIsxxx" field in template data.
func (c *Context) PageIs(name string) {
	c.Data["PageIs"+name] = true
}

// Require sets "Requirexxx" field in template data.
func (c *Context) Require(name string) {
	c.Data["Require"+name] = true
}

func (c *Context) RequireHighlightJS() {
	c.Require("HighlightJS")
}

func (c *Context) RequireSimpleMDE() {
	c.Require("SimpleMDE")
}

func (c *Context) RequireAutosize() {
	c.Require("Autosize")
}

func (c *Context) RequireDropzone() {
	c.Require("Dropzone")
}

// FormErr sets "Err_xxx" field in template data.
func (c *Context) FormErr(names ...string) {
	for i := range names {
		c.Data["Err_"+names[i]] = true
	}
}

// HasError returns true if error occurs in form validation.
func (c *Context) HasApiError() bool {
	hasErr, ok := c.Data["HasError"]
	if !ok {
		return false
	}
	return hasErr.(bool)
}

func (c *Context) GetErrMsg() string {
	return c.Data["ErrorMsg"].(string)
}

// HasError returns true if error occurs in form validation.
func (c *Context) HasError() bool {
	hasErr, ok := c.Data["HasError"]
	if !ok {
		return false
	}
	c.Flash.ErrorMsg = c.Data["ErrorMsg"].(string)
	c.Data["Flash"] = c.Flash
	return hasErr.(bool)
}

// HasValue returns true if value of given name exists.
func (c *Context) HasValue(name string) bool {
	_, ok := c.Data[name]
	return ok
}

// HTML responses template with given status.
func (c *Context) HTML(status int, name string) {
	log.Trace("Template: %s", name)
	c.Context.HTML(status, name)
}

// Success responses template with status http.StatusOK.
func (c *Context) Success(name string) {
	c.HTML(http.StatusOK, name)
}

// JSONSuccess responses JSON with status http.StatusOK.
func (c *Context) JSONSuccess(data any) {
	c.JSON(http.StatusOK, data)
}

// RawRedirect simply calls underlying Redirect method with no escape.
func (c *Context) RawRedirect(location string, status ...int) {
	c.Context.Redirect(location, status...)
}

// NotFound renders the 404 page.
func (c *Context) NotFound() {
	c.Title("status.page_not_found")
	c.HTML(http.StatusNotFound, fmt.Sprintf("status/%d", http.StatusNotFound))
}

// Errorf renders the 500 response with formatted message.
func (c *Context) Errorf(err int, format string, args ...any) {
	c.Error(err, fmt.Sprintf(format, args...))
}

func (c *Context) PlainText(status int, msg string) {
	c.Render.PlainText(status, []byte(msg))
}

func (c *Context) ServeContent(name string, r io.ReadSeeker, params ...any) {
	modtime := time.Now()
	for _, p := range params {
		switch v := p.(type) {
		case time.Time:
			modtime = v
		}
	}
	c.Resp.Header().Set("Content-Description", "File Transfer")
	c.Resp.Header().Set("Content-Type", "application/octet-stream")
	c.Resp.Header().Set("Content-Disposition", "attachment; filename="+name)
	c.Resp.Header().Set("Content-Transfer-Encoding", "binary")
	c.Resp.Header().Set("Expires", "0")
	c.Resp.Header().Set("Cache-Control", "must-revalidate")
	c.Resp.Header().Set("Pragma", "public")
	http.ServeContent(c.Resp, c.Req.Request, name, modtime, r)
}

// Contexter initializes a classic context for a request.
func Contexter() macaron.Handler {
	return func(ctx *macaron.Context) {
		c := &Context{
			Context: ctx,
			Repo: &Repository{
				PullRequest: &PullRequest{},
			},
		}
		ctx.Map(c)
	}
}
