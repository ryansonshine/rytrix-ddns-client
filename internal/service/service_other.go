//go:build !darwin && !linux && !windows

package service

import (
	"context"
	"errors"
)

var errUnsupported = errors.New("installing as a service isn't supported on this OS; start `rytrix-ddns run` from your init system instead")

func Install(string, string) (string, error) { return "", errUnsupported }

func Uninstall() (string, error) { return "", errUnsupported }

func RunManaged(func(context.Context)) (bool, error) { return false, nil }
