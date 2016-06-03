package main

import (
    "os"
    "log"
    "fmt"
    "database/sql"

    "github.com/domeos/dockerConnector/connector"
    "github.com/codegangsta/cli"
    _ "github.com/go-sql-driver/mysql"
)

func main() {

    logFile, err := os.OpenFile("dockerConnector.log", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
    if err == nil {
        log.SetOutput(logFile)
    } else {
        fmt.Println("[Error] Set Log File: ", err.Error())
    }

    app := cli.NewApp()
    app.Name = "dockerConnector"
    app.Usage = "Provide convenient access to docker container on any hosts via SSH"
    app.Version = "1.0"
    app.Author = "dockerConnector"
    app.Email = "baokangwang@sohu-inc.com"
    app.Flags = []cli.Flag{
        cli.StringFlag{
            Name:   "ssh-addr",
            Usage:  "the address that SSH listens",
            Value:  ":2222",
        },
        cli.StringFlag{
            Name:   "mysql-user",
            Usage:  "MySQL user",
            Value:  "root",
        },
        cli.StringFlag{
            Name:   "mysql-password",
            Usage:  "MySQL password",
            Value:  "root",
        },
        cli.StringFlag{
            Name:   "mysql-ip",
            Usage:  "MySQL ip",
            Value:  "127.0.0.1",
        },
        cli.StringFlag{
            Name:   "mysql-port",
            Usage:  "MySQL port",
            Value:  "3306",
        },
        cli.StringFlag{
            Name:   "mysql-database",
            Usage:  "MySQL database",
            Value:  "dockerConnector",
        },
    }
    app.Action = run
    if err := app.Run(os.Args); err != nil {
        log.Fatal("[Fatal] Start to Run: ", err.Error())
    }
}

func run(c *cli.Context) {

    errs := make(chan error)
    db, err := initDB(c)
    if err != nil {
        log.Fatal("[Fatal] Connect to MySQL: ", err.Error())
    }
    con, err := connector.New(c.String("ssh-addr"), c.String("server"), db, errs)
    if err != nil {
        log.Fatal("[Fatal] Create New Connector: ", err.Error())
    }
    con.Start()
    for i := 0; i < 2; i++ {
        err := <-errs
        if err != nil {
            log.Fatal("[Fatal]", err)
        }
    }
}

func initDB(c *cli.Context) (*sql.DB, error) {
    user := c.String("mysql-user")
    password := c.String("mysql-password")
    ip := c.String("mysql-ip")
    port := c.String("mysql-port")
    database := c.String("mysql-database")
    sqlString := user + ":" + password + "@tcp(" + ip + ":" + port + ")/" + database
    db, err := sql.Open("mysql", sqlString)
    if err != nil {
        return nil, err
    }
    return db, nil
}
