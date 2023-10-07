package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"git-server/internal/conf"
)

type Config struct {
	APIEndpoint string
	SkipVerify  bool
}

type OrginoneUserResp struct {
	Code    int                  `json:"code"`
	Data    OrginoneUserRespData `json:"data"`
	Msg     string               `json:"msg"`
	Success bool                 `json:"success"`
}

type OrginoneUserRespData struct {
	Author      string                     `json:"author"`
	License     string                     `json:"license"`
	TokenType   string                     `json:"tokenType"`
	Target      OrginoneUserRespDataTarget `json:"target"`
	AccessToken string                     `json:"accessToken"`
	ExpiresIn   int                        `json:"expiresIn"`
}

type OrginoneUserRespDataTarget struct {
	Id         string                           `json:"id"`
	Name       string                           `json:"name"`
	Code       string                           `json:"code"`
	Remark     string                           `json:"remark"`
	TypeName   string                           `json:"typeName"`
	BelongId   string                           `json:"belongId"`
	ThingId    string                           `json:"thingId"`
	Status     int                              `json:"status"`
	CreateUser string                           `json:"createUser"`
	UpdateUser string                           `json:"updateUser"`
	Version    string                           `json:"version"`
	CreateTime string                           `json:"createTime"`
	UpdateTime string                           `json:"updateTime"`
	Team       OrginoneUserRespDataTargetTeam   `json:"team"`
	Belong     OrginoneUserRespDataTargetBelong `json:"belong"`
}

type OrginoneUserRespDataTargetTeam struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	TargetId   string `json:"targetId"`
	Status     int    `json:"status"`
	CreateUser string `json:"createUser"`
	UpdateUser string `json:"updateUser"`
	Version    string `json:"version"`
	CreateTime string `json:"createTime"`
	UpdateTime string `json:"updateTime"`
}

type OrginoneUserRespDataTargetBelong struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	Code       string `json:"code"`
	Remark     string `json:"remark"`
	TypeName   string `json:"typeName"`
	BelongId   string `json:"belongId"`
	ThingId    string `json:"thingId"`
	Status     int    `json:"status"`
	CreateUser string `json:"createUser"`
	UpdateUser string `json:"updateUser"`
	Version    string `json:"version"`
	CreateTime string `json:"createTime"`
	UpdateTime string `json:"updateTime"`
}

type OrginoneUserJoinTargetResp struct {
	Code int `json:"code"`
	Data struct {
		Limit  int `json:"limit"`
		Total  int `json:"total"`
		Result []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Code       string `json:"code"`
			Icon       string `json:"icon,omitempty"`
			Remark     string `json:"remark"`
			TypeName   string `json:"typeName"`
			BelongID   string `json:"belongId"`
			ThingID    string `json:"thingId"`
			Status     int    `json:"status"`
			CreateUser string `json:"createUser"`
			UpdateUser string `json:"updateUser"`
			Version    string `json:"version"`
			CreateTime string `json:"createTime"`
			UpdateTime string `json:"updateTime"`
			Team       struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				Code       string `json:"code"`
				TargetID   string `json:"targetId"`
				Status     int    `json:"status"`
				CreateUser string `json:"createUser"`
				UpdateUser string `json:"updateUser"`
				Version    string `json:"version"`
				CreateTime string `json:"createTime"`
				UpdateTime string `json:"updateTime"`
			} `json:"team"`
			Belong struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				Code       string `json:"code"`
				Icon       string `json:"icon"`
				Remark     string `json:"remark"`
				TypeName   string `json:"typeName"`
				BelongID   string `json:"belongId"`
				ThingID    string `json:"thingId"`
				Status     int    `json:"status"`
				CreateUser string `json:"createUser"`
				UpdateUser string `json:"updateUser"`
				Version    string `json:"version"`
				CreateTime string `json:"createTime"`
				UpdateTime string `json:"updateTime"`
			} `json:"belong,omitempty"`
		} `json:"result"`
	} `json:"data"`
	Msg     string `json:"msg"`
	Success bool   `json:"success"`
}

var (
	Authenticator *authenticator
	Authorizer    *authorizer
)

type authorizer struct {
	APIEndpoint string
}

type authenticator struct {
	APIEndpoint string
}

func Init() {
	Authenticator = &authenticator{
		APIEndpoint: conf.Auth.APIEndpoint,
	}
	Authorizer = &authorizer{
		APIEndpoint: conf.Auth.APIEndpoint,
	}
}

