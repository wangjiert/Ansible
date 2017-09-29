package controllers

import (
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/revel/revel"
)

var split = "*_*"

type WSError struct {
	Status int         `json:"status"`
	Error  interface{} `json:"error"`
}

type WSSuccess struct {
	Status int         `json:"status"`
	Data   interface{} `json:"data"`
}

type ErrorInfo struct {
	Ip     string
	Reason string
}

type StatusInfo struct {
	Ip     string
	Status bool
}

type Ansible struct {
	*revel.Controller
}

var rootPath = "/home/revel/config/"

func (c Ansible) Index() revel.Result {
	return c.Render()
}

func (c Ansible) FileList() revel.Result {
	files, err := ioutil.ReadDir(rootPath)
	if err != nil {
		c.RenderJson(WSError{Status: 500, Error: err.Error()})
	}
	fileList := []string{"hosts"}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".yaml") {
			fileList = append(fileList, file.Name())
		}
	}
	return c.RenderJson(WSSuccess{Status: 200, Data: fileList})
}

func (c Ansible) FileContent() revel.Result {
	filename := c.Params.Query.Get("filename")
	if filename == "hosts" {
		filename = "/etc/ansible/hosts"
	} else if strings.HasSuffix(filename, ".yaml") {
		filename = rootPath + filename
	} else {
		filename = ""
	}
	var fileContent string
	if filename != "" {
		content, err := ioutil.ReadFile(filename)
		if err != nil {
			return c.RenderJson(WSError{Status: 500, Error: err.Error()})
		}
		fileContent = string(content)
	}
	return c.RenderJson(WSSuccess{Status: 200, Data: fileContent})
}

func (c Ansible) Alter() revel.Result {
	var filePath string
	if c.Params.Form.Get("filename") == "hosts" {
		filePath = "/etc/ansible/hosts"
	} else {
		filePath = rootPath + c.Params.Form.Get("filename")
	}
	file, err := os.OpenFile(filePath, os.O_TRUNC|os.O_RDWR, 0777)
	if err != nil {
		return c.RenderJson(WSError{Status: 500, Error: err.Error()})
	}
	_, err = file.WriteString(c.Params.Form.Get("filecontent"))
	if err != nil {
		return c.RenderJson(WSError{Status: 500, Error: err.Error()})
	}
	return c.RenderJson(WSSuccess{Status: 200, Data: "OK"})
}

func (c Ansible) Deploy() revel.Result {
	regexp, err := regexp.Compile("fatal: \\[(\\d+\\.\\d+\\.\\d+\\.\\d+)\\](?:.*)\"msg\": \"([^\"]*)\"")
	if err != nil {
		return c.RenderJson(WSError{Status: 500, Error: err.Error()})
	}
	if c.Params.Form.Get("filename") == "" {
		return c.RenderJson(WSError{Status: 500, Error: "please choose a file!"})
	} else if !strings.HasSuffix(c.Params.Form.Get("filename"), ".yaml") {
		return c.RenderJson(WSError{Status: 500, Error: "please choose correct file!"})
	}
	cmd := exec.Command("/bin/bash", "-c", "ansible-playbook "+rootPath+c.Params.Form.Get("filename"))
	result, _ := cmd.Output()
	errInfos := make([]ErrorInfo, 0)
	for _, value := range regexp.FindAllString(string(result), -1) {
		results := strings.Split(regexp.ReplaceAllString(value, "$1"+split+"$2"), split)
		errInfos = append(errInfos, ErrorInfo{Ip: results[0], Reason: results[1]})
	}
	return c.RenderJson(WSSuccess{Status: 200, Data: errInfos})
}

func (c Ansible) Status() revel.Result {
	if c.Params.Form.Get("group") == "" || c.Params.Form.Get("service") == "" {
		return c.RenderJson(WSError{Status: 500, Error: "input can't be null"})
	}
	regexp, err := regexp.Compile("(\\d+\\.\\d+\\.\\d+\\.\\d+) \\| (SUCCESS|FAILED)")
	if err != nil {
		return c.RenderJson(WSError{Status: 500, Error: err.Error()})
	}
	cmd := exec.Command("/bin/bash", "-c", "ansible "+c.Params.Form.Get("group")+" -m shell -a \"ps -C "+c.Params.Form.Get("service")+"\"")
	result, _ := cmd.Output()
	status := make([]StatusInfo, 0)
	for _, value := range regexp.FindAllString(string(result), -1) {
		results := strings.Split(regexp.ReplaceAllString(value, "$1"+split+"$2"), split)
		status = append(status, StatusInfo{Ip: results[0], Status: (results[1] == "SUCCESS")})
	}
	return c.RenderJson(WSSuccess{Status: 200, Data: status})
}

func (c Ansible) Restart() revel.Result {
	if c.Params.Form.Get("ip") == "" || c.Params.Form.Get("service") == "" {
		return c.RenderJson(WSError{Status: 500, Error: "input can't be null"})
	}
	if c.Params.Form.Get("service") == "Web" {
		c.Params.Form.Set("service", "gorevel")
	}
	cmd := exec.Command("/bin/bash", "-c", "ansible "+c.Params.Form.Get("ip")+" -m service -a \"name="+c.Params.Form.Get("service")+" state=restarted\" --become")
	result, _ := cmd.Output()
	return c.RenderJson(WSSuccess{Status: 200, Data: strings.Contains(string(result), "SUCCESS")})
}
