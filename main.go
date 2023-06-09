package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/heb-dtc/systray"
	"gopkg.in/yaml.v2"
)

//go:embed "assets/icon.ico"
var icon []byte

const (
	RECENTS_ENDOINT  = "recent?size=10"
	ACTIVE_ENDOINT   = "active"
	RESTART_ENDPOINT = "%d/restart"
	STOP_ENDPOINT    = "%d/stop"
)

type Config struct {
	URL      string `yaml:"url"`
	Username string `yaml:"user"`
	Token    string `yaml:"token"`
}

var (
	config      Config
	recentTasks []Task
	activeTask  Task
)

type NoActiveTaskErrror struct{}

func (e NoActiveTaskErrror) Error() string {
	return fmt.Sprintf("No active task")
}

type Activity struct {
	Name string `json:"name"`
}

type Project struct {
	Name string `json:"name"`
}

type Task struct {
	Id        int      `json:"id"`
	Activity  Activity `json:"activity"`
	Project   Project  `json:"project"`
	StartTime string   `json:"begin"`
}

func (t Task) TextOutput() string {
	p := fmt.Sprintf("[%s] %s", t.Project.Name, t.Activity.Name)
	return p
}

func (t Task) TaskDuration() string {
	// format StartTime to match RFC3339 format -> YYYY:MM:DDTHH:MM:SS+00:00
	var builder strings.Builder
	for index, char := range t.StartTime {
		builder.WriteString(string(char))
		if index == len(activeTask.StartTime)-3 {
			builder.WriteString(":")
		}
	}
	timeString := builder.String()

	taskTime, _ := time.Parse(time.RFC3339, timeString)
	duration := time.Since(taskTime)

    hours := int(duration.Seconds() / 3600)
    minutes := int(duration.Seconds() / 60) % 60
	return fmt.Sprintf("%d:%d h", hours, minutes)
}

func main() {
	homeDir, err := os.UserHomeDir()
	file, err := ioutil.ReadFile(homeDir + "/.config/qckm/qckm.yaml")
	if err != nil {
		panic("config file failed to open -> " + err.Error())
	}

	config = Config{}
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		panic("config file parsing err -> " + err.Error())
	}

	systray.Run(onReady, onExit)
}

func onReady() {
	kimaiClient := http.Client{}
	systray.SetIcon(icon)
	recentMenu := systray.AddMenuItem("Recent", "")
	systray.AddSeparator()
	activeMenu := systray.AddMenuItem("Active", "")
	systray.AddSeparator()
	refreshAction := systray.AddMenuItem("Refresh", "Refresh the menu")
	systray.AddSeparator()
	systray.AddMenuItem("Quit", "Quit the whole app")

	go func() {
		for {
			select {
			case <-refreshAction.ClickedCh:
				SetupMenu(kimaiClient, recentMenu, activeMenu)
			}
		}
	}()

	SetupMenu(kimaiClient, recentMenu, activeMenu)
}

func onExit() {
}

func SetupMenu(client http.Client, recentMenu *systray.MenuItem, activeMenu *systray.MenuItem) {
	GetMenuState(client)
	recentMenu.RemoveSubMenuItems()
	activeMenu.RemoveSubMenuItems()

	for i, task := range recentTasks {
		recentEntry := recentMenu.AddSubMenuItem(task.TextOutput(), "")
		go func(idx int) {
			for {
				select {
				case <-recentEntry.ClickedCh:
					task := recentTasks[idx]
					fmt.Printf("%s clicked \n", task.Project.Name)
					err := RestartTask(task.Id, client)
					if err == nil {
						SetupMenu(client, recentMenu, activeMenu)
					}
				}
			}
		}(i)

		i++
	}

	if activeTask.Id <= 0 {
		activeMenu.Disable()
	} else {
		activeMenu.Enable()
		activeTaskItem := fmt.Sprintf("%s (%s)", activeTask.TextOutput(), activeTask.TaskDuration())
		activeMenu.AddSubMenuItem(activeTaskItem, "")
		stopItem := activeMenu.AddSubMenuItem("Stop", "")
		go func() {
			for {
				select {
				case <-stopItem.ClickedCh:
					err := StopTask(activeTask.Id, client)
					if err == nil {
						SetupMenu(client, recentMenu, activeMenu)
					}
				}
			}
		}()
	}
}

func GetMenuState(client http.Client) {
	recent, err := FetchRecent(client)
	if err == nil {
		recentTasks = recent
	}

	active, err := FetchActive(client)
	if err == nil {
		activeTask = active
	} else {
		activeTask = *new(Task)
	}
}

func BuildRequest(method string, endpoint string) (*http.Request, error) {
	requestUrl := config.URL + endpoint
	fmt.Println("---- prepare request", method, " ", requestUrl)
	req, err := http.NewRequest(method, requestUrl, nil)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	req.Header.Set("X-AUTH-USER", config.Username)
	req.Header.Set("X-AUTH-TOKEN", config.Token)
	return req, nil
}

func StopTask(id int, client http.Client) error {
	fmt.Println("Stopping task with id ", id)

	req, err := BuildRequest(http.MethodPatch, fmt.Sprintf(STOP_ENDPOINT, id))
	if err != nil {
		return err
	}

	_, err = client.Do(req)
	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func RestartTask(id int, client http.Client) error {
	req, err := BuildRequest(http.MethodPatch, fmt.Sprintf(RESTART_ENDPOINT, id))
	if err != nil {
		return err
	}

	_, err = client.Do(req)
	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func FetchActive(client http.Client) (Task, error) {
	var active Task

	req, err := BuildRequest(http.MethodGet, ACTIVE_ENDOINT)
	if err != nil {
		return active, err
	}

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return active, err
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	if res.StatusCode != 200 {
		fmt.Println("request failed with code -> " + res.Status)
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		fmt.Println(readErr)
		return active, readErr
	}

	var activeRes []Task
	jsonErr := json.Unmarshal(body, &activeRes)
	if jsonErr != nil {
		fmt.Println(jsonErr)
		return active, jsonErr
	}

	if len(activeRes) > 0 {
		return activeRes[0], nil
	} else {
		return active, NoActiveTaskErrror{}
	}
}

func FetchRecent(client http.Client) ([]Task, error) {
	req, err := BuildRequest(http.MethodGet, RECENTS_ENDOINT)
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	if res.StatusCode != 200 {
		fmt.Println("request failed with code -> " + res.Status)
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		fmt.Println(readErr)
		return nil, readErr
	}

	var recent []Task
	jsonErr := json.Unmarshal(body, &recent)
	if jsonErr != nil {
		fmt.Println(jsonErr)
		return nil, jsonErr
	}

	return recent, nil
}
