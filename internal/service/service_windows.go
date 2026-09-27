package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// Install registers an automatic-start Windows service. It needs an elevated prompt.
func Install(exe, configPath string) (string, error) {
	m, err := connect()
	if err != nil {
		return "", err
	}
	defer m.Disconnect()

	if s, err := m.OpenService(Name); err == nil {
		stop(s)
		s.Delete()
		s.Close()
		time.Sleep(2 * time.Second)
	}

	s, err := m.CreateService(Name, exe, mgr.Config{
		DisplayName:      DisplayName,
		Description:      Description,
		StartType:        mgr.StartAutomatic,
		DelayedAutoStart: true,
	}, runArgs(configPath)...)
	if err != nil {
		return "", fmt.Errorf("creating the service: %w", err)
	}
	defer s.Close()

	_ = s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
		{Type: mgr.ServiceRestart, Delay: time.Minute},
		{Type: mgr.ServiceRestart, Delay: 5 * time.Minute},
	}, 24*60*60)

	if err := s.Start(); err != nil {
		return "", fmt.Errorf("starting the service: %w", err)
	}
	return fmt.Sprintf("Installed and started the %q service. It starts with Windows.", DisplayName), nil
}

func Uninstall() (string, error) {
	m, err := connect()
	if err != nil {
		return "", err
	}
	defer m.Disconnect()

	s, err := m.OpenService(Name)
	if err != nil {
		return "The service isn't installed.", nil
	}
	defer s.Close()
	stop(s)
	if err := s.Delete(); err != nil {
		return "", err
	}
	return "Stopped and removed the service.", nil
}

func connect() (*mgr.Mgr, error) {
	m, err := mgr.Connect()
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return nil, errors.New("run this from an administrator prompt (right-click PowerShell > Run as administrator)")
	}
	return m, err
}

func stop(s *mgr.Service) {
	status, err := s.Control(svc.Stop)
	for i := 0; err == nil && status.State != svc.Stopped && i < 20; i++ {
		time.Sleep(500 * time.Millisecond)
		status, err = s.Query()
	}
}

// RunManaged hands control to the Windows service manager when started as a service.
func RunManaged(runFn func(context.Context)) (bool, error) {
	isService, err := svc.IsWindowsService()
	if err != nil || !isService {
		return false, err
	}
	return true, svc.Run(Name, handler{runFn})
}

type handler struct{ run func(context.Context) }

func (h handler) Execute(_ []string, requests <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		h.run(ctx)
		close(done)
	}()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case req := <-requests:
			switch req.Cmd {
			case svc.Interrogate:
				status <- req.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				cancel()
				<-done
				return false, 0
			}
		case <-done:
			cancel()
			return false, 0
		}
	}
}
