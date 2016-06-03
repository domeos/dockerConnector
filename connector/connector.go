package connector

import (
    "io"
    "net"
    "log"
    "sync"
    "errors"
    "unsafe"
    "syscall"
    "os/exec"
    "io/ioutil"
    "encoding/binary"

    "github.com/kr/pty"
    "golang.org/x/crypto/ssh"
)

var (
    ErrInvalidContainer     = errors.New("login name must has the format of USERNAME.CONTAINER_ID, where CONTAINER_ID is the container that you want to login")
    ErrNoPassword           = errors.New("username or password is not set")
    ErrInvalidPassword      = errors.New("username or password is incorrect")
)


func (con *Connector) initServerConfig() error {
    keybytes, err := ioutil.ReadFile(con.hostKeyPath)
    if err != nil {
        return err
    }
    cfg := &ssh.ServerConfig{}
    key, err := ssh.ParsePrivateKey(keybytes)
    if err != nil {
        return err
    }
    cfg.AddHostKey(key)
    cfg.PasswordCallback = con.passwordCallback
    con.sshConfig = cfg
    return nil
}

func (con *Connector) passwordCallback(meta ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
    container := meta.User()
    if len(container) != 12 {
        return nil, ErrInvalidContainer
    }
    /*
    stmt, err := con.db.Prepare("select password from user where name = ?")
    if err != nil {
        return nil, err
    }

    var savedPass string
    err = stmt.QueryRow(user).Scan(&savedPass)
    stmt.Close()
    if err != nil {
        log.Println("[Error] Scan:", err)
        return nil, ErrNoPassword
    }

    if string(pass) != savedPass {
        return nil, ErrInvalidPassword
    }
    */
    return nil, nil
}

func (con *Connector) serve(conn net.Conn) {
    sshConn, chans, reqs, err := ssh.NewServerConn(conn, con.sshConfig)
    if err != nil {
        log.Println("[Error] Ssh.NewServerConn: ", err)
        conn.Write([]byte(err.Error()))
        conn.Close()
        return
    }
    container := sshConn.User()

    log.Println("[Info] New connection", sshConn.RemoteAddr(), string(sshConn.ClientVersion()))
    go con.handleRequests(reqs)
    go con.handleChannels(chans, container)
}

func (con *Connector) handleRequests(requests <-chan *ssh.Request) {
    for req := range requests {
        log.Println("[Info] Out-of-band request:", req.Type)
    }
}

func (con *Connector) handleChannels(chans <-chan ssh.NewChannel, container string) {
    for ch := range chans {
        if ch.ChannelType() != "session" {
            ch.Reject(ssh.UnknownChannelType, "unknown channel type")
            continue
        }
        channel, requests, err := ch.Accept()
        if err != nil {
            log.Println("[Error]", err)
            continue
        }
        con.handleChannel(channel, requests, container)
    }
}

func (con *Connector) handleChannel(channel ssh.Channel, requests <-chan *ssh.Request, container string) {
    cmd := exec.Command("docker", "exec", "-i", "-t", container, "/bin/bash")
    
    closeChannel := func() {
        channel.Close()
        err := cmd.Process.Kill()
        if err != nil {
            log.Println("[Error] Failed to kill docker exec", err)
        }
        cmd.Process.Wait()
    }

    fp, err := pty.Start(cmd)
    if err != nil {
        log.Println("[Error] pty.Start: ", err)
        closeChannel()
        return
    }

    go func() {
        for req := range requests {
            log.Println("[Info] new request: ", req.Type)
            switch req.Type {
            case "shell":
                if len(req.Payload) == 0 {
                    req.Reply(true, nil)
                }
            case "pty-req":
                termLen := req.Payload[3]
                w, h := con.parseDims(req.Payload[termLen+4:])
                con.setWinsize(fp.Fd(), w, h)
                req.Reply(true, nil)
            case "window-change":
                w, h := con.parseDims(req.Payload)
                con.setWinsize(fp.Fd(), w, h)
            case "env":
            }
        }
    }()
    
    var once sync.Once
    cp := func(dst io.Writer, src io.Reader) {
        io.Copy(dst, src)
        once.Do(closeChannel)
    }
    go cp(channel, fp)
    go cp(fp, channel)
}

func (con *Connector) parseDims(b []byte) (uint32, uint32) {
    w := binary.BigEndian.Uint32(b)
    h := binary.BigEndian.Uint32(b[4:])
    return w, h
}
type Winsize struct {
    Height uint16
    Width  uint16
    x      uint16 // unused
    y      uint16 // unused
}

func (con *Connector) setWinsize(fd uintptr, w, h uint32) {
    ws := &Winsize{Width: uint16(w), Height: uint16(h)}
    syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(ws)))
}
