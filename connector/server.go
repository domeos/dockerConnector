package connector

import (
    "fmt"
    "log"
    "net/http"
    "os"
    "net"
    "time"
    "strings"
    "io/ioutil"
    "database/sql"
    "encoding/json"

    "golang.org/x/crypto/ssh"
    "github.com/mountkin/dockerclient"
)

const DATA_DIR = "/var/lib/dockerConnector"

type ContainerInfo struct {
    ContainerId     string     `json:"containerId,omitempty"`
    ContainerName   string     `json:"containerName,omitempty"`
}

type Connector struct {
    sshAddr     string
    errch       chan <- error
    sshConfig   *ssh.ServerConfig
    hostKeyPath string
    db          *sql.DB
    hasDocker   bool
    dockerClient        dockerclient.Client
}

func New(addr string, server string, db *sql.DB, ch chan <- error) (*Connector, error) {
    hasDocker := true
    dockerClient, err := dockerclient.NewDockerClientTimeout("unix:///var/run/docker.sock", nil, 3*time.Second)
    if err != nil {
        hasDocker = false
    }
    con := &Connector{
        sshAddr:        addr,
        errch:          ch,
        hostKeyPath:    DATA_DIR + "/hostkey.rsa",
        db:             db,
        hasDocker:      hasDocker,
        dockerClient:   dockerClient,
    }
    if fp, err := os.Open(con.hostKeyPath); os.IsNotExist(err) {
        key, err := GenHostKey()
        if err != nil {
            return nil, err
        }
        if dirExisted := os.Chdir(DATA_DIR); dirExisted != nil {
            if mkdirError := os.MkdirAll(DATA_DIR, 0400); mkdirError != nil {
                return nil, err
            }
        }
        err = ioutil.WriteFile(con.hostKeyPath, key, 0400)
        if err != nil {
            return nil, err
        }
    } else {
        fp.Close()
    }
    if err := con.initServerConfig(); err != nil {
        return nil, err
    }
    return con, nil
}

func (con *Connector) Start() {

    log.Println(con.ListContainer())

    go func() {
        http.HandleFunc("/", con.ReportContainerList)
        err := http.ListenAndServe(":9090", nil)
        if err != nil {
            log.Fatal("[Fatal] ListenAndServe: ", err)
        }
    } ()

    go func() {
        l, err := net.Listen("tcp", con.sshAddr)
        if err != nil {
            con.errch <- err
            return
        }
        for {
            conn, err := l.Accept()
            if err != nil {
                log.Println("[Error] Accept:", err)
                continue
            }
            go con.serve(conn)
        }
    }()
}

func (con *Connector) ReportContainerList(w http.ResponseWriter, r *http.Request) {
    containerInfo := con.ListContainer()
    fmt.Fprintf(w, containerInfo)
}

func (con *Connector)ListContainer() (ret string) {

    if !con.hasDocker {
        return "[]"
    }
    containerRet := make([]ContainerInfo, 0)
    containers, err := con.dockerClient.ListContainers(true, false, "")
    if err != nil {
        log.Println("[Error] ListContainers", err)
        return
    }
    for _, container := range containers {
        if strings.HasPrefix(container.Status, "Up") {
            bytes := []byte(container.Id)[0:12]
            containerInfo := ContainerInfo{
                ContainerId:        string(bytes),
                ContainerName:      strings.TrimLeft(container.Names[0], "/"),
            }
            containerRet = append(containerRet, containerInfo)
        }
    }
    sendDataJson, sendDataJsonError := json.Marshal(containerRet)
    if sendDataJsonError != nil {
        return "Error"
    }
    log.Println("[Info] ListContainers Success")
    return string(sendDataJson)
}