type UserDetail struct {
	UID         string
	Name        string
	FullName    string
	Email       string
	Location    string
	WebSite     string
	AccessToken string
	Groups      []string
}

type OrginoneInnerRequstBody struct {
	User   string   `json:"user"`
	Groups []string `json:"groups"`
}

func (c *authenticator) Authenticate(Username, Password string) (user *UserDetail, err error) {

	result, err := c.login2OriginOne(Username, Password)
	if err != nil {
		return nil, err
	}
	return &UserDetail{
		UID:         result.Data.Target.Id,
		Name:        result.Data.Target.Code,
		FullName:    result.Data.Target.Name,
		AccessToken: result.Data.AccessToken,
	}, nil
}

func (c *authenticator) login2OriginOne(login, password string) (*OrginoneUserResp, error) {

	url := c.APIEndpoint + "/orginone/kernel/rest/login"
	requestBody := map[string]string{
		"account": login,
		"pwd":     password,
	}
	data, _ := json.Marshal(requestBody)
	client := &http.Client{
		Timeout: time.Second * 10, // 设置超时时间为10秒
	}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	//if err != nil {
	//	return nil, err
	//}

	if resp.StatusCode/100 > 2 {
		return nil, fmt.Errorf("try fetch user error: %d", resp.StatusCode)
	}

	responseContent := &OrginoneUserResp{}
	err = json.Unmarshal(body, responseContent)
	return responseContent, err
}

func (authz *authorizer) Authorize(user *UserDetail, repoPath string) (bool, error) {
	// 0. 获取组织/用户信息
	isAuthorized := false
	groups := make([]string, 0)
	// -1. 如果认证阶段已经获取到了group信息直接跳过再请求过程
	if user.Groups != nil {
		groups = user.Groups
		for _, grp := range groups {
			if repoPath == grp {
				isAuthorized = true
				break
			}
		}
	} else {
		// 1. 获取用户所属机构、所属群组
		if resp, err := authz.fetchUserOrgAndGroups(user); err != nil {
			return false, fmt.Errorf("could not decide if user has permission: %s", err.Error())
		} else {
			if resp.Code/100 > 2 {
				return false, fmt.Errorf("could not decide if user has permission: %s", resp.Msg)
			}
			groups = append(groups, user.Name)
			if repoPath == user.Name {
				isAuthorized = true
			}
			// 2. 判断用户是否有仓库的相关权限
			for _, grp := range resp.Data.Result {
				groups = append(groups, grp.Code)
				if repoPath == grp.Code {
					isAuthorized = true
					break
				}
			}
		}
	}

	return isAuthorized, nil
}

func GetUserGroups(userId, token string) ([]string, error) {
	url := conf.Auth.APIEndpoint + "/orginone/kernel/rest/request"
	requestBody := map[string]interface{}{
		"module": "target",
		"action": "QueryJoinedTargetById",
		"params": map[string]interface{}{
			"id":        userId,
			"typeNames": []string{"群组", "单位", "医院", "大学"},
		},
	}
	data, _ := json.Marshal(requestBody)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)
	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode/100 > 2 {
		return nil, fmt.Errorf("fetch user joined groups error: %d", resp.StatusCode)
	}

	responseContent := &OrginoneUserJoinTargetResp{}
	err = json.Unmarshal(body, responseContent)
	groups := make([]string, 0)
	for _, v := range responseContent.Data.Result {
		groups = append(groups, v.Code)
	}
	return groups, err
}

// 查询用户所属的组织和群组信息
func (authz *authorizer) fetchUserOrgAndGroups(user *UserDetail) (*OrginoneUserJoinTargetResp, error) {
	url := authz.APIEndpoint + "/orginone/kernel/rest/request"
	requestBody := map[string]interface{}{
		"module": "target",
		"action": "QueryJoinedTargetById",
		"params": map[string]interface{}{
			"id":        user.UID,
			"typeNames": []string{"群组", "单位", "医院", "大学"},
		},
	}
	data, _ := json.Marshal(requestBody)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", user.AccessToken)
	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	//if err != nil {
	//	return nil, err
	//}

	if resp.StatusCode/100 > 2 {
		return nil, fmt.Errorf("fetch user joined groups error: %d", resp.StatusCode)
	}

	responseContent := &OrginoneUserJoinTargetResp{}
	err = json.Unmarshal(body, responseContent)
	return responseContent, err
}

// 从repoName中获取repoPath,即仓库名之外的部分
func repoPath(repoName string) string {
	repoPath := strings.Split(repoName, "/")
	num := len(repoPath)
	return strings.Join(repoPath[0:num-1], "/")
}
