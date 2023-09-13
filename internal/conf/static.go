// Copyright 2020 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package conf

import (
	"github.com/pkg/errors"
	"gopkg.in/ini.v1"
)

func Init() error {
	inidata, err := ini.Load("app.ini")
	if err != nil {
		return errors.Wrap(err, "Fail to read app.ini")
	}

	if err = inidata.Section("repository").MapTo(&Repository); err != nil {
		return errors.Wrap(err, "mapping git section")
	}

	if err = inidata.Section("server").MapTo(&Server); err != nil {
		return errors.Wrap(err, "mapping server section")
	}

	if err = inidata.Section("auth").MapTo(&Auth); err != nil {
		return errors.Wrap(err, "mapping auth section")
	}

	if err = inidata.Section("git").MapTo(&Git); err != nil {
		return errors.Wrap(err, "mapping auth section")
	}
	return nil
}

var (
	Auth       AuthOpts
	Server     ServerOpts
	Repository RepositoryOpts
	Git        GitOpts
)

type AuthOpts struct {
	APIEndpoint string `ini:"endpoint"`
}
type ServerOpts struct {
	ExternalURL string `ini:"EXTERNAL_URL"`
	Domain      string `ini:"DOMAIN"`
	HTTPPort    string `ini:"HTTP_PORT"`
	AppDataPath string `ini:"APP_DATA_PATH"`
}

type RepositoryOpts struct {
	Root          string `ini:"ROOT"`
	LocalPath     string `ini:"LOCAL_PATH"`
	DefaultBranch string `ini:"DEFAULT_BRANCH"`
	ANSICharset   string `ini:"ANSI_CHARSET"`
}

type GitOpts struct {
	DisableDiffHighlight bool
	MaxDiffFiles         int `ini:"MAX_GIT_DIFF_FILES"`
	MaxDiffLines         int `ini:"MAX_GIT_DIFF_LINES"`
	MaxDiffLineChars     int `ini:"MAX_GIT_DIFF_LINE_CHARACTERS"`
	Timeout              struct {
		Migrate int `ini:"MIGRATE"`
		Mirror  int `ini:"MIRROR"`
		Clone   int `ini:"CLONE"`
		Pull    int `ini:"PULL"`
		Diff    int `ini:"DIFF"`
		GC      int `ini:"GC"`
	} `ini:"git.timeout"`
}
