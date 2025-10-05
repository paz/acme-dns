//go:build !windows && !test
// +build !windows,!test

package main

import "syscall"

func init() {
	// Created files are not world writable
	syscall.Umask(0077)
}
