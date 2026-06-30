//go:build !(darwin && cgo)

package main

import "errors"

var errShmUnavailable = errors.New("shared memory medium unavailable")

// shmAvailable reports that the shared-memory medium is not compiled in (it
// needs cgo on darwin). The shm medium path errors out cleanly via this.
const shmAvailable = false

func shmWrite(string, []byte) error { return errShmUnavailable }
func shmUnlink(string)              {}

func shmRead(string, int) ([]byte, error) { return nil, errShmUnavailable }
