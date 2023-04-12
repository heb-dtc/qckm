package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/getlantern/systray"
)

const KIMAI_API = "https://kimai.curious-company.com/api/timesheets/"
const RECENTS_ENDOINT = "recent?size=10"
const ACTIVE_ENDOINT = "active"
const RESTART_ENDPOINT = "%d/restart"
const STOP_ENDPOINT = "%d/stop"

var recentTasks []Task
var activeTask Task

type NoActiveTaskErrror struct {}

func (e NoActiveTaskErrror) Error() string {
    return fmt.Sprintf("No active task")
}

type MenuState struct {
    Recent []Task
    Active Task
}

type Activity struct {
    Name string `json:"name"`     
}

type Project struct {
    Name string `json:"name"`
}

type Task struct {
    Id int `json:"id"`
    Activity Activity `json:"activity"`
    Project Project `json:"project"`
}

func (t Task) TextOutput() string {
    p := fmt.Sprintf("[%s] %s", t.Project.Name, t.Activity.Name)
    return p
}

func getIcon(s string) []byte {
    b, err := ioutil.ReadFile(s) 
    if err != nil {
        fmt.Println(err)
    }
    return b
}

func main() {
    systray.Run(onReady, onExit)
}

func onReady() {
    kimaiClient := http.Client{}

    systray.SetIcon(getIcon("assets/icon.ico"))
    recentMenu := systray.AddMenuItem("Recent", "")
    var recentEntries []*systray.MenuItem
    i := 0
    for i < 10 {
        recentEntry := recentMenu.AddSubMenuItem("", "")
        go func(idx int) {
            for {
                select {
                case <-recentEntry.ClickedCh:
                    fmt.Printf("%d clicked \n", idx)
                    task := recentTasks[idx]
                    fmt.Printf("%s clicked \n", task.Project.Name)
                    RestartTask(task.Id, kimaiClient)
                }
            }
        }(i)
        recentEntries = append(recentEntries, recentEntry)
        i++    
    }
    systray.AddSeparator()
    activeMenu := systray.AddMenuItem("Active", "")
    activeEntry := activeMenu.AddSubMenuItem("", "")
    stopItem := activeMenu.AddSubMenuItem("Stop", "")
    go func() {
        for {
            select {
            case <- stopItem.ClickedCh:
                StopTask(activeTask.Id, kimaiClient)
            }
        }
    }()
    systray.AddSeparator()
    refreshAction := systray.AddMenuItem("Refresh", "Refresh the menu")
    systray.AddSeparator()
    systray.AddMenuItem("Quit", "Quit the whole app")

    SetupMenu(kimaiClient, recentEntries, activeMenu, activeEntry)

    go func() {
        for {
            select {
            case <- refreshAction.ClickedCh:
                SetupMenu(kimaiClient, recentEntries, activeMenu, activeEntry)
            }
        }
    }()
}

func onExit() {
}

func SetupMenu(client http.Client, recentEntries []*systray.MenuItem, activeMenu *systray.MenuItem, activeEntry *systray.MenuItem) {
    GetMenuState(client)

    i := 0
    for i < 10 {
        task := recentTasks[i]
        recentEntries[i].SetTitle(task.TextOutput())

        i++
    }

    if activeTask.Id <= 0 {
       activeMenu.Disable()
    } else {
        activeMenu.Enable()
        activeEntry.SetTitle(activeTask.TextOutput())
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

func StopTask(id int, client http.Client) error {
    fmt.Println("Stopping task with id %d", id)
    requestUrl := KIMAI_API + fmt.Sprintf(STOP_ENDPOINT, id)

    req, err := http.NewRequest(http.MethodPatch, requestUrl, nil)
    if err != nil {
        fmt.Println(err)
        return  err
    }

    req.Header.Set("X-AUTH-USER", "flow")
    req.Header.Set("X-AUTH-TOKEN", "Upload-Italics1-Flammable")
    _, err = client.Do(req)
    if err != nil {
        fmt.Println(err)
        return err
    }

    return nil
}

func RestartTask(id int, client http.Client) error {
    requestUrl := KIMAI_API + fmt.Sprintf(RESTART_ENDPOINT, id)

    req, err := http.NewRequest(http.MethodPatch, requestUrl, nil)
    if err != nil {
        fmt.Println(err)
        return  err
    }

    req.Header.Set("X-AUTH-USER", "flow")
    req.Header.Set("X-AUTH-TOKEN", "Upload-Italics1-Flammable")
    _, err = client.Do(req)
    if err != nil {
        fmt.Println(err)
        return err
    }

    return nil
}

func FetchActive(client http.Client) (Task, error) {
    requestUrl := KIMAI_API + ACTIVE_ENDOINT
    var active Task

    req, err := http.NewRequest(http.MethodGet, requestUrl, nil)
    if err != nil {
        fmt.Println(err)
        return active, err
    }

    req.Header.Set("X-AUTH-USER", "flow")
    req.Header.Set("X-AUTH-TOKEN", "Upload-Italics1-Flammable")
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
    requestUrl := KIMAI_API + RECENTS_ENDOINT

    req, err := http.NewRequest(http.MethodGet, requestUrl, nil)
    if err != nil {
        fmt.Println(err)
        return nil, err
    }

    req.Header.Set("X-AUTH-USER", "flow")
    req.Header.Set("X-AUTH-TOKEN", "Upload-Italics1-Flammable")
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