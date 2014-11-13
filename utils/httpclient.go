package utils

import (
	"fmt"
	"io/ioutil"
	//"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// JobqueueHTTPClient provides some utility methods to communicate with the Task Manager runner via HTTP
type JobqueueHTTPClient struct {
	URL url.URL
}

func (client *JobqueueHTTPClient) setDefaultURL() {
	u, _ := url.Parse("http://localhost:8000/")
	client.URL = *u
}

//TODO: support JSON content type

// Open an HTTP connection to control the task manager runner
func (client *JobqueueHTTPClient) Open(url string) (string, error) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "http://" + url
	}
	re1, err := regexp.Compile(`([hftps]+)?://([\w\.]+)?(:\d+)?(/[^\?]+)?(?:\?(.*))?`)
	if nil != err {
		return "", err
	}
	res := re1.FindAllStringSubmatch(url, -1)
	if nil == res {
		return "", fmt.Errorf("Cannot parse url %s", url)
	}
	result := res[0]
	client.setDefaultURL()

	if "" != result[1] {
		client.URL.Scheme = result[1]
	}
	if "" != result[2] {
		client.URL.Host = result[2] + result[3]
	} else if "" != result[3] {
		client.URL.Host = "localhost" + result[3]
	}
	if "" != result[4] {
		client.URL.Path = result[4]
	}
	if "" != result[5] {
		client.URL.RawQuery = result[5]
	}

	return "Connected to " + client.URL.String(), err
}

// get the full URL from the path
func (client *JobqueueHTTPClient) getAddress(path string) string {
	u := client.URL
	u.Path = path
	return u.String()
}

// List the task managers listening at this address
func (client *JobqueueHTTPClient) List() (string, error) {
	resp, err := http.Get(client.getAddress("/list"))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	return string(bytes), err
}

// ListWorkers gets the status of each worker process for a given task
func (client *JobqueueHTTPClient) ListWorkers(name string) (string, error) {
	//log.Println("Listing workers for task", name)
	resp, err := http.Get(client.getAddress("/tasks/" + name + "/list"))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	return string(bytes), err
}

// ListAsList returns a list of task names (as a string slice)
func (client *JobqueueHTTPClient) ListAsList() []string {
	res, err := client.List()
	if err != nil {
		return []string{}
	}
	return strings.Split(res, "\n")
}

// Set an option on a certain task
func (client *JobqueueHTTPClient) Set(name string, param string, value string) (string, error) {
	//log.Println("Stopping task", name)
	path := fmt.Sprintf("/tasks/%s/set/%s/%s", name, param, value)
	resp, err := http.PostForm(client.getAddress(path), nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	return string(bytes), err
}

// Start a stopped task
func (client *JobqueueHTTPClient) Start(name string) (string, error) {
	//log.Println("Starting task", name)
	resp, err := http.PostForm(client.getAddress("/tasks/"+name+"/start"), nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	return string(bytes), err
}

// Stop a running task, or all of them
func (client *JobqueueHTTPClient) Stop(name string) (string, error) {
	//log.Println("Stopping task", name)
	//resp, err := http.PostForm(client.getAddress("/tasks/"+name+"/stop"), nil)
	req, err := http.NewRequest("DELETE", client.getAddress("/tasks/"+name), nil)
	if err != nil {
		return "", err
	}
	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	return string(bytes), err
}

// Status gets some information about the status of a task (or all of them)
func (client *JobqueueHTTPClient) Status(name string) (string, error) {
	//log.Println("Status")
	resp, err := http.Get(client.getAddress("/tasks/" + name))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	return string(bytes), err
}
