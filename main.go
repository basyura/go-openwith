package main

import (
	"fmt"
	"log"
	"os"

	"github.com/kardianos/service"
	"github.com/labstack/echo/v4"
)

var Name = "OpenWith"
var DisplayName = "OpenWith"
var Description = "OpenWith Service"

// perfv.go Run (mac だと認識してくれないので変数に入れてから呼ぶ)
var Run func() *echo.Echo

func doRun() *echo.Echo {
	return Run()
}

var serviceLogger service.Logger

type pgservice struct {
	exit chan struct{}
}

func (e *pgservice) Start(s service.Service) error {
	if service.Interactive() {
		fmt.Println("*****  Running in terminal  *****")
	} else {
		serviceLogger.Info(DisplayName, "running under service manager.")
		// Set environment variable to indicate service mode
		os.Setenv("SERVICE_MODE", "true")
	}
	e.exit = make(chan struct{})
	go e.run()

	return nil
}

func (e *pgservice) run() error {

	sv := doRun()

	for {
		select {
		case <-e.exit:
			serviceLogger.Info(DisplayName, "Stop ...")
			sv.Close()
			serviceLogger.Info(DisplayName, "Stop ... Done")
			return nil
		}
	}

}

func (e *pgservice) Stop(s service.Service) error {
	close(e.exit)
	return nil
}

func main() {

	program := &pgservice{}
	s, err := service.New(program, &service.Config{
		Name:        Name,
		DisplayName: DisplayName,
		Description: Description,
	})

	if err != nil {
		log.Fatal(err)
	}

	// Setup the logger
	errs := make(chan error, 5)
	serviceLogger, err = s.Logger(errs)
	if err != nil {
		log.Fatal()
	}

	if len(os.Args) > 1 {
		err = service.Control(s, os.Args[1])
		if err != nil {
			fmt.Printf("Failed (%s) : %s\n", os.Args[1], err)
			return
		}
		fmt.Printf("Succeeded (%s)\n", os.Args[1])
		return
	}

	// run in terminal
	s.Run()
}
